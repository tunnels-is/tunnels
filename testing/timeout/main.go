package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httptrace"
	"time"
)

func main() {
	// A server that is slow to respond after connection.
	// httpbin.org/delay/5 will wait 5 seconds before responding.
	const slowServerURL = "https://httpbin.org/delay/5"

	req, _ := http.NewRequest("GET", slowServerURL, nil)

	// Create a trace object.
	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			log.Printf("DNS Start: %+v\n", info)
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			log.Printf("DNS Done: %+v\n", info)
		},
		ConnectStart: func(network, addr string) {
			log.Printf("Connect Start: %s %s\n", network, addr)
		},
		ConnectDone: func(network, addr string, err error) {
			log.Printf("Connect Done: %s %s (err: %v)\n", network, addr, err)
		},
		GotConn: func(info httptrace.GotConnInfo) {
			log.Printf("Got Connection: reused: %v, from_idle: %v\n", info.Reused, info.WasIdle)
		},
		TLSHandshakeStart: func() {
			log.Println("TLS Handshake Start")
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			log.Printf("TLS Handshake Done (err: %v)\n", err)
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			log.Printf("Wrote Request (err: %v)\n", info.Err)
		},
		GotFirstResponseByte: func() {
			log.Println("Got First Response Byte")
		},
	}

	// Attach the trace to the request's context.
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	// Now, create a parent context with a timeout that is SHORTER than the server delay.
	ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancel()

	// Re-apply the timeout context to the request.
	req = req.WithContext(ctx)

	log.Println("Making request with 3s timeout to a 5s-delayed server...")
	_, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Request failed: %v", err)
	}
}
