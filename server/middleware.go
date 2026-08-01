package main

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

type contextKey string

const (
	contextKeyUser          contextKey = "user"
	contextKeyIsAdminAPIKey contextKey = "isAdminAPIKey"
	contextKeyServer        contextKey = "server"
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

func xAdminAPIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !HTTP_validateKey(r) {
			senderr(w, 401, "Unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), contextKeyIsAdminAPIKey, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func wireGuardServerKeyCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server, ok := HTTP_validateWGKey(r)
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
			senderr(w, 401, err.Error())
			return
		}

		user, err := authenticateUserFromEmailOrIDAndToken("", uid, deviceToken)
		if err != nil {
			senderr(w, 401, err.Error())
			return
		}

		if !user.IsAdmin {
			senderr(w, 401, "Admin access required")
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func clientAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deviceToken := r.Header.Get("X-Device-Token")
		if deviceToken == "" {
			senderr(w, 401, "Unauthorized - no device token")
			return
		}

		email := r.Header.Get("X-Email")
		uidStr := r.Header.Get("X-UID")

		var parsedUID uuid.UUID
		if uidStr != "" {
			var err error
			parsedUID, err = uuid.Parse(uidStr)
			if err != nil {
				senderr(w, 401, "Invalid X-UID header")
				return
			}
		}

		user, err := authenticateUserFromEmailOrIDAndToken(email, parsedUID, deviceToken)
		if err != nil {
			senderr(w, 401, err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func applyMiddleware(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
