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

	if LANEnabled {
		mux.HandleFunc("/v3/firewall", API_Firewall)
		mux.HandleFunc("/v3/devices", API_ListDevices)
	}

	mux.HandleFunc("/v3/session", API_SessionCreate)
	if VPNEnabled || LANEnabled {
		mux.HandleFunc("/v3/connect", API_AcceptUserConnections)
	}

	if AUTHEnabled {
		mux.HandleFunc("/v3/user/create", API_UserCreate)
		mux.HandleFunc("/v3/user/update", API_UserUpdate)
		mux.HandleFunc("/v3/user/adminupdate", API_UserAdminUpdate)
		mux.HandleFunc("/v3/user/login", API_UserLogin)
		mux.HandleFunc("/v3/user/logout", API_UserLogout)
		mux.HandleFunc("/v3/user/reset/password", API_UserResetPassword)
		mux.HandleFunc("/v3/user/2fa/confirm", API_UserTwoFactorConfirm)
		mux.HandleFunc("/v3/user/list", API_UserList)

		mux.HandleFunc("/v3/device/list", API_DeviceList)
		mux.HandleFunc("/v3/device/create", API_DeviceCreate)
		mux.HandleFunc("/v3/device/delete", API_DeviceDelete)
		mux.HandleFunc("/v3/device/update", API_DeviceUpdate)
		mux.HandleFunc("/v3/device", API_DeviceGet)

		mux.HandleFunc("/v3/group/create", API_GroupCreate)
		mux.HandleFunc("/v3/group/delete", API_GroupDelete)
		mux.HandleFunc("/v3/group/update", API_GroupUpdate)
		mux.HandleFunc("/v3/group/add", API_GroupAdd)
		mux.HandleFunc("/v3/group/remove", API_GroupRemove)
		mux.HandleFunc("/v3/group/list", API_GroupList)
		mux.HandleFunc("/v3/group", API_GroupGet)
		mux.HandleFunc("/v3/group/entities", API_GroupGetEntities)

		mux.HandleFunc("/v3/server", API_ServerGet)
		mux.HandleFunc("/v3/server/create", API_ServerCreate)
		mux.HandleFunc("/v3/server/update", API_ServerUpdate)
		mux.HandleFunc("/v3/servers", API_ServersForUser)

		// Tunnels public network specific
		if loadSecret("PayKey") != "" {
			mux.HandleFunc("/v3/key/activate", API_ActivateLicenseKey)
			mux.HandleFunc("/v3/user/toggle/substatus", API_UserToggleSubStatus)
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
	if key != Config.AdminAPIKey || Config.AdminAPIKey == "" {
		return false
	}
	return true
}
