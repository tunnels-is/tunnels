package main

import (
	"net"
	"testing"
	"time"
)

func TestUploadAndDownload(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	errc := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errc <- err
			return
		}
		handle(conn)
		errc <- nil
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := writeHeader(conn, modeBoth, 1); err != nil {
		t.Fatal(err)
	}
	sent, _, err := sendFor(conn, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	gotN, _, err := readStats(conn)
	if err != nil {
		t.Fatal(err)
	}
	if gotN != sent {
		t.Fatalf("server received %d, client sent %d", gotN, sent)
	}
	n, elapsed, err := recvUntilEnd(conn)
	if err != nil {
		t.Fatal(err)
	}
	if n < chunkSize || elapsed < 500*time.Millisecond {
		t.Fatalf("download n=%d elapsed=%s", n, elapsed)
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
}
