package ui

import (
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func TestServerWGAddr(t *testing.T) {
	cases := []struct {
		name string
		s    *types.Server
		want string
	}{
		{name: "nil", want: "—"},
		{name: "empty ip", s: &types.Server{WireGuardPort: 51820}, want: "—"},
		{name: "ip only", s: &types.Server{IP: "1.2.3.4"}, want: "1.2.3.4"},
		{name: "ip and wg port", s: &types.Server{IP: "1.2.3.4", Port: "443", WireGuardPort: 51820}, want: "1.2.3.4:51820"},
		{name: "ipv6", s: &types.Server{IP: "2001:db8::1", WireGuardPort: 51820}, want: "[2001:db8::1]:51820"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := serverWGAddr(c.s); got != c.want {
				t.Fatalf("serverWGAddr() = %q, want %q", got, c.want)
			}
		})
	}
}
