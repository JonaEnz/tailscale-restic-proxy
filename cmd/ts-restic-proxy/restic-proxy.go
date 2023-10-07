package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/thanhpk/randstr"
	"tailscale.com/client/tailscale"
)

type Repository struct {
	Path         string    `json:"path"`
	LastRead     time.Time `json:"lastRead"`
	LastWrite    time.Time `json:"lastWrite"`
	LastSnapshot time.Time `json:"lastSnapshot"`
}

type State struct {
	Version      int
	Passwords    map[string]string
	Repositories map[string]Repository
}

const (
	cacheTimeoutSpan = 15 * time.Minute
)

var (
	state         State
	tsLocalClient tailscale.LocalClient
	cacheUsers    map[string][]string // map[ip][userResticUser,nodeResticUser]
	cacheTimeout  map[string]int64
)

var httpProxyHandler http.Handler = http.HandlerFunc(
	func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s Request: %s\n", r.Method, r.URL.Path)
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/ts/" {
			// Check if tailscale is up
			if !TailscaleUp() {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("Tailscale is not running"))
				return
			} else {
				// Transform request
				request, err := TransformRequest(*r)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
				// Proxy request
				err = ProxyRequest(request, &w, *resticServer)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
				checkRequestForRepositoryUpdate(request)
			}
		} else {
			if *proxyNonTailscale {
				// Proxy request
				err := ProxyRequest(r, &w, *resticServer)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
				log.Printf("Unauthorized access attempt from %s\n", r.RemoteAddr)
				w.Write([]byte("Not authorized."))
				return
			}
		}
	},
)

func TailscaleUp() bool {
	_, err := tsLocalClient.Status(context.Background())
	return err == nil
}

func AddToCache(ip string, tsUserResticUser string, tsNodeResticUser string) {
	// Add ip to cache
	ipWithoutPort := strings.Split(ip, ":")[0]
	if cacheUsers == nil {
		cacheUsers = map[string][]string{}
		cacheTimeout = map[string]int64{}
	}
	cacheUsers[ipWithoutPort] = []string{tsUserResticUser, tsNodeResticUser}
	cacheTimeout[ipWithoutPort] = time.Now().Add(cacheTimeoutSpan).Unix()
}

func CheckCache(ip string) ([]string, bool) {
	// Check if ip is in cache
	ipWithoutPort := strings.Split(ip, ":")[0]
	users, ok := cacheUsers[ipWithoutPort]
	if ok {
		timeout, ok := cacheTimeout[ipWithoutPort]
		if ok {
			if timeout > time.Now().Unix() {
				// Cache hit
				return users, true
			}
		}
	}
	return nil, false
}

func LoadState() {
	// Load state from json file
	//If file does not exist, create it
	//If file exists, load it
	filePath := (*dataDir) + "/state.json"
	file, err := os.Open(filePath)
	if err != nil {
		// File does not exist
		state = State{
			Version:      1,
			Passwords:    map[string]string{},
			Repositories: map[string]Repository{},
		}
		SaveState()
	} else {
		// File exists
		defer file.Close()
		decoder := json.NewDecoder(file)
		err := decoder.Decode(&state)
		if state.Version == 1 {
			//Upgrade state version
			state.Repositories = map[string]Repository{}
			state.Version = 2
			SaveState()
			log.Printf("Upgraded state to version %d\n", state.Version)
		}
		if err != nil || state.Version > 2 || state.Version < 1 {
			panic(err)
		}
	}
}

func SaveState() {
	// Save state to json file
	file, err := os.Create((*dataDir) + "/state.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	err = json.NewEncoder(file).Encode(state)
	if err != nil {
		panic(err)
	}
}

func getRepository(path string) Repository {
	// Get repository from state
	repository, ok := state.Repositories[path]
	if ok {
		return repository
	}
	return Repository{
		Path:         path,
		LastRead:     time.Unix(0, 0),
		LastWrite:    time.Unix(0, 0),
		LastSnapshot: time.Unix(0, 0),
	}
}

func (r *Repository) repositoryRead() {
	if time.Since(r.LastRead) < 5*time.Second {
		return
	}
	// Update lastRead
	r.LastRead = time.Now()
	updateRepository(*r)
}

func (r *Repository) repositoryWritten() {
	if time.Since(r.LastWrite) < 5*time.Second {
		return
	}

	r.LastWrite = time.Now()
	updateRepository(*r)
}

func (r *Repository) repositorySnapshot() {
	if time.Since(r.LastSnapshot) < 5*time.Second {
		return
	}
	r.LastSnapshot = time.Now()
	updateRepository(*r)
}
func updateRepository(repository Repository) {
	// Update repository in state
	state.Repositories[repository.Path] = repository
	SaveState()
}

func checkRequestForRepositoryUpdate(r *http.Request) {
	validTypes := [...]string{"data", "keys", "snapshots", "index", "config", "locks"}
	repoName := r.URL.Path[1:]
	if repoName[len(repoName)-1:] == "/" {
		repoName = repoName[:len(repoName)-1]
	}
	// Remove last segment of path
	splitName := strings.Split(repoName, "/")
	// Iterate backwards over path segments until we find a valid restic request type
	resticRequestType, typeIndex := "", -1
	for i := len(splitName) - 1; i >= 0; i-- {
		for _, validType := range validTypes {
			if splitName[i] == validType {
				resticRequestType = validType
				typeIndex = i
				break
			}
		}
	}
	if resticRequestType != "" {
		// Request type found, remove everything after it
		repoName = strings.Join(splitName[:typeIndex], "/")
	} else if splitName[len(splitName)-1] != "config" {
		// If no request type is found, remove nothing
		repoName = strings.Join(splitName, "/")
	} else {
		// If config is requested, remove config from path
		repoName = strings.Join(splitName[:len(splitName)-1], "/")
	}

	repository := getRepository(repoName)
	if r.Method != "POST" || resticRequestType == "locks" {
		repository.repositoryRead()
	} else if resticRequestType == "snapshots" {
		repository.repositorySnapshot()
	} else {
		repository.repositoryWritten()
	}
}

func GetNodeUserAndKey(ipAndPort string) (string, string, error) {
	// Check if ip is in cache
	users, ok := CheckCache(ipAndPort)
	if ok {
		return users[0], users[1], nil
	}

	// Get node key and userId from IP
	whosis, err := localClient.WhoIs(context.Background(), ipAndPort)
	if err != nil {
		return "", "", err
	}
	return whosis.UserProfile.ID.String(), whosis.Node.Key.ShortString(), nil
}

func getResticUsername(keyOrID string) string {
	// Hash userID or nodekey to get htpasswd username
	hash := sha256.New()
	hash.Write([]byte(keyOrID))
	return string(hex.EncodeToString(hash.Sum(nil)))[0:8]
}

func TransformRequest(request http.Request) (*http.Request, error) {
	// Adapt the request for the proxy target
	if len(request.URL.Path) < 8 || request.URL.Path[:3] != "/ts" { //
		return nil, errors.New("path doesn't start with /ts/<node/user>/")
	}

	user, key, err := GetNodeUserAndKey(request.RemoteAddr)
	path := request.URL.Path[3:] //Remove /ts
	basicAuth := ""
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	uname := ""
	if path[:5] == "/node" {
		uname = getResticUsername(key)
		request.URL.Path = "/" + uname + path[5:]

	} else if path[:5] == "/user" {
		uname = getResticUsername(user)
		request.URL.Path = "/" + uname + path[5:]
	} else {
		return &request, nil
	}

	// Add user to cache
	AddToCache(request.RemoteAddr, user, key)

	if _, ok := state.Passwords[uname]; !ok {
		// Generate new password
		state.Passwords[uname] = randstr.Hex(32)
		SaveState()
	}
	// Add user to htpasswd file
	htpasswdAddUser(uname, state.Passwords[uname])
	basicAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(uname+":"+state.Passwords[uname]))

	if basicAuth != "" {
		request.Header.Set("Authorization", basicAuth)
	}
	return &request, nil

}

func ProxyRequest(requestIn *http.Request, respOut *http.ResponseWriter, target string) error {
	//Proxy the request to the target
	requestOut, err := http.NewRequest(requestIn.Method, target+requestIn.URL.RequestURI(), requestIn.Body)
	if err != nil {
		return err
	}
	requestOut.Header = requestIn.Header
	requestOut.Method = requestIn.Method
	requestOut.URL.Path = requestIn.URL.Path
	requestOut.Proto = requestIn.Proto
	responseIn, err := http.DefaultClient.Do(requestOut)
	if err != nil {
		return err
	}

	//Copy the response to the original response
	defer responseIn.Body.Close()
	(*respOut).Header().Set("Content-Type", responseIn.Header.Get("Content-Type"))
	(*respOut).Header().Set("Content-Length", fmt.Sprintf("%d", responseIn.ContentLength))
	(*respOut).WriteHeader(responseIn.StatusCode)
	fmt.Println(responseIn.StatusCode)

	data, err := io.ReadAll(responseIn.Body)
	fmt.Println(string(data))
	if err != nil {
		return err
	}

	_, err = (*respOut).Write([]byte(data))
	if err != nil {
		return err
	}
	return nil
}
