package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

type contextKey string

const (
	contextKeyUser          contextKey = "user"
	contextKeyIsAdminAPIKey contextKey = "isAdminAPIKey"
	contextKeyServer        contextKey = "server"
	contextKeyDeviceToken   contextKey = "deviceToken"
)

func getUserFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(contextKeyUser).(*User)
	return user
}

func isAdminAPIKeyFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(contextKeyIsAdminAPIKey).(bool)
	return v
}

func getServerFromContext(ctx context.Context) *types.Server {
	user, _ := ctx.Value(contextKeyServer).(*types.Server)
	return user
}

func getDeviceTokenFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyDeviceToken).(string)
	return v
}

func xAdminAPIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !adminAPIKeyValid(r) {
			senderr(w, 401, "Unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), contextKeyIsAdminAPIKey, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func wireGuardServerKeyCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server, ok := serverFromWGKey(r)
		if !ok {
			senderr(w, 401, "Unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyServer, server)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func adminUIMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAdminAPIKeyFromContext(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("admin_session")
		if err != nil {
			senderr(w, 401, "Unauthorized")
			return
		}

		uid, deviceToken, err := decryptAdminCookie(cookie.Value, clientIP(r))
		if err != nil {
			senderr(w, 401, "Unauthorized")
			return
		}

		user, err := authenticateUserFromEmailOrIDAndToken("", uid, deviceToken)
		if err != nil {
			senderr(w, 401, "Unauthorized")
			return
		}

		if !user.IsAdmin {
			senderr(w, 401, "Unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUser, user)
		ctx = context.WithValue(ctx, contextKeyDeviceToken, deviceToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func clientAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deviceToken := r.Header.Get("X-Device-Token")
		if deviceToken == "" {
			senderr(w, 401, "Unauthorized")
			return
		}

		email := r.Header.Get("X-Email")
		uidStr := r.Header.Get("X-UID")

		var parsedUID uuid.UUID
		if uidStr != "" {
			var err error
			parsedUID, err = uuid.Parse(uidStr)
			if err != nil {
				senderr(w, 401, "Unauthorized")
				return
			}
		}

		user, err := authenticateUserFromEmailOrIDAndToken(email, parsedUID, deviceToken)
		if err != nil {
			senderr(w, 401, "Unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUser, user)
		ctx = context.WithValue(ctx, contextKeyDeviceToken, deviceToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func applyMiddleware(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
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

func originAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	for _, o := range Config.Load().AllowedOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if origin := r.Header.Get("Origin"); originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-API-KEY, X-Device-Token, X-UID, X-Email")
		}

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
