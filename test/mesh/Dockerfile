FROM alpine:3.20
RUN apk add --no-cache iptables ip6tables iproute2 curl iputils \
	&& mkdir -p /data
COPY server /server
COPY meshpeer /meshpeer
COPY dummyhttp /dummyhttp
WORKDIR /data
