package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
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
	handler = loggingTimingMiddleware(handler)

	mux.HandleFunc("/health", healthCheckHandler)
	mux.HandleFunc("/", healthCheckHandler)

	adminHandler := adminUIHandler()
	mux.Handle("/admin/", http.StripPrefix("/admin", adminHandler))
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})

	// WireGuard server config fetch — authenticated via X-WG-KEY in the handler itself
	mux.HandleFunc("/wg/server-config/fetch", API_WGServerConfigFetch)

	if AUTHEnabled {
		// ----------------------------------------------------------------
		// Public client endpoints (no auth required)
		// ----------------------------------------------------------------
		mux.HandleFunc("/client/user/login", API_UserLogin)
		mux.HandleFunc("/client/user/create", API_UserCreate)
		mux.HandleFunc("/client/user/reset/password", API_UserResetPassword)

		// ----------------------------------------------------------------
		// Protected client endpoints (clientAuthMiddleware)
		// ----------------------------------------------------------------
		clientMW := func(h http.HandlerFunc) http.Handler {
			return applyMiddleware(h, xAPIKeyMiddleware, clientAuthMiddleware)
		}

		mux.Handle("/client/user/logout", clientMW(API_UserLogout))
		mux.Handle("/client/user/update", clientMW(API_UserUpdate))
		mux.Handle("/client/user/2fa/confirm", clientMW(API_UserTwoFactorConfirm))
		mux.Handle("/client/device/list/user", clientMW(API_DeviceListUser))
		mux.Handle("/client/device/create", clientMW(API_DeviceCreate))
		mux.Handle("/client/device", clientMW(API_DeviceGet))
		mux.Handle("/client/servers", clientMW(API_ServersForUser))
		mux.Handle("/client/server", clientMW(API_ServerGet))
		mux.Handle("/client/wg/config", clientMW(API_WGConfig))

		if loadSecret("PayKey") != "" {
			mux.Handle("/client/key/activate", clientMW(API_ActivateLicenseKey))
			mux.Handle("/client/user/toggle/substatus", clientMW(API_UserToggleSubStatus))
		}

		// ----------------------------------------------------------------
		// Admin UI login (public — sets the admin_session cookie)
		// ----------------------------------------------------------------
		mux.HandleFunc("/ui/user/login", API_AdminUILogin)

		// ----------------------------------------------------------------
		// Protected admin UI endpoints (adminUIMiddleware)
		// ----------------------------------------------------------------
		adminMW := func(h http.HandlerFunc) http.Handler {
			return applyMiddleware(h, xAPIKeyMiddleware, adminUIMiddleware)
		}

		// User management
		mux.Handle("/ui/user/logout", adminMW(API_AdminUILogout))
		mux.Handle("/ui/user/list", adminMW(API_UserList))
		mux.Handle("/ui/user/adminupdate", adminMW(API_UserAdminUpdate))

		// Device management
		mux.Handle("/ui/device/list", adminMW(API_DeviceList))
		mux.Handle("/ui/device/create", adminMW(API_DeviceCreate))
		mux.Handle("/ui/device/delete", adminMW(API_DeviceDelete))
		mux.Handle("/ui/device/update", adminMW(API_DeviceUpdate))
		mux.Handle("/ui/device", adminMW(API_DeviceGet))

		// Group management
		mux.Handle("/ui/group/create", adminMW(API_GroupCreate))
		mux.Handle("/ui/group/delete", adminMW(API_GroupDelete))
		mux.Handle("/ui/group/update", adminMW(API_GroupUpdate))
		mux.Handle("/ui/group/add", adminMW(API_GroupAdd))
		mux.Handle("/ui/group/remove", adminMW(API_GroupRemove))
		mux.Handle("/ui/group/list", adminMW(API_GroupList))
		mux.Handle("/ui/group/entities", adminMW(API_GroupGetEntities))
		mux.Handle("/ui/group", adminMW(API_GroupGet))

		// Server management
		mux.Handle("/ui/server", adminMW(API_ServerGet))
		mux.Handle("/ui/server/create", adminMW(API_ServerCreate))
		mux.Handle("/ui/server/update", adminMW(API_ServerUpdate))
		mux.Handle("/ui/servers", adminMW(API_ServersForUser))

		// WireGuard peer/server info
		mux.Handle("/ui/wg/peers", adminMW(API_WGPeers))
		mux.Handle("/ui/wg/config", adminMW(API_WGConfig))
		mux.Handle("/ui/wg/servers", adminMW(API_WGServers))

		// WireGuard server config management
		mux.Handle("/ui/wg/server-config", adminMW(API_WGServerConfigCreate))
		mux.Handle("/ui/wg/server-config/list", adminMW(API_WGServerConfigList))
		mux.Handle("/ui/wg/server-config/update", adminMW(API_WGServerConfigUpdate))
		mux.Handle("/ui/wg/server-config/get", adminMW(API_WGServerConfigGet))
		mux.Handle("/ui/wg/server-config/assign", adminMW(API_WGServerConfigAssign))

		// Network management
		mux.Handle("/ui/network/list", adminMW(API_NetworkList))
		mux.Handle("/ui/network/update", adminMW(API_NetworkUpdate))

		if loadSecret("PayKey") != "" {
			mux.Handle("/ui/user/toggle/substatus", adminMW(API_UserToggleSubStatus))
		}
	}

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
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
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
		if key == Config.AdminAPIKey {
			return true
		}
	}
	return false
}
