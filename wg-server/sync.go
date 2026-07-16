package wgserver

import (
	"crypto/tls"
	"encoding/json"
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

// authResult classifies the controller's answer for a peer at (re)connect time.
type authResult int

const (
	authAllowed authResult = iota // authorized; peer is (re)installed
	authDenied                    // controller definitively rejected it; caller removes the peer
	authUnknown                   // transient failure (unreachable / 5xx); caller must NOT change peer state
)

// reconcilePeer re-checks the controller's authorization for pubKeyB64 and
// installs the peer when allowed. It runs on every (re)connect handshake, so a
// revocation on the controller (user disabled, subscription expired, device
// deleted, group removed) takes effect on the peer's next handshake without a
// wg-server restart. A transient controller error returns authUnknown so a blip
// does not tear down live tunnels.
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

	// Install the firewall slot BEFORE adding the peer to the wg device, so the
	// slot exists the instant the peer can pass traffic. Reset only on a fresh
	// add — a rekey/reconnect of an already-installed peer must not wipe the
	// allowlist it announced after connecting.
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

// queryPeer asks the controller whether pubKeyB64 is authorized on this server,
// mapping the HTTP status to an authResult:
//   - 200            → authAllowed (with the decoded peer);
//   - 401/403/404    → authDenied (not allowed / disabled / expired / unknown device);
//   - anything else  → authUnknown (5xx or transport error — treated as transient).
func queryPeer(cfg *Config, pubKeyB64 string) (authResult, *types.WGPeer) {
	endpoint := cfg.ControllerURL + "/wg/peer?pubkey=" + url.QueryEscape(pubKeyB64)
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
