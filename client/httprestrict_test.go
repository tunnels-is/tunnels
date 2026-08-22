package client

import (
	"net"
	"net/http"
	"net/url"
	"testing"
)

func TestRequirePublicHTTPSURL(t *testing.T) {
	if err := requirePublicHTTPSURL(&url.URL{Scheme: "http", Host: "example.com"}); err == nil {
		t.Fatal("http must be refused")
	}
	if err := requirePublicHTTPSURL(&url.URL{Scheme: "https", Host: "127.0.0.1"}); err == nil {
		t.Fatal("loopback must be refused")
	}
	if err := requirePublicHTTPSURL(&url.URL{Scheme: "https", Host: "10.0.0.1"}); err == nil {
		t.Fatal("RFC1918 must be refused")
	}
	if err := requirePublicHTTPSURL(&url.URL{Scheme: "https", Host: "169.254.169.254"}); err == nil {
		t.Fatal("IMDS must be refused")
	}
	if err := requirePublicHTTPSURL(&url.URL{Scheme: "https", Host: "localhost"}); err == nil {
		t.Fatal("localhost must be refused")
	}
	if err := requirePublicHTTPSURL(&url.URL{Scheme: "https", Host: "192.0.2.1"}); err != nil {
		t.Fatalf("TEST-NET public IP must be allowed: %v", err)
	}
}

func TestCheckPublicHTTPSRedirect_LimitsHops(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://192.0.2.1/next", nil)
	if err != nil {
		t.Fatal(err)
	}
	via := make([]*http.Request, 5)
	if err := checkPublicHTTPSRedirect(req, via); err == nil {
		t.Fatal("too many redirects must be refused")
	}
}

func TestRequirePublicHTTPSURL_HostnameResolvesPrivate(t *testing.T) {
	prev := lookupIPFunc
	t.Cleanup(func() { lookupIPFunc = prev })
	lookupIPFunc = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	if err := requirePublicHTTPSURL(&url.URL{Scheme: "https", Host: "evil.example"}); err == nil {
		t.Fatal("hostname that resolves to loopback must be refused")
	}
}

func TestIsPublicIP_CGNAT(t *testing.T) {
	if isPublicIP(net.ParseIP("100.64.0.1")) {
		t.Fatal("CGNAT must not be public")
	}
	if !isPublicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("8.8.8.8 must be public")
	}
}

func TestOfficialControllerHost(t *testing.T) {
	if !isOfficialControllerHost("api.tunnels.is") {
		t.Fatal("exact host must match")
	}
	if isOfficialControllerHost("evil-api.tunnels.is.example") {
		t.Fatal("substring must not match")
	}
	if isOfficialControllerHost("notapi.tunnels.is") {
		t.Fatal("suffix must not match")
	}
}

func TestValidateListURL_RefusesPrivateInitialHop(t *testing.T) {
	if err := validateListURL("https://127.0.0.1/list"); err == nil {
		t.Fatal("loopback list URL must be refused")
	}
	if err := validateListURL("https://169.254.169.254/latest"); err == nil {
		t.Fatal("IMDS list URL must be refused")
	}
	if err := validateListURL("http://example.com/list"); err == nil {
		t.Fatal("http list URL must be refused")
	}
	if err := validateListURL("https://192.0.2.1/list"); err != nil {
		t.Fatalf("public TEST-NET URL must be allowed: %v", err)
	}
}

func TestOfficialControllerHost_CaseInsensitive(t *testing.T) {
	if !isOfficialControllerHost("API.TUNNELS.IS") {
		t.Fatal("exact host match should be case-insensitive")
	}
}
