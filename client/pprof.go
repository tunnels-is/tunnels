package client

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"
)

const defaultPprofAddr = "127.0.0.1:6060"

func startPprofIfEnabled() {
	state := STATE.Load()
	if state == nil || !state.Pprof {
		return
	}
	go func() {
		if err := servePprof(state.PprofAddr); err != nil && err != http.ErrServerClosed {
			ERROR("pprof server:", err)
		}
	}()
}

func servePprof(addr string) error {
	addr, err := canonicalizePprofAddr(addr)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	INFO("pprof listening on http://", ln.Addr().String(), "/debug/pprof/")
	srv := &http.Server{
		Handler:           pprofMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.Serve(ln)
}

func pprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

// canonicalizePprofAddr requires a loopback host. Heap profiles can contain
// tokens and keys, so this must never bind a public interface.
func canonicalizePprofAddr(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = defaultPprofAddr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("pprof address: %w", err)
	}
	if host == "" || strings.EqualFold(host, "localhost") {
		host = "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", fmt.Errorf("pprof must bind to loopback, got %q", addr)
	}
	return net.JoinHostPort(ip.String(), port), nil
}
