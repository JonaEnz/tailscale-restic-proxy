package main

import (
	"flag"
	"fmt"
	"net/http"

	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
)

var (
	listenAdress           = flag.String("listen", ":8000", "Address to listen on")
	dataDir                = flag.String("data-dir", ".", "Directory to store state in")
	htpasswdFile           = flag.String("htpasswd-file", "", "Path to htpasswd file")
	proxyNonTailscale      = flag.Bool("proxy-non-tailscale", false, "Proxy non-tailscale requests to the restic server")
	resticServer           = flag.String("restic-rest-server", "http://localhost:9234/", "Address of the restic server")
	hostname               = flag.String("hostname", "restic-server", "Hostname to use for the restic server in the tailnet")
	tailscaleAuthKey       = flag.String("ts-auth-key", "", "Tailscale auth key")
	tailscaleControlServer = flag.String("ts-login-server", "https://login.tailscale.com", "Address of the tailscale control server")
)

var localClient tailscale.LocalClient

func main() {
	flag.Parse()
	LoadState()

	fmt.Printf("%s %s\n", *tailscaleControlServer, *tailscaleAuthKey)
	if *tailscaleAuthKey == "" {
		s := new(http.Server)
		s.Addr = *listenAdress
		s.Handler = httpProxyHandler
		err := s.ListenAndServe()
		if err != nil {
			panic(err)
		}
		return
	} else {
		s := new(tsnet.Server)
		s.Hostname = *hostname
		s.AuthKey = *tailscaleAuthKey
		s.ControlURL = *tailscaleControlServer
		defer s.Close()
		ln, err := s.Listen("tcp", *listenAdress)

		if err != nil {
			panic(err)
		}
		defer ln.Close()
		http.Serve(ln, httpProxyHandler)

		if err != nil {
			panic(err)
		}
	}
}
