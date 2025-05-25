[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]

# Tunnels
this readme is in progress..

# LICENSE
https://github.com/tunnels-is/tunnels/blob/main/LICENSE

# Block List Source
- https://github.com/n00bady/bluam

# Linux
## Binary permissions
sudo setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' main

# Windows
Needs to run as admin

# MacOS
Needs to run as sudo

# iptables
$ iptables -I OUTPUT -p tcp --src {interface_IP} --tcp-flags ACK,RST RST -j DROP

# Building
 - DEV: ./releaser-build-snapshot.sh
 - PROD: ./releaser-build-release.sh ( requires GITHUB_TOKEN )

# Setting up a dev environment 
## GUI
the entire gui is located in ./fronend just run `vite dev` from that dir
## Backend
The client can be found in: `cmd/main` just run `go build .` the webUI should open in your default browser.

# Live Development
[Keyb1nd_ twitch stream](https://twitch.tv/keyb1nd_)

# Special mentiones
These are the real MVPs:

    - n00bady: creator of bluam
    - 0xMALVEE: for major contributiosn to the front-end
    - keyb1nd_'s twitch chat for the backseat debugging and support!
    - comahacks for security reviews

[forks-shield]: https://img.shields.io/github/forks/tunnels-is/tunnels?style=for-the-badge&logo=github
[forks-url]: https://github.com/tunnels-is/tunnels/network/members
[stars-shield]: https://img.shields.io/github/stars/tunnels-is/tunnels?style=for-the-badge&logo=github
[stars-url]: https://github.com/tunnels-is/tunnels/stargazers
[issues-shield]: https://img.shields.io/github/issues/tunnels-is/tunnels?style=for-the-badge&logo=github
[issues-url]: https://github.com/tunnels-is/tunnels/issues
