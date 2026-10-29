package client

import "testing"

func TestDemotePhysicalDefault(t *testing.T) {
	cases := []struct {
		name           string
		def, blackhole bool
		tunnelLink     bool
		tunnelGw       bool
		protect        bool
		priority       int
		want           bool
	}{
		{name: "physical default", def: true, priority: 0, want: true},
		{name: "already above sink", def: true, priority: 100, want: false},
		{name: "sink metric", def: true, priority: killSwitchMetric, want: false},
		{name: "blackhole", def: true, blackhole: true, priority: 0, want: false},
		{name: "tunnel link", def: true, tunnelLink: true, priority: 0, want: false},
		{name: "tunnel gateway", def: true, tunnelGw: true, priority: 0, want: false},
		{name: "protect table", def: true, protect: true, priority: 0, want: false},
		{name: "not a default", priority: 0, want: false},
	}
	for _, tc := range cases {
		got := demotePhysicalDefault(tc.def, tc.blackhole, tc.tunnelLink, tc.tunnelGw, tc.protect, tc.priority)
		if got != tc.want {
			t.Fatalf("%s: got %v", tc.name, got)
		}
	}
}

func TestClassifyDefaultRoute(t *testing.T) {
	tunnels := map[string]struct{}{"utun4": {}}
	gateways := map[string]struct{}{"10.0.0.1": {}}

	physical := `   route to: default
destination: default
    gateway: 192.168.2.1
  interface: en1
      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING,GLOBAL>
`
	if classifyDefaultRoute(physical, tunnels, gateways) != defaultChoicePhysical {
		t.Fatal("physical default")
	}

	blackhole := `   route to: default
destination: default
  interface: lo0
      flags: <UP,DONE,STATIC,BLACKHOLE>
`
	if classifyDefaultRoute(blackhole, tunnels, gateways) != defaultChoiceBlackhole {
		t.Fatal("blackhole")
	}

	tunnel := `   route to: default
destination: default
    gateway: 10.0.0.1
  interface: utun4
      flags: <UP,GATEWAY,DONE,STATIC>
`
	if classifyDefaultRoute(tunnel, tunnels, gateways) != defaultChoiceTunnel {
		t.Fatal("tunnel interface")
	}

	viaGW := `   route to: default
destination: default
    gateway: 10.0.0.1
  interface: en1
      flags: <UP,GATEWAY,DONE,STATIC>
`
	if classifyDefaultRoute(viaGW, tunnels, gateways) != defaultChoiceTunnel {
		t.Fatal("tunnel gateway")
	}

	if classifyDefaultRoute("flags: <UP>\n", tunnels, gateways) != defaultChoiceNone {
		t.Fatal("empty route")
	}
}

func TestNetstatHasBlackholeDefault(t *testing.T) {
	table := "default            192.168.1.1        UGScg           en0\n" +
		"default                                 UGScB           lo0\n" +
		"10.0.0.0/8         10.0.0.1           UGSc            utun4\n"
	if !netstatHasBlackholeDefault(table) {
		t.Fatal("expected blackhole default")
	}
	plain := "default            192.168.1.1        UGScg           en0\n"
	if netstatHasBlackholeDefault(plain) {
		t.Fatal("gateway default is not a blackhole")
	}
}

func TestRaisePhysicalDefault(t *testing.T) {
	if !raisePhysicalDefault(0, 25, false, false) {
		t.Fatal("physical default should be raised")
	}
	if raisePhysicalDefault(0, killSwitchMetric, false, false) {
		t.Fatal("metric at the sink should stay")
	}
	if raisePhysicalDefault(0, 1, true, false) {
		t.Fatal("loopback sink should stay")
	}
	if raisePhysicalDefault(0, 1, false, true) {
		t.Fatal("tunnel default should stay")
	}
	if raisePhysicalDefault(24, 0, false, false) {
		t.Fatal("narrower route should stay")
	}
}
