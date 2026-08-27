//go:build darwin

package client

import "testing"

func TestRouteGetOutputHasBlackhole(t *testing.T) {
	normal := `   route to: default
destination: default
       mask: default
    gateway: 192.168.2.1
  interface: en1
      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING,GLOBAL>
`
	if routeGetOutputHasBlackhole(normal) {
		t.Fatal("normal default must not look like a blackhole")
	}

	blackhole := `   route to: default
destination: default
       mask: default
  interface: lo0
      flags: <UP,DONE,BLACKHOLE,STATIC,PRCLONING,GLOBAL>
`
	if !routeGetOutputHasBlackhole(blackhole) {
		t.Fatal("BLACKHOLE flag must be detected")
	}
}
