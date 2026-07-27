package client

import (
	"strings"
	"testing"
)

// Kill switch must be rejected unless the tunnel also has the default route
// enabled — a kill switch without a default route protects nothing.
func TestValidateTunnelMeta_KillSwitchRequiresDefaultRoute(t *testing.T) {
	hasKSErr := func(errs []string) bool {
		for _, e := range errs {
			if strings.Contains(e, "kill switch requires the default route") {
				return true
			}
		}
		return false
	}

	bad := &TunnelMETA{IFName: "tunt", Tag: "t1", KillSwitch: true, EnableDefaultRoute: false}
	if !hasKSErr(validateTunnelMeta(bad, "")) {
		t.Fatal("kill switch without default route should be rejected")
	}

	good := &TunnelMETA{IFName: "tunt", Tag: "t1", KillSwitch: true, EnableDefaultRoute: true}
	if hasKSErr(validateTunnelMeta(good, "")) {
		t.Fatal("kill switch with default route should be allowed")
	}

	off := &TunnelMETA{IFName: "tunt", Tag: "t1", KillSwitch: false, EnableDefaultRoute: false}
	if hasKSErr(validateTunnelMeta(off, "")) {
		t.Fatal("kill switch off should not trigger the rule")
	}
}
