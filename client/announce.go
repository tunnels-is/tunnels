package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const aclControlPort = 51821

type aclAnnouncement struct {
	AllowAll bool     `json:"AllowAll,omitempty"`
	Allowed  []string `json:"Allowed"`
}

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

func NormalizeAllowedHost(s string) (string, error) {
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutPrefix(s, "*:"); ok {
		port, err := strconv.ParseUint(rest, 10, 16)
		if err != nil || port == 0 {
			return "", fmt.Errorf("allowed host has an invalid port: %s", s)
		}
		return "*:" + strconv.FormatUint(port, 10), nil
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return a.String(), nil
	}
	if ap, err := netip.ParseAddrPort(s); err == nil {
		if ap.Port() == 0 {
			return "", fmt.Errorf("allowed host has an invalid port: %s", s)
		}
		return ap.String(), nil
	}
	return "", fmt.Errorf("allowed host must be IP, IP:PORT, or *:PORT: %s", s)
}

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
