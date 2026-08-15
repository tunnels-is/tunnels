FROM alpine:3.20
RUN apk add --no-cache iptables ip6tables iproute2 curl \
	&& mkdir -p /data
COPY server /server
COPY tunnels /tunnels
WORKDIR /data
