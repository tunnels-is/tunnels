package client

import "testing"

func TestAuthorizeControlServer_MultiPort(t *testing.T) {
	conf := &configV2{
		ControlServers: []*ControlServer{
			{Host: "api.tunnels.is", Port: "443", ValidateCertificate: true},
			{Host: "api.tunnels.is", Port: "444", ValidateCertificate: false},
		},
	}
	CONFIG.Store(conf)

	s := &ControlServer{Host: "api.tunnels.is", Port: "444"}
	if err := authorizeControlServer(s); err != nil {
		t.Fatalf("expected :444 to be authorized, got %v", err)
	}
	if s.Port != "444" {
		t.Fatalf("port rewritten: want 444, got %q", s.Port)
	}
	if s.ValidateCertificate != false {
		t.Fatalf("TLS setting not taken from the :444 entry")
	}

	s = &ControlServer{Host: "api.tunnels.is", Port: "443"}
	if err := authorizeControlServer(s); err != nil {
		t.Fatalf("expected :443 to be authorized, got %v", err)
	}
	if s.Port != "443" {
		t.Fatalf("port rewritten: want 443, got %q", s.Port)
	}

	s = &ControlServer{Host: "api.tunnels.is", Port: "9999"}
	if err := authorizeControlServer(s); err == nil {
		t.Fatal("expected unconfigured port to be rejected")
	}

	s = &ControlServer{Host: "api.tunnels.is", Port: ""}
	if err := authorizeControlServer(s); err != nil {
		t.Fatalf("expected empty-port fallback to be authorized, got %v", err)
	}
	if s.Port != "443" {
		t.Fatalf("empty-port fallback: want 443, got %q", s.Port)
	}

	s = &ControlServer{Host: "evil.example.com", Port: "443"}
	if err := authorizeControlServer(s); err == nil {
		t.Fatal("expected unknown host to be rejected")
	}
}
