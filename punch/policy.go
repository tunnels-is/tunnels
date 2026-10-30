package punch

import (
	"net/netip"
	"strconv"
	"strings"
)

// AdmitsAllPorts ...
func AdmitsAllPorts(firewallOff, allowAll bool, entries []string, peer netip.Addr) bool {
	if !peer.IsValid() {
		return false
	}
	if firewallOff || allowAll {
		return true
	}
	for _, raw := range entries {
		s := strings.TrimSpace(raw)
		if s == "" || strings.HasPrefix(s, "*:") {
			continue
		}
		if _, err := strconv.Atoi(s); err == nil {
			continue
		}
		if ap, err := netip.ParseAddrPort(s); err == nil {
			if ap.Addr() == peer && ap.Port() != 0 {
				continue
			}
		}
		if a, err := netip.ParseAddr(s); err == nil && a == peer {
			return true
		}
	}
	return false
}

// PairAdmitted ...
func PairAdmitted(aFirewallOff, aAllowAll bool, aEntries []string, aIP netip.Addr, bFirewallOff, bAllowAll bool, bEntries []string, bIP netip.Addr) bool {
	return AdmitsAllPorts(aFirewallOff, aAllowAll, aEntries, bIP) &&
		AdmitsAllPorts(bFirewallOff, bAllowAll, bEntries, aIP)
}
