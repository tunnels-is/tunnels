# Tunnels
 - [Website](https://tunnels.is)
 - [License](https://github.com/tunnels-is/tunnels/blob/main/LICENSE)
 - [Discord]( https://discord.gg/7Ts3PCnCd9)
 - [Live Development](https://twitch.tv/keyb1nd_)

# Requirements
### Client Web UI
 - vite
### Client Golang App
 - golang (macos: ifconfig, route), (windows: netsh)
### Server (supports linux and docker)
 - iptables (only needed when running the server)
 - golang

# Starting the golang client
NOTE: The client requires sudo/admin on windows and mac, but only narrow net admin permission on linux.
```bash
$ cd ./cmd/main
$ go build .
$ ./main
```

# Starting the web UI
NOTE: when opening the dev ui, you must first accept the TLS certificate on port 7777 (https://127.0.0.1:7777)
```bash
$ cd ./frontend
$ pnpm install .
$ vite dev 
```

# Starting the server
```bash
$ cd ./server
$ go build .
fresh run: $ ./server --config 
$ ./server
```

# Notes about development
We accept any code, even from machine learning models
as long as the code makes sense, even small spellfixing
contributions. Just remember to run the linter before submitting.

# Random development / deployment information
### iptables for the server
these are applied automatically on startup
```
$ iptables -I OUTPUT -p tcp --src {interface_IP} --tcp-flags ACK,RST RST -j DROP
```
### Testing
The project includes comprehensive test coverage for all server components.

```bash
# Run all tests
$ make test

# Run tests with verbose output
$ make test-server

# Run tests with coverage report
$ make test-coverage

# Count total number of tests
$ make test-count

# Run tests with race detection
$ make test-verbose

# Or run tests directly with go
$ go test ./server/...
```

### Linting
```
$ golangci-lint run --timeout=10m --config .golangci.yml

# Or use make
$ make lint
```
### Permissions
 - Windows: admin
 - macos: sudo
 - linux: setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' main

## Building
The goreleaser pipeline builds three artifacts:

| Binary | Path | Notes |
| --- | --- | --- |
| `tunnels-cli` | `./cmd/main` | Client with embedded browser UI (no Wails) |
| `tunnels-server` | `./server` | Control plane (Linux) |
| `tunnels-app` | `./cmd/wails` | Wails desktop app (Windows always; Linux/macOS only on a matching host) |

 - DEV: `./releaser-build-snapshot.sh` (or `make release`)
 - PROD: `./releaser-build-release.sh` (requires `GITHUB_TOKEN`)

```bash
# Build using make
$ make build           # tunnels-cli + tunnels-server
$ make build-server    # tunnels-server only
$ make build-client    # tunnels-cli only
$ make build-app       # tunnels-app (via ./build-wails.sh)

# Test before building
$ make pre-commit      # Run tests and linting
$ make ci              # Run CI checks locally
```

### Wails desktop (`tunnels-app`)
Requires the frontend to be built first (`./build-ui.sh`). Linux needs
`libgtk-3-dev` + `libwebkit2gtk-4.1-dev`. macOS needs Xcode CLT. Windows
cross-compiles with `CGO_ENABLED=0` from any host.  

# Special mentiones
These are the real MVPs:

    - n00bady: creator of bluam https://github.com/n00bady/bluam
    - 0xMALVEE: for major contributiosn to the front-end
    - keyb1nd_'s twitch chat for the backseat debugging and support!
    - comahacks for security reviews
    - klauspost for development advice

[forks-shield]: https://img.shields.io/github/forks/tunnels-is/tunnels?style=for-the-badge&logo=github
[forks-url]: https://github.com/tunnels-is/tunnels/network/members
[stars-shield]: https://img.shields.io/github/stars/tunnels-is/tunnels?style=for-the-badge&logo=github
[stars-url]: https://github.com/tunnels-is/tunnels/stargazers
[issues-shield]: https://img.shields.io/github/issues/tunnels-is/tunnels?style=for-the-badge&logo=github
[issues-url]: https://github.com/tunnels-is/tunnels/issues
