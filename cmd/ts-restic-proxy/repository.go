package main

import (
	"net/http"
	"strings"
	"time"
)

type Repository struct {
	Path         string    `json:"path"`
	LastRead     time.Time `json:"lastRead"`
	LastWrite    time.Time `json:"lastWrite"`
	LastSnapshot time.Time `json:"lastSnapshot"`
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
