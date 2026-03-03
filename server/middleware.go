package main

import (
	"context"
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type contextKey string

const (
	contextKeyUser          contextKey = "user"
	contextKeyIsAdminAPIKey contextKey = "isAdminAPIKey"
)

func getUserFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(contextKeyUser).(*User)
	return user
}

func isAdminAPIKeyFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(contextKeyIsAdminAPIKey).(bool)
	return v
}

func xAPIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if HTTP_validateKey(r) {
			ctx := context.WithValue(r.Context(), contextKeyIsAdminAPIKey, true)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		next.ServeHTTP(w, r)
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

		parts := strings.SplitN(cookie.Value, ":", 2)
		if len(parts) != 2 {
			senderr(w, 401, "Invalid session")
			return
		}

		uid, err := primitive.ObjectIDFromHex(parts[0])
		if err != nil {
			senderr(w, 401, "Invalid session")
			return
		}

		user, err := authenticateUserFromEmailOrIDAndToken("", uid, parts[1])
		if err != nil {
			senderr(w, 401, err.Error())
			return
		}

		if !user.IsAdmin && !user.IsManager {
			senderr(w, 401, "Admin or Manager access required")
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func clientAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAdminAPIKeyFromContext(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}

		deviceToken := r.Header.Get("X-Device-Token")
		if deviceToken == "" {
			senderr(w, 401, "Unauthorized")
			return
		}

		email := r.Header.Get("X-Email")
		uidStr := r.Header.Get("X-UID")

		var parsedUID primitive.ObjectID
		if uidStr != "" {
			var err error
			parsedUID, err = primitive.ObjectIDFromHex(uidStr)
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
