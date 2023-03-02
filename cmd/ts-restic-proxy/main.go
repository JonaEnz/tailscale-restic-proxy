package main

import (
	"flag"
	"net/http"

	"tailscale.com/client/tailscale"
)

var (
	listenAdress      = flag.String("listen", ":8000", "Address to listen on")
	dataDir           = flag.String("data-dir", ".", "Directory to store state in")
	htpasswdFile      = flag.String("htpasswd-file", "", "Path to htpasswd file")
	proxyNonTailscale = flag.Bool("proxy-non-tailscale", false, "Proxy non-tailscale requests to the restic server")
	resticServer      = flag.String("restic-rest-server", "http://localhost:9234/", "Address of the restic server")
)

var server = http.Server{}
var localClient tailscale.LocalClient

func main() {
	flag.Parse()
	loadState()

	server.Addr = *listenAdress
	server.Handler = httpProxyHandler
	err := server.ListenAndServe()

	if err != nil {
		panic(err)
	}
}