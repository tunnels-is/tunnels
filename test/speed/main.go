package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

const (
	magic    = "SPD1"
	modeUp   = 1
	modeDown = 2
	modeBoth = 3

	chunkSize = 256 * 1024
	maxChunk  = 1 << 20
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "server":
		err = runServer(os.Args[2:])
	case "client":
		err = runClient(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage:
  speedtest server -listen 10.0.1.3:5201
  speedtest client -server 10.0.1.3:5201 -time 10 -dir both

  -dir is upload (client to server), download (server to client), or both.
`)
}

func runServer(args []string) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	listen := fs.String("listen", ":5201", "listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ln, err := listenRetry(*listen, 30*time.Second)
	if err != nil {
		return err
	}
	fmt.Printf("speedtest listening %s\n", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		handle(conn)
	}
}

func listenRetry(addr string, wait time.Duration) (net.Listener, error) {
	deadline := time.Now().Add(wait)
	for {
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("listen %s: %w", addr, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func runClient(args []string) error {
	fs := flag.NewFlagSet("client", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	server := fs.String("server", "", "server host:port")
	seconds := fs.Uint("time", 10, "seconds per direction (1-300)")
	dir := fs.String("dir", "both", "upload, download, or both")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *server == "" {
		return fmt.Errorf("-server is required")
	}
	mode, err := parseMode(*dir)
	if err != nil {
		return err
	}
	if *seconds == 0 || *seconds > 300 {
		return fmt.Errorf("-time must be 1-300")
	}
	conn, err := net.Dial("tcp", *server)
	if err != nil {
		return err
	}
	defer conn.Close()
	tune(conn)
	dur := time.Duration(*seconds) * time.Second
	budget := dur + 30*time.Second
	if mode == modeBoth {
		budget = 2*dur + 30*time.Second
	}
	_ = conn.SetDeadline(time.Now().Add(budget))
	if err := writeHeader(conn, mode, uint16(*seconds)); err != nil {
		return err
	}
	if mode == modeUp || mode == modeBoth {
		if _, _, err := sendFor(conn, dur); err != nil {
			return fmt.Errorf("upload: %w", err)
		}
		n, elapsed, err := readStats(conn)
		if err != nil {
			return fmt.Errorf("upload result: %w", err)
		}
		printResult("upload", n, elapsed)
	}
	if mode == modeDown || mode == modeBoth {
		n, elapsed, err := recvUntilEnd(conn)
		if err != nil {
			return fmt.Errorf("download: %w", err)
		}
		printResult("download", n, elapsed)
	}
	return nil
}

func handle(conn net.Conn) {
	defer conn.Close()
	tune(conn)
	var hdr [7]byte
	if err := readFull(conn, hdr[:]); err != nil {
		fmt.Fprintf(os.Stderr, "header: %v\n", err)
		return
	}
	if string(hdr[:4]) != magic {
		fmt.Fprintf(os.Stderr, "bad magic from %s\n", conn.RemoteAddr())
		return
	}
	mode := hdr[4]
	seconds := binary.BigEndian.Uint16(hdr[5:])
	if seconds == 0 || seconds > 300 || (mode != modeUp && mode != modeDown && mode != modeBoth) {
		fmt.Fprintf(os.Stderr, "bad request from %s\n", conn.RemoteAddr())
		return
	}
	dur := time.Duration(seconds) * time.Second
	budget := dur + 30*time.Second
	if mode == modeBoth {
		budget = 2*dur + 30*time.Second
	}
	_ = conn.SetDeadline(time.Now().Add(budget))
	fmt.Printf("client %s dir=%s time=%ds\n", conn.RemoteAddr(), modeName(mode), seconds)
	if mode == modeUp || mode == modeBoth {
		n, elapsed, err := recvUntilEnd(conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "upload: %v\n", err)
			return
		}
		printResult("upload", n, elapsed)
		if err := writeStats(conn, n, elapsed); err != nil {
			fmt.Fprintf(os.Stderr, "upload stats: %v\n", err)
			return
		}
	}
	if mode == modeDown || mode == modeBoth {
		n, elapsed, err := sendFor(conn, dur)
		if err != nil {
			fmt.Fprintf(os.Stderr, "download: %v\n", err)
			return
		}
		printResult("download", n, elapsed)
	}
}

func parseMode(s string) (byte, error) {
	switch s {
	case "upload":
		return modeUp, nil
	case "download":
		return modeDown, nil
	case "both":
		return modeBoth, nil
	default:
		return 0, fmt.Errorf("-dir must be upload, download, or both")
	}
}

func modeName(mode byte) string {
	switch mode {
	case modeUp:
		return "upload"
	case modeDown:
		return "download"
	default:
		return "both"
	}
}

func writeHeader(w io.Writer, mode byte, seconds uint16) error {
	var buf [7]byte
	copy(buf[:4], magic)
	buf[4] = mode
	binary.BigEndian.PutUint16(buf[5:], seconds)
	return writeFull(w, buf[:])
}

func sendFor(conn net.Conn, d time.Duration) (int64, time.Duration, error) {
	payload := make([]byte, chunkSize)
	for i := range payload {
		payload[i] = 0x5a
	}
	frame := make([]byte, 4+chunkSize)
	binary.BigEndian.PutUint32(frame[:4], chunkSize)
	copy(frame[4:], payload)
	var n int64
	start := time.Now()
	deadline := start.Add(d)
	for time.Now().Before(deadline) {
		if err := writeFull(conn, frame); err != nil {
			return n, time.Since(start), err
		}
		n += chunkSize
	}
	var end [4]byte
	if err := writeFull(conn, end[:]); err != nil {
		return n, time.Since(start), err
	}
	return n, time.Since(start), nil
}

func recvUntilEnd(r io.Reader) (int64, time.Duration, error) {
	var n int64
	var start time.Time
	hdr := make([]byte, 4)
	buf := make([]byte, chunkSize)
	for {
		if err := readFull(r, hdr); err != nil {
			return n, since(start), err
		}
		sz := binary.BigEndian.Uint32(hdr)
		if sz == 0 {
			return n, since(start), nil
		}
		if sz > maxChunk {
			return n, since(start), fmt.Errorf("chunk %d exceeds %d", sz, maxChunk)
		}
		if start.IsZero() {
			start = time.Now()
		}
		left := int(sz)
		for left > 0 {
			chunk := buf
			if left < len(chunk) {
				chunk = buf[:left]
			}
			nr, err := io.ReadFull(r, chunk)
			n += int64(nr)
			left -= nr
			if err != nil {
				return n, since(start), err
			}
		}
	}
}

func writeStats(w io.Writer, n int64, d time.Duration) error {
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[:8], uint64(n))
	binary.BigEndian.PutUint64(buf[8:], uint64(d.Nanoseconds()))
	return writeFull(w, buf[:])
}

func readStats(r io.Reader) (int64, time.Duration, error) {
	var buf [16]byte
	if err := readFull(r, buf[:]); err != nil {
		return 0, 0, err
	}
	n := int64(binary.BigEndian.Uint64(buf[:8]))
	elapsed := time.Duration(binary.BigEndian.Uint64(buf[8:]))
	return n, elapsed, nil
}

func printResult(dir string, n int64, d time.Duration) {
	if d <= 0 {
		d = time.Nanosecond
	}
	mbit := float64(n*8) / d.Seconds() / 1e6
	mb := float64(n) / d.Seconds() / 1e6
	fmt.Printf("%-8s %6.2fs  %8.1f MB  %8.1f MB/s  %8.1f Mbit/s\n", dir, d.Seconds(), float64(n)/1e6, mb, mbit)
}

func tune(conn net.Conn) {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tc.SetNoDelay(true)
}

func since(start time.Time) time.Duration {
	if start.IsZero() {
		return 0
	}
	return time.Since(start)
}

func writeFull(w io.Writer, b []byte) error {
	_, err := writeAll(w, b)
	return err
}

func writeAll(w io.Writer, b []byte) (int, error) {
	n := 0
	for len(b) > 0 {
		nw, err := w.Write(b)
		n += nw
		if err != nil {
			return n, err
		}
		b = b[nw:]
	}
	return n, nil
}

func readFull(r io.Reader, b []byte) error {
	_, err := io.ReadFull(r, b)
	return err
}
