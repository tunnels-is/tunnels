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

			ip = p.WireGuardIP
			peerStore.Set(p.DeviceID, ip, pubKeyB64)
		} else {

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

func fetchDesiredPeers(cfg *Config) (*types.WGPeersResponse, error) {
	url := cfg.ControllerURL + "/wg/peers"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-WG-KEY", cfg.APIKey)

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
