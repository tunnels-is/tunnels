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
// It reconciles the wg0 peer list against the intersection of:
//   - authorized devices (fetched from the controller's /v3/wg/peers)
//   - devices with an assigned IP in the local peer store
//
// This means a device appears on wg0 only after it has successfully
// authenticated with the controller AND connected to this wg-server at least
// once (triggering IP assignment via /v3/wg/assign).
func SyncPeers() {
	cfg := activeConfig.Load()
	if cfg == nil {
		ERR("sync: config not initialized")
		return
	}

	// Fetch the set of authorized devices from the controller.
	// The controller returns DeviceID + hex public key; IPs are not included
	// because each wg-server manages its own address space.
	authorized, err := fetchDesiredPeers(cfg)
	if err != nil {
		ERR("sync: failed to fetch authorized peers: ", err)
		return
	}

	// Build desired map: hex pubkey → AllowedIP (/32)
	// Only include devices that have an IP in our local store.
	desiredMap := make(map[string]string, len(authorized.Peers))
	for _, p := range authorized.Peers {
		rec, ok := peerStore.Get(p.DeviceID)
		if !ok {
			// Device is authorized but hasn't connected to this wg-server yet.
			continue
		}
		desiredMap[p.PublicKeyHex] = rec.IP + "/32"
	}

	// Get the set of keys currently on the device.
	currentKeys, err := GetCurrentPeerKeys()
	if err != nil {
		ERR("sync: GetCurrentPeerKeys failed: ", err)
		return
	}

	added, removed := 0, 0

	// Add peers that are in desired but not yet on the device.
	for key, allowedIP := range desiredMap {
		if _, exists := currentKeys[key]; !exists {
			if err := AddPeer(key, allowedIP); err != nil {
				ERR("sync: AddPeer failed: ", err)
			} else {
				added++
			}
		}
	}

	// Remove peers that are on the device but no longer authorized.
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
