# Tunnels
 - [Website](https://tunnels.is)
 - [License](https://github.com/tunnels-is/tunnels/blob/main/LICENSE)
 - [Discord](https://discord.gg/7Ts3PCnCd9)
 - [Live Development](https://twitch.tv/keyb1nd_)

# Requirements
Go 1.26 (`go.mod`).

### Client
 - macOS: `ifconfig` and `route`. The client must run as root. The packaged desktop app asks for sudo.
 - Windows: `netsh`. Run the client as administrator.
 - Linux: `ip` (iproute2). The client does not need full root. It needs `cap_net_raw`, `cap_net_bind_service`, and `cap_net_admin`. If those are missing it asks for a sudo password and runs `setcap` on its own binary.

### Fyne desktop
A C compiler, plus:
 - Linux: `gcc`, `pkg-config`, `libgl1-mesa-dev`, `xorg-dev`, and `libxkbcommon-dev`. `./build-fyne.sh linux` only builds on a Linux host.
 - macOS: Xcode command line tools. `./build-fyne.sh darwin` only builds on a Mac.
 - Windows: `x86_64-w64-mingw32-gcc` or `zig` (`zig cc -target x86_64-windows-gnu`).

### Server
Linux (goreleaser builds amd64, arm64, and arm). `iptables` and `ip6tables` are required only when the WireGuard module is enabled (`-wg` or `-allinone`). The auth controller alone does not use them.

### Admin UI
pnpm. `frontend-admin/package.json` pins `pnpm@10.10.0`.

# Starting the desktop app
```bash
$ ./build-fyne.sh linux     # or windows|darwin|all
$ ./bin/tunnels-app-linux-amd64
```

Windows output is `bin/tunnels-app-windows-amd64.exe`. macOS output is `bin/Tunnels-darwin-arm64.app` and `bin/Tunnels-darwin-amd64.app`.

To set the Linux capabilities by hand:

```bash
$ sudo setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' ./bin/tunnels-app-linux-amd64
```

# Starting the CLI
`go build .` in this directory names the binary `main`. `make build-client` writes `./builds/tunnels-cli` instead.

```bash
$ cd ./cmd/main
$ go build .
$ ./main
```

The same Linux `setcap` string applies to whichever client binary you run.

# Starting the server
`go build .` here names the binary `server`. `make build-server` writes `./builds/tunnels-server`.

There is no `--config` flag. `-configPath` defaults to `./config.json` and `-wgConfigPath` defaults to `./wg-config.json`. A bare `./server` starts neither the auth controller nor WireGuard.

```bash
$ cd ./server
$ go build .

# first run: configs, self-signed cert, admin user, default server, then auth + WireGuard
$ ./server -allinone -createCert selfsign

# later runs
$ ./server -auth -wg
```

`-auth` is the control plane. `-wg` is the WireGuard module. `-createCert example.com` requests a Let's Encrypt certificate instead of `selfsign`.

# Starting the admin UI
The control-plane admin UI lives in `frontend-admin`. The server embeds `server/admin_dist`.

```bash
$ ./build-ui-admin.sh
```

That script builds `frontend-admin` and copies `frontend-admin/dist` to `server/admin_dist`. Restart the server after rebuilding so the embedded files are the new ones.

# Notes about development
We accept any code, even from machine learning models
as long as the code makes sense, even small spellfixing
contributions. Just remember to run the linter before submitting.

# Random development / deployment information
### iptables
The WireGuard module installs `iptables` and `ip6tables` rules when it starts: accept the WireGuard port, drop host input from the tunnel interface, forward, NAT, and TCPMSS. Nothing in this repo installs an `OUTPUT` RST drop.

```bash
$ ./server -showActiveRules
$ ./server -wg -showNewRules
```

`-showActiveRules` prints matching rules already on the host and exits. `-showNewRules` needs `-wg` (or `-allinone`) and a controller the module can fetch. It prints the rules it would install and exits without applying them.

### Testing
The `make test*` targets run `./server/...` only. `client`, `ui`, `wg-server`, and other packages have their own tests.

```bash
# Server tests
$ make test

# Server tests, verbose
$ make test-server

# Server tests with a coverage report (coverage.html)
$ make test-coverage

# Count server tests
$ make test-count

# Server tests with the race detector
$ make test-verbose

$ go test ./server/...
$ go test ./client/... ./ui/... ./wg-server/...
```

### Linting
```
$ golangci-lint run --timeout=10m --config .golangci.yml

# Or use make
$ make lint
```
### Permissions
 - Windows: administrator
 - macOS: root (sudo)
 - Linux: `sudo setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' <client-binary>`

## Building
Goreleaser builds two binaries. The desktop app is built separately with Fyne.

| Binary | Source | Notes |
| --- | --- | --- |
| `tunnels-cli` | `./cmd/main` | Headless client. Linux, Windows, and macOS (amd64 and arm64). |
| `tunnels-server` | `./server` | Control plane. Linux only (amd64, arm64, arm). |
| `tunnels-app` | `./cmd/fyne` | Fyne desktop app. Not built by goreleaser. |

 - Snapshot binaries, no GitHub release: `./releaser-build-snapshot.sh` (`goreleaser build --snapshot`)
 - Local snapshot release: `make release` (`goreleaser release --snapshot`)
 - Publish: `./releaser-build-release.sh` (`goreleaser release`, requires `GITHUB_TOKEN`)

```bash
# Build using make
$ make build           # ./builds/tunnels-cli and ./builds/tunnels-server
$ make build-server    # ./builds/tunnels-server
$ make build-client    # ./builds/tunnels-cli
$ make build-app       # Linux desktop only, via ./build-fyne.sh linux

# Test before building
$ make pre-commit      # go mod tidy, server tests, and lint
$ make ci              # server tests and lint
```

### Fyne desktop (`tunnels-app`)
Native GUI in the same binary as the client (`./cmd/fyne`). The window
calls client functions directly (no HTTP hop for UI actions).

```bash
./build-fyne.sh linux     # or windows|darwin|all
```

# Special mentions
These are the real MVPs:

    - n00bady: creator of bluam https://github.com/n00bady/bluam
    - 0xMALVEE: for major contributions to the front-end
    - keyb1nd_'s twitch chat for the backseat debugging and support!
    - comahacks for security reviews
    - klauspost for development advice

[forks-shield]: https://img.shields.io/github/forks/tunnels-is/tunnels?style=for-the-badge&logo=github
[forks-url]: https://github.com/tunnels-is/tunnels/network/members
[stars-shield]: https://img.shields.io/github/stars/tunnels-is/tunnels?style=for-the-badge&logo=github
[stars-url]: https://github.com/tunnels-is/tunnels/stargazers
[issues-shield]: https://img.shields.io/github/issues/tunnels-is/tunnels?style=for-the-badge&logo=github
[issues-url]: https://github.com/tunnels-is/tunnels/issues
