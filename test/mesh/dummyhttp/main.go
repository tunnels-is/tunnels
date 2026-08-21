package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := flag.String("addr", ":9090", "listen address (bind to a WireGuard IP to avoid LAN shortcuts)")
	name := flag.String("name", "dummy", "identity string included in every response")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "DUMMY_OK name=%s host=%s remote=%s\n", *name, r.Host, r.RemoteAddr)
	})

	var ln net.Listener
	deadline := time.Now().Add(30 * time.Second)
	for {
		var err error
		ln, err = net.Listen("tcp", *addr)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "listen:", err)
			os.Exit(1)
		}
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Printf("dummyhttp listening addr=%s name=%s\n", ln.Addr(), *name)
	if err := http.Serve(ln, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
