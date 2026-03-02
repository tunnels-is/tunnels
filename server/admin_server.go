package main

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func launchAdminServer() {
	if !AUTHEnabled {
		return
	}

	cfg := Config.Load()

	// Proxy all /v3/ calls through to the TLS API server.
	// InsecureSkipVerify is fine here since it's a loopback hop to ourselves.
	apiTarget, _ := url.Parse(fmt.Sprintf("https://%s:%s", cfg.APIIP, cfg.APIPort))
	proxy := httputil.NewSingleHostReverseProxy(apiTarget)
	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	mux := http.NewServeMux()
	var handler http.Handler = mux
	handler = bodyCloseMiddleware(handler)
	handler = corsMiddleware(handler)
	handler = loggingTimingMiddleware(handler)

	// Admin UI static files
	adminHandler := adminUIHandler()
	mux.Handle("/admin/", http.StripPrefix("/admin", adminHandler))
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})

	// Proxy API calls to the TLS API server
	mux.Handle("/v3/", proxy)

	addr := "0.0.0.0:8080"
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		IdleTimeout:  time.Second * 60,
		WriteTimeout: time.Second * 60,
		ReadTimeout:  time.Second * 60,
	}

	logger.Info("Admin HTTP server launching", slog.Any("address", addr))
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		logger.Error("Admin server error", slog.Any("err", err))
	}
}
