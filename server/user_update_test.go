package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAPI_UserUpdate_APIKeyValidation(t *testing.T) {
	setupTestDB(t)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	u := &User{ID: uuid.New(), Email: "u@example.com", Groups: []uuid.UUID{}, Tokens: []*DeviceToken{}}
	if err := BBolt_CreateUser(u); err != nil {
		t.Fatal(err)
	}

	call := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/client/user/update", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, u))
		w := httptest.NewRecorder()
		API_UserUpdate(w, req)
		return w
	}

	if w := call(`{"APIKey":"not-a-uuid"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("non-UUID APIKey should be rejected with 400, got %d", w.Code)
	}
	if w := call(`{"APIKey":"` + uuid.NewString() + `"}`); w.Code != http.StatusOK {
		t.Fatalf("valid UUID APIKey should be accepted (200), got %d: %s", w.Code, w.Body.String())
	}
	if w := call(`{"APIKey":""}`); w.Code != http.StatusOK {
		t.Fatalf("empty APIKey should be allowed (clears the key), got %d", w.Code)
	}
}
