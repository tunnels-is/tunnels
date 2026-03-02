package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
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

// assignAndAdd is called by LazyBind when a new initiator's pubkey is
// not yet in the local peer store. It fetches the authorized peer list
// from the controller, finds the matching device, uses the controller-assigned
// IP (or falls back to local peerStore allocation), caches the result, and
// calls AddPeer so the handshake can complete.
// Returns true if the peer was successfully added to wg0.
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

	authorized, err := fetchDesiredPeers(cfg)
	if err != nil {
		WARN("assignAndAdd: fetchDesiredPeers failed: ", err)
		return false
	}

	for _, p := range authorized.Peers {
		if p.PublicKeyHex != hexKey {
			continue
		}
		INFO("assignAndAdd: peer authorized by controller, deviceID=", p.DeviceID)

		var ip string
		if p.WireGuardIP != "" {
			// Use controller-assigned IP; cache it locally.
			ip = p.WireGuardIP
			peerStore.Set(p.DeviceID, ip, pubKeyB64)
		} else {
			// Fallback: assign locally (backward compatibility for devices
			// that haven't had a controller-side IP assigned yet).
			var assignErr error
			ip, assignErr = peerStore.GetOrAssign(p.DeviceID, pubKeyB64)
			if assignErr != nil {
				WARN("assignAndAdd: GetOrAssign failed: ", assignErr)
				return false
			}
		}

		INFO("assignAndAdd: ip=", ip, " calling AddPeer")
		if err := AddPeer(hexKey, ip+"/32"); err != nil {
			WARN("assignAndAdd: AddPeer failed: ", err)
			return false
		}
		INFO("assignAndAdd: peer added to wg0 → ", pubKeyB64[:12], "… ip=", ip)
		return true
	}

	INFO("assignAndAdd: pubkey not authorized by controller → ", pubKeyB64[:12], "…")
	return false
}

// fetchDesiredPeers calls GET /v3/wg/peers on the auth controller.
// The controller returns hex-encoded public keys and pre-assigned IPs.
func fetchDesiredPeers(cfg *Config) (*types.WGPeersResponse, error) {
	url := cfg.ControllerURL + "/v3/wg/peers"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-API-KEY", cfg.AdminAPIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("controller returned %d", resp.StatusCode)
	}

	var result types.WGPeersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}
