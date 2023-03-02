package main

import (
	"bufio"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func htpasswdUserExists(username string) bool {
	// Read htpasswd file
	file, err := os.Open(*dataDir + "/.htpasswd")
	if err != nil {
		return false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Split(scanner.Text(), ":")[0] == username {
			return true
		}
	}
	return false
}

func htpasswdAddUser(username string, password string) {
	// Add user to htpasswd file or updates the password

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	file, err := os.OpenFile(*dataDir+"/.htpasswd", os.O_RDWR|os.O_CREATE, 0700)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	buffer := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Split(scanner.Text(), ":")[0] != username {
			// User does not exist, add to buffer
			buffer = append(buffer, scanner.Text())
		}
	}
	// Add user to buffer
	buffer = append(buffer, username+":"+string(hash))
	// Write buffer to file
	file.Truncate(0)
	file.Seek(0, 0)
	for _, line := range buffer {
		file.WriteString(line + "\n")
	}
}
