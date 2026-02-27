package main

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sort"

	"github.com/tunnels-is/tunnels/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FORM_WG_REGISTER is the request body for POST /v3/wg/register.
type FORM_WG_REGISTER struct {
	DeviceToken  string             `json:"DeviceToken"`
	UID          primitive.ObjectID `json:"UID"`
	DeviceID     primitive.ObjectID `json:"DeviceID"`
	ServerID     primitive.ObjectID `json:"ServerID"`
	PublicKeyB64 string             `json:"PublicKeyB64"`
}

// FORM_WG_UNREGISTER is the request body for DELETE /v3/wg/register.
type FORM_WG_UNREGISTER struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	DeviceID    primitive.ObjectID `json:"DeviceID"`
}

// API_WGRegister handles POST /v3/wg/register.
// It assigns a WireGuard IP to the device and stores its public key.
func API_WGRegister(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodPost {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	F := new(FORM_WG_REGISTER)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	hasAPIKey := HTTP_validateKey(r)
	if !hasAPIKey {
		user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
		if err != nil {
			senderr(w, 401, err.Error())
			return
		}
		if !user.IsAdmin && !user.IsManager {
			senderr(w, 401, "Insufficient permissions to register WireGuard keys")
			return
		}
	}

	if F.DeviceID == primitive.NilObjectID {
		senderr(w, 400, "DeviceID is required")
		return
	}
	if F.PublicKeyB64 == "" {
		senderr(w, 400, "PublicKeyB64 is required")
		return
	}

	// Validate the public key: must be 32 bytes of base64.
	keyBytes, err := base64.StdEncoding.DecodeString(F.PublicKeyB64)
	if err != nil || len(keyBytes) != 32 {
		senderr(w, 400, "PublicKeyB64 must be a base64-encoded 32-byte Curve25519 public key")
		return
	}

	device, err := DB_FindDeviceByID(F.DeviceID)
	if err != nil || device == nil {
		senderr(w, 404, "Device not found")
		return
	}

	// Assign an IP from the WireGuard subnet if not already assigned.
	if device.WireGuardIP == "" {
		ip, err := assignNextWGIP()
		if err != nil {
			senderr(w, 500, "Failed to assign WireGuard IP", slog.Any("err", err))
			return
		}
		device.WireGuardIP = ip
	}
	device.WireGuardKey = F.PublicKeyB64

	if err := DB_UpdateDevice(device); err != nil {
		senderr(w, 500, "Failed to update device", slog.Any("err", err))
		return
	}

	resp := &types.WGRegisterResponse{
		AssignedIP: device.WireGuardIP,
	}

	// If a ServerID was provided, include the WireGuard server's connection details.
	if F.ServerID != primitive.NilObjectID {
		server, err := DB_FindServerByID(F.ServerID)
		if err == nil && server != nil && server.WireGuardPubKey != "" {
			resp.ServerPubKey = server.WireGuardPubKey
			resp.ServerIP = server.IP
			resp.ServerPort = server.WireGuardPort
			resp.Conf = buildWGConf(device.WireGuardIP, F.PublicKeyB64, server)
		}
	}

	sendObject(w, resp)
}

// API_WGUnregister handles DELETE /v3/wg/register.
// It clears the WireGuard key and IP from the device.
func API_WGUnregister(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodDelete {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	F := new(FORM_WG_UNREGISTER)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	hasAPIKey := HTTP_validateKey(r)
	if !hasAPIKey {
		user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
		if err != nil {
			senderr(w, 401, err.Error())
			return
		}
		if !user.IsAdmin && !user.IsManager {
			senderr(w, 401, "Insufficient permissions to unregister WireGuard keys")
			return
		}
	}

	if F.DeviceID == primitive.NilObjectID {
		senderr(w, 400, "DeviceID is required")
		return
	}

	device, err := DB_FindDeviceByID(F.DeviceID)
	if err != nil || device == nil {
		senderr(w, 404, "Device not found")
		return
	}

	device.WireGuardKey = ""
	device.WireGuardIP = ""

	if err := DB_UpdateDevice(device); err != nil {
		senderr(w, 500, "Failed to update device", slog.Any("err", err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// API_WGPeers handles GET /v3/wg/peers.
// Returns all devices with a WireGuard key registered, in hex-key format
// for direct consumption by wg-server's UAPI IPC.
// Protected by AdminAPIKey.
func API_WGPeers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodGet {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if !HTTP_validateKey(r) {
		senderr(w, 401, "Unauthorized")
		return
	}

	// Fetch all devices; filter those with a WireGuard key.
	// Use a large limit to get all; in practice device counts are modest.
	devices, err := DB_GetDevices(100000, 0)
	if err != nil {
		senderr(w, 500, "Failed to fetch devices", slog.Any("err", err))
		return
	}

	resp := types.WGPeersResponse{
		Peers: make([]types.WGPeer, 0),
	}
	for _, d := range devices {
		if d.WireGuardKey == "" || d.WireGuardIP == "" {
			continue
		}
		hexKey, err := b64KeyToHex(d.WireGuardKey)
		if err != nil {
			logger.Warn("skipping device with invalid WireGuard key",
				slog.String("device", d.ID.Hex()), slog.Any("err", err))
			continue
		}
		resp.Peers = append(resp.Peers, types.WGPeer{
			PublicKeyHex: hexKey,
			AllowedIP:    d.WireGuardIP + "/32",
			DeviceID:     d.ID.Hex(),
		})
	}

	sendObject(w, resp)
}

// API_WGConfig handles GET /v3/wg/config.
// Returns the WireGuard server's public key and connection details for a given server.
func API_WGConfig(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodGet {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	serverIDStr := r.URL.Query().Get("serverID")
	if serverIDStr == "" {
		senderr(w, 400, "serverID query parameter is required")
		return
	}
	serverID, err := primitive.ObjectIDFromHex(serverIDStr)
	if err != nil {
		senderr(w, 400, "Invalid serverID")
		return
	}

	server, err := DB_FindServerByID(serverID)
	if err != nil || server == nil {
		senderr(w, 404, "Server not found")
		return
	}

	sendObject(w, map[string]string{
		"WireGuardPubKey": server.WireGuardPubKey,
		"WireGuardPort":   server.WireGuardPort,
		"ServerIP":        server.IP,
	})
}

// assignNextWGIP finds the highest WireGuard IP currently assigned across all
// devices and returns the next sequential IP within the configured subnet.
func assignNextWGIP() (string, error) {
	cfg := Config.Load()
	if cfg.WireGuardSubnet == "" {
		return "", fmt.Errorf("WireGuardSubnet is not configured in server config")
	}

	_, ipNet, err := net.ParseCIDR(cfg.WireGuardSubnet)
	if err != nil {
		return "", fmt.Errorf("invalid WireGuardSubnet %q: %w", cfg.WireGuardSubnet, err)
	}

	devices, err := DB_GetDevices(100000, 0)
	if err != nil {
		return "", fmt.Errorf("fetch devices: %w", err)
	}

	// Collect all assigned IPs within the subnet.
	assigned := make([]net.IP, 0)
	for _, d := range devices {
		if d.WireGuardIP == "" {
			continue
		}
		ip := net.ParseIP(d.WireGuardIP).To4()
		if ip != nil && ipNet.Contains(ip) {
			assigned = append(assigned, ip)
		}
	}

	// Sort IPs and find the max.
	sort.Slice(assigned, func(i, j int) bool {
		return ipToUint32(assigned[i]) < ipToUint32(assigned[j])
	})

	// Start from the second host (.2) — .1 is reserved for the wg-server itself.
	baseIP := ipToUint32(ipNet.IP.To4()) + 2
	if len(assigned) > 0 {
		max := ipToUint32(assigned[len(assigned)-1])
		if max >= baseIP {
			baseIP = max + 1
		}
	}

	// Verify the next IP is still within the subnet.
	nextIP := uint32ToIP(baseIP)
	if !ipNet.Contains(nextIP) {
		return "", fmt.Errorf("WireGuard subnet %s is exhausted", cfg.WireGuardSubnet)
	}

	return nextIP.String(), nil
}

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func uint32ToIP(n uint32) net.IP {
	return net.IP{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
}

// b64KeyToHex converts a base64-encoded 32-byte key to a hex string for UAPI.
func b64KeyToHex(b64 string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(b) != 32 {
		return "", fmt.Errorf("expected 32 bytes, got %d", len(b))
	}
	return hex.EncodeToString(b), nil
}

// buildWGConf generates a ready-to-use WireGuard .conf file for a device.
func buildWGConf(assignedIP, clientPubKeyB64 string, server *types.Server) string {
	return fmt.Sprintf(`[Interface]
Address = %s/32
# PrivateKey = <paste your private key here>
DNS = 1.1.1.1

[Peer]
PublicKey = %s
Endpoint = %s:%s
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
`, assignedIP, server.WireGuardPubKey, server.IP, server.WireGuardPort)
}
