package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/tunnels-is/tunnels/types"
)

var (
	activeConfig atomic.Pointer[Config]
	httpClient   *http.Client
)

func initSyncClient(cfg *Config) {
	activeConfig.Store(cfg)
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

// SyncPeers is the recurring goroutine method called by signal.NewSignal.
// It fetches the desired peer list from the controller and diffs it against
// the peers currently on the WireGuard device, adding and removing only what
// changed. Existing peers are never torn down, so active sessions are preserved.
func SyncPeers() {
	cfg := activeConfig.Load()
	if cfg == nil {
		ERR("sync: config not initialized")
		return
	}

	desired, err := fetchDesiredPeers(cfg)
	if err != nil {
		ERR("sync: failed to fetch peers: ", err)
		return
	}

	// Build desired map: hex pubkey → AllowedIP
	desiredMap := make(map[string]string, len(desired.Peers))
	for _, p := range desired.Peers {
		desiredMap[p.PublicKeyHex] = p.AllowedIP
	}

	// Get the set of keys currently on the device
	currentKeys, err := GetCurrentPeerKeys()
	if err != nil {
		ERR("sync: GetCurrentPeerKeys failed: ", err)
		return
	}

	added, removed := 0, 0

	// Add peers that are in desired but not yet on the device
	for key, allowedIP := range desiredMap {
		if _, exists := currentKeys[key]; !exists {
			if err := AddPeer(key, allowedIP); err != nil {
				ERR("sync: AddPeer failed: ", err)
			} else {
				added++
			}
		}
	}

	// Remove peers that are on the device but no longer desired
	for key := range currentKeys {
		if _, exists := desiredMap[key]; !exists {
			if err := RemovePeer(key); err != nil {
				ERR("sync: RemovePeer failed: ", err)
			} else {
				removed++
			}
		}
	}

	INFO(fmt.Sprintf("sync: %d peers active, +%d added, -%d removed", len(desiredMap), added, removed))
}

// fetchDesiredPeers calls GET /v3/wg/peers on the auth controller.
// The controller returns hex-encoded public keys ready for UAPI use.
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
