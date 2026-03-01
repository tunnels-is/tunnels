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

	// Sync server-to-server peers for cross-server LAN routing.
	SyncCrossServerPeers(cfg)
}

// assignAndAdd is called when LazyBind has decrypted an initiator's pubkey but
// it has no IP in the local peer store yet. It fetches the authorized peer list
// from the controller, finds the matching device, assigns an IP via the store,
// and calls AddPeer — all without a full sync.
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
		ip, err := peerStore.GetOrAssign(p.DeviceID, pubKeyB64)
		if err != nil {
			WARN("assignAndAdd: GetOrAssign failed: ", err)
			return false
		}
		INFO("assignAndAdd: assigned ip=", ip, " calling AddPeer")
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

// fetchPeerServers calls GET /v3/wg/servers to discover other wg-servers for
// cross-server routing. Returns an empty slice (not an error) when ServerID
// is not configured, since cross-server peering is optional.
func fetchPeerServers(cfg *Config) ([]peerServer, error) {
	url := cfg.ControllerURL + "/v3/wg/servers"
	if cfg.ServerID != "" {
		url += "?excludeID=" + cfg.ServerID
	}
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

	var result struct {
		Servers []peerServer `json:"Servers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Servers, nil
}

// peerServer holds the WireGuard connection details for a remote wg-server.
type peerServer struct {
	WireGuardPubKey string `json:"WireGuardPubKey"`
	WireGuardPort   string `json:"WireGuardPort"`
	WireGuardSubnet string `json:"WireGuardSubnet"`
	IP              string `json:"IP"`
}

// SyncCrossServerPeers fetches the list of peer wg-servers from the controller
// and ensures each one is configured as a WireGuard peer on wg0 with an
// endpoint (for server-initiated keepalive) and AllowedIPs = their subnet.
// It also installs a MASQUERADE exclusion rule so traffic between server
// subnets is never NAT'd.
func SyncCrossServerPeers(cfg *Config) {
	peers, err := fetchPeerServers(cfg)
	if err != nil {
		WARN("cross-server sync: fetchPeerServers failed: ", err)
		return
	}
	if len(peers) == 0 {
		return
	}

	for _, p := range peers {
		if p.WireGuardPubKey == "" || p.WireGuardSubnet == "" || p.IP == "" || p.WireGuardPort == "" {
			continue
		}
		hexKey, err := b64ToHex(p.WireGuardPubKey)
		if err != nil {
			WARN("cross-server sync: invalid pubkey: ", err)
			continue
		}
		endpoint := p.IP + ":" + p.WireGuardPort
		if err := AddPeerWithEndpoint(hexKey, p.WireGuardSubnet, endpoint); err != nil {
			WARN("cross-server sync: AddPeerWithEndpoint failed for ", endpoint, ": ", err)
			continue
		}
		INFO("cross-server sync: peered with ", endpoint, " subnet=", p.WireGuardSubnet)

		// Ensure MASQUERADE doesn't apply to cross-server traffic (defense in depth).
		if err := addCrossServerMasqueradeExclusion(p.WireGuardSubnet, cfg.InternetIface); err != nil {
			WARN("cross-server sync: masquerade exclusion failed for ", p.WireGuardSubnet, ": ", err)
		}
	}
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
