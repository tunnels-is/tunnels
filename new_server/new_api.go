package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"
)

func launchAPIServer() {

	Config := Config.Load()
	mux := http.NewServeMux()
	var handler http.Handler = mux
	handler = bodyCloseMiddleware(handler)
	handler = corsMiddleware(handler)
	handler = loggingTimingMiddleware(handler)

	mux.HandleFunc("/health", healthCheckHandler)
	mux.HandleFunc("/", healthCheckHandler)

	if LANEnabled {
		mux.HandleFunc("/firewall", HTTP_Firewall)
		mux.HandleFunc("/devices", HTTP_ListDevices)
	}

	if AUTHEnabled {
		mux.HandleFunc("/user/create", APICreateUser)
		mux.HandleFunc("/user/update", APIUpdateUser)
		// mux.HandleFunc("/user/update")
		// mux.HandleFunc("/user/enable", nil)
		mux.HandleFunc("/user/login", APILoginUser)
		mux.HandleFunc("/user/logout", APILogoutUser)
		mux.HandleFunc("/user/2fa/confirm", APITwoFactorConfirm)
		mux.HandleFunc("/user/reset/password", nil)
		mux.HandleFunc("/user/reset/code", nil)

		mux.HandleFunc("/groupd/create", APICreateGroup)
		mux.HandleFunc("/groupd/update", APIUpdateGroup)
		mux.HandleFunc("/groupd/add", APIAddToGroup)
		mux.HandleFunc("/group", APIGetGroup)

		mux.HandleFunc("/servers/create", nil)
		mux.HandleFunc("/servers/update", APIUpdateServer)
		mux.HandleFunc("/servers", APIGetServers)

		mux.HandleFunc("/session/public", nil)

		// Tunnels public network specific
		mux.HandleFunc("/key/activate", nil)
		mux.HandleFunc("/user/toggle/substatus", nil)
	}

	tlsConfig := &tls.Config{
		Certificates:             []tls.Certificate{*KeyPair.Load()},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		// CurvePreferences:         []tls.CurveID{tls.CurveP256, tls.X25519, tls.CurveP521},
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP521},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	addr := fmt.Sprintf("%s:%s",
		Config.APIIP,
		Config.APIPort,
	)

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		IdleTimeout:  time.Second * 60,
		WriteTimeout: time.Second * 60,
		ReadTimeout:  time.Second * 60,
		TLSConfig:    tlsConfig,
	}

	logger.Info("API Server launching", slog.Any("address", addr))
	err := server.ListenAndServeTLS("", "")
	if err != nil && err != http.ErrServerClosed {
		logger.Error("API Server error", slog.Any("err", err))
	}
}

// healthCheckHandler responds with a simple OK status for health checks.
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

func loggingTimingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		log.Printf("-> %s %s from %s", r.Method, r.URL.RequestURI(), r.RemoteAddr)
		next.ServeHTTP(w, r)
		duration := time.Since(startTime)
		log.Printf("<- %s %s completed in %dms",
			r.Method,
			r.URL.RequestURI(),
			duration.Milliseconds(),
		)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func bodyCloseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r.Body != nil {
				_ = r.Body.Close()
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func senderr(w http.ResponseWriter, code int, msg string, slogArgs ...any) {
	logger.Error(msg, slogArgs...)
	responsePayload := map[string]string{"Message": msg}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(responsePayload)
	if err != nil {
		logger.Error("unable to write JSON errResponse:", slog.Any("err", err))
	}
}

func HTTP_validateKey(r *http.Request) (ok bool) {
	key := r.Header.Get("X-API-KEY")
	Config := Config.Load()
	if key != Config.AdminApiKey || Config.AdminApiKey != "" {
		return false
	}
	return true
}
