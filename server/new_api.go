package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tunnels-is/tunnels/types"
	"github.com/tunnels-is/tunnels/version"
)

func launchAPIServer() {
	Config := Config.Load()
	mux := http.NewServeMux()
	var handler http.Handler = mux
	handler = bodyCloseMiddleware(handler)
	handler = corsMiddleware(handler)
	handler = securityHeadersMiddleware(handler)
	handler = loggingTimingMiddleware(handler)

	mux.HandleFunc("GET /health", healthCheckHandler)
	mux.HandleFunc("GET /{$}", healthCheckHandler)

	adminHandler := adminUIHandler()
	mux.Handle("/admin/", http.StripPrefix("/admin", adminHandler))
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})

	// todo.. WG key auth middleware ??
	mux.HandleFunc("GET /wg/server-config/fetch", API_WGServerConfigFetch)
	mux.HandleFunc("GET /wg/peers", API_WGPeers)
	mux.HandleFunc("GET /wg/peer", API_WGPeer)
	mux.HandleFunc("GET /wg/servers", API_WGServers)

	mux.HandleFunc("POST /client/user/login", API_UserLogin)
	mux.HandleFunc("POST /client/user/create", API_UserCreate)
	mux.HandleFunc("POST /client/user/reset/password", API_UserResetPassword)
	clientMW := func(h http.HandlerFunc) http.Handler {
		return applyMiddleware(h, clientAuthMiddleware)
	}

	mux.Handle("POST /client/user/logout", clientMW(API_UserLogout))
	mux.Handle("POST /client/user/update", clientMW(API_UserUpdate))
	mux.Handle("POST /client/user/2fa/confirm", clientMW(API_UserTwoFactorConfirm))
	mux.Handle("POST /client/device/list/user", clientMW(API_DeviceListUser))
	mux.Handle("POST /client/device/create", clientMW(API_DeviceCreate))
	mux.Handle("POST /client/device/delete", clientMW(API_ClientDeviceDelete))
	mux.Handle("POST /client/device", clientMW(API_DeviceGet))
	mux.Handle("POST /client/servers", clientMW(API_ServersForUser))
	mux.Handle("POST /client/server", clientMW(API_ServerGet))
	mux.Handle("GET /client/wg/config", clientMW(API_WGConfig))

	// this is only used for production environments (tunnels.is)
	// ======================================
	if loadSecret("PayKey") != "" {
		mux.Handle("POST /client/key/activate", clientMW(API_ActivateLicenseKey))
		mux.Handle("POST /client/user/toggle/substatus", clientMW(API_UserToggleSubStatus))
	}
	// ======================================

	mux.HandleFunc("POST /ui/user/login", API_AdminUILogin)
	adminMW := func(h http.HandlerFunc) http.Handler {
		return applyMiddleware(h, adminUIMiddleware)
	}

	// shared
	mux.Handle("POST /ui/device/create", adminMW(API_DeviceCreate))
	mux.Handle("POST /ui/device", adminMW(API_DeviceGet))
	mux.Handle("POST /ui/server", adminMW(API_ServerGet))
	mux.Handle("POST /ui/servers", adminMW(API_ServersForUser))
	mux.Handle("POST /ui/user/toggle/substatus", adminMW(API_UserToggleSubStatus))

	// unique
	mux.Handle("POST /ui/user/logout", adminMW(API_AdminUILogout))
	mux.Handle("POST /ui/user/list", adminMW(API_AdminUserList))
	mux.Handle("POST /ui/user/adminupdate", adminMW(API_UserAdminUpdate))

	mux.Handle("POST /ui/device/list", adminMW(API_AdminDeviceList))
	mux.Handle("POST /ui/device/delete", adminMW(API_AdminDeviceDelete))
	mux.Handle("POST /ui/device/update", adminMW(API_AdminDeviceUpdate))

	mux.Handle("POST /ui/group/create", adminMW(API_AdminGroupCreate))
	mux.Handle("POST /ui/group/delete", adminMW(API_AdminGroupDelete))
	mux.Handle("POST /ui/group/update", adminMW(API_AdminGroupUpdate))
	mux.Handle("POST /ui/group/add", adminMW(API_AdminGroupAdd))
	mux.Handle("POST /ui/group/remove", adminMW(API_AdminGroupRemove))
	mux.Handle("POST /ui/group/list", adminMW(API_AdminGroupList))
	mux.Handle("POST /ui/group/entities", adminMW(API_AdminGroupGetEntities))
	mux.Handle("POST /ui/group", adminMW(API_AdminGroupGet))

	mux.Handle("POST /ui/server/create", adminMW(API_AdminServerCreate))
	mux.Handle("POST /ui/server/update", adminMW(API_AdminServerUpdate))

	tlsConfig := APITLSConfig.Load()

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

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	rs := new(types.HealthResponse)
	rs.ServerVersion = version.Version
	cfg := Config.Load()
	rs.ClientVersion = cfg.ClientVersion
	rs.Uptime = types.Uptime
	enc := json.NewEncoder(w)
	enc.Encode(rs)
}

func loggingTimingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		conf := Config.Load()
		if conf.LogAPIHosts && !disableLogs {
			log.Printf("-> %s %s %s", r.RemoteAddr, r.Method, r.URL.RequestURI())
		} else {
			if !disableLogs {
				log.Printf("-> %s %s", r.Method, r.URL.RequestURI())
			}
		}

		next.ServeHTTP(w, r)
		if !disableLogs {
			duration := time.Since(startTime)
			log.Printf("<- %s %s completed in %d ms",
				r.Method,
				r.URL.RequestURI(),
				duration.Milliseconds(),
			)
		}
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-API-KEY, X-Device-Token, X-UID, X-Email")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")

		path := r.URL.Path
		if strings.HasPrefix(path, "/admin") || strings.HasPrefix(path, "/ui/") {
			w.Header().Set("X-Frame-Options", "DENY")
		}

		if strings.HasPrefix(path, "/admin") {
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data:; font-src 'self'; form-action 'self'; "+
					"frame-ancestors 'none'; base-uri 'self'; object-src 'none'")
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
	responsePayload := map[string]string{"Error": msg}
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
	if Config.AdminAPIKey != "" {
		if subtle.ConstantTimeCompare([]byte(key), []byte(Config.AdminAPIKey)) == 1 {
			return true
		}
	}
	return false
}
