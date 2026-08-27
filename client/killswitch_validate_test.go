package client

import (
	"strings"
	"testing"
)

func TestDefaultConfig_KillSwitchDefaults(t *testing.T) {
	conf := DefaultConfig()
	if conf.KillSwitchIPv4 {
		t.Fatal("IPv4 kill switch should be off by default")
	}
	if conf.KillSwitchIPv6 {
		t.Fatal("IPv6 kill switch should be off by default")
	}
}

func TestApplyMissingKillSwitchDefaults_IPv6OnWhenAbsent(t *testing.T) {
	cfg := &configV2{}
	applyMissingKillSwitchDefaults([]byte(`{"APIIP":"127.0.0.1"}`), cfg)
	if !cfg.KillSwitchIPv6 {
		t.Fatal("missing KillSwitchIPv6 must default to on")
	}
	if cfg.KillSwitchIPv4 {
		t.Fatal("missing KillSwitchIPv4 must stay off")
	}

	off := &configV2{}
	applyMissingKillSwitchDefaults([]byte(`{"KillSwitchIPv6":false}`), off)
	if off.KillSwitchIPv6 {
		t.Fatal("explicit KillSwitchIPv6 false must be preserved")
	}
}

func TestValidateTunnelMeta_NoPerTunnelKillSwitchRule(t *testing.T) {
	tun := &TunnelMETA{IFName: "tunt", Tag: "t1", EnableDefaultRoute: false}
	for _, e := range validateTunnelMeta(tun, "") {
		if strings.Contains(e, "kill switch") {
			t.Fatalf("per-tunnel kill switch rule should be gone: %s", e)
		}
	}
}

func TestKillSwitchSupportedOnThisPlatform(t *testing.T) {
	if !killSwitchSupported() {
		t.Skip("no blackhole implementation on this OS")
	}
}
