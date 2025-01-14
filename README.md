[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]

# Tunnels
... don't worry, we are writing this right now.
... We will be addding an business source license to this soon (tm)

# Liecens
We are still deciding on a license, but it will be an open source ( non-commercial ) one.

# Special Mentioned
 - n00bady ( creator of bluam )

# Main golang module licenses
go-ping: MIT
google/uuid: BSD 3-Clause "New" or "Revised" License
jackpal/gateway: Using a copy of googles BSD 3-Claus license (not a valid license)
github.com/miekg/dns: BSD 3-Clause "New" or "Revised" License
github.com/shirou/gopsutil: BSD license
github.com/vishvananda/netlink: Apache License 2.0
github.com/xlzd/gotp: MIT
go.mongodb.org/mongo-driver: Apache License 2.0
kernel.org/pub/linux/libs/security/libcap/cap: BSD-3-Clause OR GPL-2.0-only
 - https://sites.google.com/site/fullycapable

# Block List Source
https://github.com/n00bady/bluam
Special thanks to Kazaboo from twitch!

# Live Development
https://twitch.tv/keyb1nd_


# Linux
## Binary permissions
sudo setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' main

# Windows
Needs to run as admin

# MacOS
Needs to run as sudo

# private server
 - run server with custom cert generation
 - create server in UI with serial
 - create tunnels for server
    - assign server to tunnel
    - give tunnel ip + port

# notes
InterfaceIP == vpn outgoing IP

# iptables
$ iptables -I OUTPUT -p tcp --src {interface_IP} --tcp-flags ACK,RST RST -j DROP


# Building
 - toggle debug to false in app.jsx
 - $ goreleaser release --clean ( add GITHUB_TOKEN )
 - $ goreleaser build --snapchot --clean 

# Setting up a dev environment 
## GUI
the entire gui is located in ./fronend just run `npm run dev` from that dir
## Backend
We have two backends, one is the `iot` client and the other is the full client.
The full client can be found in: `cmd/main` just run `go build .` and then start the client.

NOTE: macos requies sudo, windows requires admin, linux requires set cap:
```bash
$ sudo setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' main
```





[forks-shield]: https://img.shields.io/github/forks/tunnels-is/tunnels?style=for-the-badge&logo=github
[forks-url]: https://github.com/tunnels-is/tunnels/network/members
[stars-shield]: https://img.shields.io/github/stars/tunnels-is/tunnels?style=for-the-badge&logo=github
[stars-url]: https://github.com/tunnels-is/tunnels/stargazers
[issues-shield]: https://img.shields.io/github/issues/tunnels-is/tunnels?style=for-the-badge&logo=github
[issues-url]: https://github.com/tunnels-is/tunnels/issues
