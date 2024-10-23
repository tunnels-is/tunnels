[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]

# Tunnels
... don't worry, we are writing this right now.
... We will be addding an open source license to this soon (tm)

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


# Test server
93.95.231.66:444
cert: certs/test-server.crt
consoller sign cert: certs/controller.crt
serial: 1ebad4235e13f3dbc437d0a6804fc03f
- it's not always running

# Building
 - toggle debug to false in app.jsx
 - $ goreleaser release --clean ( add GITHUB_TOKEN )
 - $ goreleaser build --snapchot --clean 





[forks-shield]: https://img.shields.io/github/forks/tunnels-is/tunnels?style=for-the-badge&logo=github
[forks-url]: https://github.com/tunnels-is/tunnels/network/members
[stars-shield]: https://img.shields.io/github/stars/tunnels-is/tunnels?style=for-the-badge&logo=github
[stars-url]: https://github.com/tunnels-is/tunnels/stargazers
[issues-shield]: https://img.shields.io/github/issues/tunnels-is/tunnels?style=for-the-badge&logo=github
[issues-url]: https://github.com/tunnels-is/tunnels/issues
