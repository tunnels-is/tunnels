package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
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
	handler = securityHeadersMiddleware(handler)
	handler = loggingTimingMiddleware(handler)

	mux.HandleFunc("GET /health", healthCheckHandler)
	mux.HandleFunc("GET /{$}", healthCheckHandler)

	adminHandler := adminUIHandler()
	mux.Handle("/admin/", http.StripPrefix("/admin", adminHandler))
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})

	registerWGRoutes(mux)
	registerClientRoutes(mux)
	registerAdminRoutes(mux)

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

func registerWGRoutes(mux *http.ServeMux) {
	wgServerMW := func(h http.HandlerFunc) http.Handler {
		return applyMiddleware(h, wireGuardServerKeyCheck)
	}
	mux.Handle("GET /wg/server-config/fetch", wgServerMW(handleWGServerConfigFetch))
	mux.Handle("GET /wg/peers", wgServerMW(handleWGPeers))
	mux.Handle("GET /wg/peer", wgServerMW(handleWGPeer))
	mux.Handle("GET /wg/mesh", wgServerMW(handleWGMesh))
}

func registerClientRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /client/user/login", handleClientLogin)
	mux.HandleFunc("POST /client/user/create", handleClientUserCreate)
	mux.HandleFunc("POST /client/user/reset/password", handleClientResetPassword)
	clientMW := func(h http.HandlerFunc) http.Handler {
		return applyMiddleware(h, clientAuthMiddleware)
	}

	mux.Handle("POST /client/user/logout", clientMW(handleClientLogout))
	mux.Handle("POST /client/user/update", clientMW(handleClientUserUpdate))
	mux.Handle("POST /client/user/2fa/confirm", clientMW(handleClientTwoFactorConfirm))
	mux.Handle("POST /client/device/list/user", clientMW(handleClientDeviceList))
	mux.Handle("POST /client/device/create", clientMW(handleClientDeviceCreate))
	mux.Handle("POST /client/device/delete", clientMW(handleClientDeviceDelete))
	mux.Handle("POST /client/device", clientMW(handleClientDeviceGet))
	mux.Handle("POST /client/servers", clientMW(handleClientServers))
	mux.Handle("POST /client/servers/country", clientMW(handleClientServersByCountry))
	mux.Handle("POST /client/server", clientMW(handleClientServerGet))
	mux.Handle("GET /client/wg/config", clientMW(handleWGConfig))

	if loadSecret("PayKey") != "" {
		mux.Handle("POST /client/key/activate", clientMW(handleClientActivateLicense))
	}
}

func registerAdminRoutes(mux *http.ServeMux) {
	adminAPIKeyMW := func(h http.HandlerFunc) http.Handler {
		return applyMiddleware(h, xAdminAPIKeyMiddleware)
	}
	mux.Handle("POST /ui/device/create", adminAPIKeyMW(handleAdminDeviceCreate))

	mux.HandleFunc("POST /ui/user/login", handleAdminLogin)
	adminMW := func(h http.HandlerFunc) http.Handler {
		return applyMiddleware(h, adminUIMiddleware)
	}

	mux.Handle("POST /ui/servers", adminMW(handleAdminServerList))
	mux.Handle("POST /ui/server", adminMW(handleAdminServerGet))

	mux.Handle("POST /ui/device", adminMW(handleAdminDeviceGet))

	mux.Handle("POST /ui/user/logout", adminMW(handleAdminLogout))
	mux.Handle("POST /ui/user/create", adminMW(handleAdminUserCreate))
	mux.Handle("POST /ui/user", adminMW(handleAdminUserGet))
	mux.Handle("POST /ui/user/list", adminMW(handleAdminUserList))
	mux.Handle("POST /ui/user/latest", adminMW(handleAdminUserLatest))
	mux.Handle("POST /ui/user/search", adminMW(handleAdminUserSearch))
	mux.Handle("POST /ui/user/adminupdate", adminMW(handleAdminUserUpdate))
	mux.Handle("POST /ui/user/delete", adminMW(handleAdminUserDelete))

	mux.Handle("POST /ui/device/list", adminMW(handleAdminDeviceList))
	mux.Handle("POST /ui/device/delete", adminMW(handleAdminDeviceDelete))
	mux.Handle("POST /ui/device/update", adminMW(handleAdminDeviceUpdate))

	mux.Handle("POST /ui/group/create", adminMW(handleAdminGroupCreate))
	mux.Handle("POST /ui/group/delete", adminMW(handleAdminGroupDelete))
	mux.Handle("POST /ui/group/update", adminMW(handleAdminGroupUpdate))
	mux.Handle("POST /ui/group/add", adminMW(handleAdminGroupAdd))
	mux.Handle("POST /ui/group/remove", adminMW(handleAdminGroupRemove))
	mux.Handle("POST /ui/group/list", adminMW(handleAdminGroupList))
	mux.Handle("POST /ui/group/entities", adminMW(handleAdminGroupGetEntities))
	mux.Handle("POST /ui/group", adminMW(handleAdminGroupGet))

	mux.Handle("POST /ui/server/create", adminMW(handleAdminServerCreate))
	mux.Handle("POST /ui/server/update", adminMW(handleAdminServerUpdate))
	mux.Handle("POST /ui/server/delete", adminMW(handleAdminServerDelete))

	mux.Handle("POST /ui/wan/create", adminMW(handleAdminWANCreate))
	mux.Handle("POST /ui/wan/update", adminMW(handleAdminWANUpdate))
	mux.Handle("POST /ui/wan/delete", adminMW(handleAdminWANDelete))
	mux.Handle("POST /ui/wan/list", adminMW(handleAdminWANList))
	mux.Handle("POST /ui/wan", adminMW(handleAdminWANGet))

	mux.Handle("POST /ui/meshgroup/create", adminMW(handleAdminMeshGroupCreate))
	mux.Handle("POST /ui/meshgroup/update", adminMW(handleAdminMeshGroupUpdate))
	mux.Handle("POST /ui/meshgroup/delete", adminMW(handleAdminMeshGroupDelete))
	mux.Handle("POST /ui/meshgroup/list", adminMW(handleAdminMeshGroupList))
	mux.Handle("POST /ui/meshgroup", adminMW(handleAdminMeshGroupGet))
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

func adminAPIKeyValid(r *http.Request) (ok bool) {
	key := r.Header.Get("X-API-KEY")
	Config := Config.Load()
	if Config.AdminAPIKey != "" {
		if subtle.ConstantTimeCompare([]byte(key), []byte(Config.AdminAPIKey)) == 1 {
			return true
		}
	}
	return false
}

func sendObject(w http.ResponseWriter, obj any) {
	w.WriteHeader(200)
	var err error
	enc := json.NewEncoder(w)
	u, ok := obj.(*User)
	if ok {
		u.RemoveSensitiveInformation()
		err = enc.Encode(u)
	} else {
		err = enc.Encode(obj)
	}
	if err != nil {
		senderr(w, 500, "unable to encode response object")
		return
	}
}
