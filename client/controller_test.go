package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendRequestToURL_RefusesRedirectWithDeviceToken(t *testing.T) {
	stolen := make(chan string, 1)
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stolen <- r.Header.Get("X-Device-Token")
		w.WriteHeader(200)
	}))
	t.Cleanup(evil.Close)

	ctrl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/steal", http.StatusFound)
	}))
	t.Cleanup(ctrl.Close)

	_, _, err := SendRequestToURL(nil, "GET", ctrl.URL, nil, 5000, false, "", map[string]string{
		"X-Device-Token": "session-secret",
		"X-UID":          "user-1",
	})
	if err == nil {
		t.Fatal("redirect must fail the request")
	}
	if !errors.Is(err, errControllerRedirect) {
		t.Fatalf("err = %v, want errControllerRedirect", err)
	}
	select {
	case tok := <-stolen:
		t.Fatalf("evil host received X-Device-Token %q", tok)
	default:
	}
}

func TestSendRequestToURL_RejectsOversizedBody(t *testing.T) {
	prev := maxControllerResponseBytes
	maxControllerResponseBytes = 64
	t.Cleanup(func() { maxControllerResponseBytes = prev })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write(make([]byte, 65))
	}))
	t.Cleanup(srv.Close)

	_, _, err := SendRequestToURL(nil, "GET", srv.URL, nil, 5000, false, "")
	if !errors.Is(err, errControllerResponseTooLarge) {
		t.Fatalf("err = %v, want errControllerResponseTooLarge", err)
	}
}

func TestSendRequestToURL_AcceptsBodyAtLimit(t *testing.T) {
	prev := maxControllerResponseBytes
	maxControllerResponseBytes = 64
	t.Cleanup(func() { maxControllerResponseBytes = prev })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write(make([]byte, 64))
	}))
	t.Cleanup(srv.Close)

	body, code, err := SendRequestToURL(nil, "GET", srv.URL, nil, 5000, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if code != 200 || len(body) != 64 {
		t.Fatalf("code=%d len=%d", code, len(body))
	}
}
