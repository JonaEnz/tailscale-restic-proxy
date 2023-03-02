package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/thanhpk/randstr"
	"tailscale.com/client/tailscale"
)

type State struct {
	Passwords map[string]string
}

var state State
var tsLocalClient tailscale.LocalClient

var httpProxyHandler http.Handler = http.HandlerFunc(
	func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.URL.Path[:4])
		if r.URL.Path[:4] == "/ts/" {
			// Check if tailscale is up
			if !tailscaleUp() {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("Tailscale is not running"))
				return
			} else {
				// Transform request
				request, err := transformRequest(*r)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
				// Proxy request
				err = proxyRequest(request, &w, *resticServer)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
			}
		}
	},
)

func tailscaleUp() bool {
	_, err := tsLocalClient.Status(context.Background())
	return err == nil
}

func loadState() {
	// Load state from json file
	//If file does not exist, create it
	//If file exists, load it
	filePath := (*dataDir) + "/state.json"
	file, err := os.Open(filePath)
	if err != nil {
		// File does not exist
		state = State{
			Passwords: map[string]string{},
		}
		saveState()
	} else {
		// File exists
		defer file.Close()
		decoder := json.NewDecoder(file)
		err := decoder.Decode(&state)
		if err != nil {
			panic(err)
		}
	}
}

func saveState() {
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

func getNodeUserAndKey(ip string) (string, string, error) {
	// Get node key and userId from IP
	whosis, err := localClient.WhoIs(context.Background(), ip)
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

func transformRequest(request http.Request) (*http.Request, error) {
	// Adapt the request for the proxy target
	path := request.URL.Path[3:] //Remove /ts
	user, key, err := getNodeUserAndKey(request.RemoteAddr)
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

	if _, ok := state.Passwords[uname]; !ok {
		// Generate new password
		state.Passwords[uname] = randstr.Hex(128)
		saveState()
	}
	if !htpasswdUserExists(uname) {
		// Add user to htpasswd file
		htpasswdAddUser(uname, state.Passwords[uname])
	}
	basicAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(uname+":"+state.Passwords[uname]))

	if basicAuth != "" {
		request.Header.Set("Authorization", basicAuth)
	}
	return &request, nil

}

func proxyRequest(requestIn *http.Request, respOut *http.ResponseWriter, target string) error {
	//Proxy the request to the target
	requestOut, err := http.NewRequest(requestIn.Method, target+requestIn.URL.RequestURI(), requestIn.Body)
	if err != nil {
		return err
	}
	requestOut.Header = requestIn.Header
	requestOut.Method = requestIn.Method
	requestOut.URL.Path = requestIn.URL.Path
	requestOut.Proto = requestIn.Proto
	//requestOut.Host = requestIn.Host
	responseIn, err := http.DefaultClient.Do(requestOut)
	if err != nil {
		return err
	}
	defer responseIn.Body.Close()

	//Copy the response to the original response
	(*respOut).Header().Set("Content-Type", responseIn.Header.Get("Content-Type"))
	(*respOut).WriteHeader(responseIn.StatusCode)
	buffer := make([]byte, responseIn.ContentLength)
	_, _ = responseIn.Body.Read(buffer)
	_, err = (*respOut).Write(buffer)
	if err != nil {
		return err
	}
	return nil
}
