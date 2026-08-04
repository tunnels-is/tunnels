package client

import "testing"

func TestControlServer_effectivePort_hotswap443to444(t *testing.T) {
	cases := []struct {
		name string
		host string
		port string
		want string
	}{
		{"api 443 → 444", "api.tunnels.is", "443", "444"},
		{"api 444 stays", "api.tunnels.is", "444", "444"},
		{"api empty stays empty", "api.tunnels.is", "", ""},
		{"other host 443 stays", "controller.example.com", "443", "443"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &ControlServer{Host: tc.host, Port: tc.port}
			if got := c.effectivePort(); got != tc.want {
				t.Fatalf("effectivePort() = %q, want %q", got, tc.want)
			}

			if c.Port != tc.port {
				t.Fatalf("Port field mutated: got %q, want %q", c.Port, tc.port)
			}
		})
	}
}

func TestControlServer_GetURL_hotswap(t *testing.T) {
	c := &ControlServer{Host: "api.tunnels.is", Port: "443"}
	got := c.GetURL("/client/servers")
	want := "https://api.tunnels.is:444/client/servers"
	if got != want {
		t.Fatalf("GetURL() = %q, want %q", got, want)
	}
	if c.Port != "443" {
		t.Fatalf("Port field mutated: got %q", c.Port)
	}
}
