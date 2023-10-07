package main

import (
	"strings"
	"time"
)

const (
	cacheTimeoutSpan = 15 * time.Minute
)

var (
	cacheUsers   map[string][]string // map[ip][userResticUser,nodeResticUser]
	cacheTimeout map[string]int64
)

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
