package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

// aclControlPort is the UDP port on the wg-server's WG-side IP that receives
// firewall allowlist announcements. The server consumes these packets in its
// packet inspector; they are fire-and-forget (no response).
const aclControlPort = 51821

type aclAnnouncement struct {
	// AllowAll, when true, tells the server any peer may reach this device,
	// overriding Allowed. Omitted from the wire when false for compatibility.
	AllowAll bool     `json:"AllowAll,omitempty"`
	Allowed  []string `json:"Allowed"`
}

// wgServerGatewayIP returns the wg-server's own WG-side IP: the first host
// address of the WireGuard subnet (mirrors the wg-server's address setup).
func wgServerGatewayIP(subnet string) (net.IP, error) {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid WireGuard subnet %q: %w", subnet, err)
	}
	base := ipNet.IP.To4()
	if base == nil {
		return nil, fmt.Errorf("WireGuard subnet %q is not IPv4", subnet)
	}
	gw := make(net.IP, 4)
	copy(gw, base)
	gw[3]++
	return gw, nil
}

// AnnounceAllowedHosts sends the firewall policy to the wg-server's ACL
// control port through the tunnel: the allowlist plus the allow-all flag.
// An empty list with allowAll=false clears this device's policy on the server
// (used on disconnect). Fire-and-forget: a lost packet is only recovered by a
// later announce.
func (t *TUN) AnnounceAllowedHosts(allowed []string, allowAll bool) error {
	sr := t.ServerResponse
	if sr == nil || sr.WireGuardSubnet == "" {
		return errors.New("no WireGuard subnet known for tunnel")
	}
	if t.localInterfaceNetIP == nil {
		return errors.New("tunnel has no local interface IP")
	}
	gw, err := wgServerGatewayIP(sr.WireGuardSubnet)
	if err != nil {
		return err
	}

	if allowed == nil {
		allowed = []string{}
	}
	payload, err := json.Marshal(&aclAnnouncement{AllowAll: allowAll, Allowed: allowed})
	if err != nil {
		return fmt.Errorf("marshal announcement: %w", err)
	}

	conn, err := net.DialUDP("udp4",
		&net.UDPAddr{IP: t.localInterfaceNetIP},
		&net.UDPAddr{IP: gw, Port: aclControlPort},
	)
	if err != nil {
		return fmt.Errorf("dial ACL control port: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("send announcement: %w", err)
	}
	DEBUG("announced ", len(allowed), " allowed host(s) to ", gw.String(), ":", aclControlPort)
	return nil
}

// announceAllowedHostsWithRetry announces the tunnel meta's AllowedHosts a
// few times with delays. Used right after connect, when the WireGuard
// handshake may not have completed yet and the first packet can be lost.
//
// An empty list is announced too — announcements are replace-set, so the
// empty announce is what overwrites a stale policy left on the server by a
// previous session (e.g. when the disconnect-time clear was lost).
func (t *TUN) announceAllowedHostsWithRetry() {
	defer RecoverAndLog()
	for _, delay := range []time.Duration{0, 2 * time.Second, 5 * time.Second} {
		time.Sleep(delay)
		if t.GetState() < TUN_Connected {
			return
		}
		m := t.meta.Load()
		if m == nil {
			return
		}
		if err := t.AnnounceAllowedHosts(m.AllowedHosts, m.AllowAll); err != nil {
			DEBUG("allowed hosts announce failed: ", err)
		}
	}
}
