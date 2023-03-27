# Tailscale Restic Proxy
An authentication proxy for the restic rest server using the Tailscale local client.

## Project status
✅ 🐶 Under active use by developer

## Features
- Backups by Tailscale user or node without passwords
- Compatible with Headscale

## Usage
### Server Setup
- Start restic REST server (tested with [Restic REST](https://github.com/restic/rest-server))
  - `--private-repos` recommended
  - set `--htpasswd-file` to path accessible to ts-restic-proxy
  - `--path` should be set, the default setting doesn't persist data
  - `--listen 127.0.0.1:<restic-port>` 
  - HTTPS not supported
- Start ts-restic-proxy
  -  set `--htpasswd-file` to same path as restic server
  - `--restic-rest-server localhost:<restic-port>`
  - `--proxy-non-tailscale` to enable backups from local network

### Initialize repository
```bash
restic init -r rest:http://<server-tailscale-ip>/ts/<user|node>/<subpath>
```
