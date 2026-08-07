package wgserver

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/tunnels-is/tunnels/types"
)

var httpClient *http.Client

func initSyncClient(cfg *Config) {
	insecure := false
	if cfg != nil {
		insecure = cfg.InsecureSkipVerify
	}
	httpClient = newControllerHTTPClient(insecure)
}

type authResult int

const (
	authAllowed authResult = iota
	authDenied
	authUnknown
)

func reconcilePeer(pubKeyB64 string) authResult {
	cfg := activeConfig.Load()
	if cfg == nil {
		return authUnknown
	}
	hexKey, err := b64ToHex(pubKeyB64)
	if err != nil {
		WARN("reconcilePeer: invalid pubkey: ", err)
		return authDenied
	}

	res, p := queryPeer(cfg, pubKeyB64)
	if res != authAllowed {
		return res
	}

	var ip, ipv6 string
	if p.WireGuardIP != "" {
		ip = p.WireGuardIP
		ipv6 = p.WireGuardIPv6
		if !subnetContains(cfg.WireGuardSubnet, ip) {
			WARN("reconcilePeer: controller-assigned IP outside subnet: ", ip)
			return authDenied
		}
		if ipv6 != "" && cfg.WireGuardSubnet6 != "" && !subnetContains(cfg.WireGuardSubnet6, ipv6) {
			WARN("reconcilePeer: controller-assigned IPv6 outside subnet: ", ipv6)
			return authDenied
		}
		peerStore.Set(p.DeviceID, ip, ipv6, pubKeyB64)
	} else {
		var assignErr error
		ip, ipv6, assignErr = peerStore.GetOrAssign(p.DeviceID, pubKeyB64)
		if assignErr != nil {
			WARN("reconcilePeer: GetOrAssign failed: ", assignErr)
			return authUnknown
		}
	}

	if _, rekey := addedPeerKeys.LoadOrStore(hexKey, struct{}{}); !rekey {
		resetPeer(ip, ipv6)
	}
	if err := AddPeer(hexKey, peerAllowedIPs(ip, ipv6)...); err != nil {
		WARN("reconcilePeer: AddPeer failed: ", err)
		return authUnknown
	}
	INFO("reconcilePeer: authorized → ", pubKeyB64[:12], "… ip=", ip)
	return authAllowed
}

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

func queryPeer(cfg *Config, pubKeyB64 string) (authResult, *types.WGPeer) {
	if err := requireHTTPSControllerURL(cfg.ControllerURL); err != nil {
		WARN("queryPeer: ", err)
		return authUnknown, nil
	}
	endpoint := strings.TrimRight(cfg.ControllerURL, "/") + "/wg/peer?pubkey=" + url.QueryEscape(pubKeyB64)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		WARN("queryPeer: build request: ", err)
		return authUnknown, nil
	}
	req.Header.Set("X-WG-KEY", cfg.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		WARN("queryPeer: request failed: ", err)
		return authUnknown, nil
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var peer types.WGPeer
		if err := json.NewDecoder(resp.Body).Decode(&peer); err != nil {
			WARN("queryPeer: decode response: ", err)
			return authUnknown, nil
		}
		return authAllowed, &peer
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return authDenied, nil
	default:
		WARN("queryPeer: controller returned ", resp.StatusCode)
		return authUnknown, nil
	}
}
