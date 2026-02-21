# TUNNELS BANDWIDTH TESTING AND IMPROVEMENTS
I want you to analyze the code inside ./client and ./server AND analyze the remote test server settings. The goal is to improve bandwidth throughput while connected to the tunnels VPN. the "client" runs locally as a cli and the "server" is deployed on the remote test server inside /opt/tunnels. The server has already been configured, if you want to modify the code you just have to rebuild the server binary and scp it.

# Current bandwidth
The current bandwidth measured using the test mentioned below is 5MB/s. Without the VPN we get 50MB/s.. this indicates some issues with the packet flow.

# Available resource
 - pre-existing packet capture: debug.tcpdump
 - pre-existing cli tunnels.conf in: ./cmd/main
 - remote test server: ssh -i /home/sveinn/.ssh/suko admin@74.63.223.157
 - server binary location on remove server: /opt/tunnels/tunnels
 - vpn client code: ./client
 - vpn server code: ./server
 - test command: wget https://dal.download.datapacket.com/1000mb.bin (feel free to try other tests too)
 - mtr 
 - ping
 - tcpdump

# how to build:
 - client(inside ./cmd/main): go build . 
 - server(inside ./server): go build . 
