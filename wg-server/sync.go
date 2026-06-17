package wgserver

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/tunnels-is/tunnels/types"
)

var httpClient *http.Client

func initSyncClient(cfg *Config) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		},
	}
	httpClient = &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
}

func assignAndAdd(pubKeyB64 string) bool {
	cfg := activeConfig.Load()
	if cfg == nil {
		return false
	}

	hexKey, err := b64ToHex(pubKeyB64)
	if err != nil {
		WARN("assignAndAdd: invalid pubkey: ", err)
		return false
	}

	p, err := fetchPeerByPubKey(cfg, pubKeyB64)
	if err != nil {
		WARN("assignAndAdd: fetchPeerByPubKey failed: ", err)
		return false
	}
	if p == nil {
		INFO("assignAndAdd: pubkey not authorized by controller → ", pubKeyB64[:12], "…")
		return false
	}

	INFO("assignAndAdd: peer authorized by controller, deviceID=", p.DeviceID)

	var ip, ipv6 string
	if p.WireGuardIP != "" {
		ip = p.WireGuardIP
		ipv6 = p.WireGuardIPv6
		if !subnetContains(cfg.WireGuardSubnet, ip) {
			WARN("assignAndAdd: controller-assigned IP outside subnet: ", ip)
			return false
		}
		if ipv6 != "" && cfg.WireGuardSubnet6 != "" && !subnetContains(cfg.WireGuardSubnet6, ipv6) {
			WARN("assignAndAdd: controller-assigned IPv6 outside subnet: ", ipv6)
			return false
		}
		peerStore.Set(p.DeviceID, ip, ipv6, pubKeyB64)
	} else {
		var assignErr error
		ip, ipv6, assignErr = peerStore.GetOrAssign(p.DeviceID, pubKeyB64)
		if assignErr != nil {
			WARN("assignAndAdd: GetOrAssign failed: ", assignErr)
			return false
		}
	}

	allowedIPs := peerAllowedIPs(ip, ipv6)
	INFO("assignAndAdd: ip=", ip, " ipv6=", ipv6, " calling AddPeer")
	if err := AddPeer(hexKey, allowedIPs...); err != nil {
		WARN("assignAndAdd: AddPeer failed: ", err)
		return false
	}
	addedPeerKeys.Store(hexKey, struct{}{})
	resetPeer(ip, ipv6)
	INFO("assignAndAdd: peer added to wg0 → ", pubKeyB64[:12], "… ip=", ip)
	return true
}

// peerAllowedIPs builds the list of allowed IPs for a peer.
// Always includes the IPv4 /32; adds the IPv6 /128 if non-empty.
func peerAllowedIPs(ip, ipv6 string) []string {
	ips := []string{ip + "/32"}
	if ipv6 != "" {
		ips = append(ips, ipv6+"/128")
	}
	return ips
}

func subnetContains(cidr, ipStr string) bool {
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return subnet.Contains(ip)
}

// fetchPeerByPubKey asks the controller for the single peer record matching
// pubKeyB64. Returns (nil, nil) when the controller reports 404 (not
// authorized). Any other non-2xx or transport failure returns an error.
func fetchPeerByPubKey(cfg *Config, pubKeyB64 string) (*types.WGPeer, error) {
	endpoint := cfg.ControllerURL + "/wg/peer?pubkey=" + url.QueryEscape(pubKeyB64)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-WG-KEY", cfg.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var peer types.WGPeer
		if err := json.NewDecoder(resp.Body).Decode(&peer); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		return &peer, nil
	case http.StatusNotFound:
		return nil, nil
	default:
		return nil, fmt.Errorf("controller returned %d", resp.StatusCode)
	}
}
