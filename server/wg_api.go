package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

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
// Stores the client's WireGuard public key against the device record.
// IP assignment is delegated to the wg-server via callWGAssign.
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

	device.WireGuardKey = F.PublicKeyB64
	if err := DB_UpdateDevice(device); err != nil {
		senderr(w, 500, "Failed to update device", slog.Any("err", err))
		return
	}

	resp := &types.WGRegisterResponse{}

	// If a ServerID was provided, ask that wg-server to assign an IP and
	// include the server's connection details in the response.
	if F.ServerID != primitive.NilObjectID {
		server, err := DB_FindServerByID(F.ServerID)
		if err == nil && server != nil {
			resp.ServerPubKey = server.WireGuardPubKey
			resp.ServerIP = server.IP
			resp.ServerPort = server.WireGuardPort
			if server.WGBaseURL != "" {
				ip, assignErr := callWGAssign(server.WGBaseURL, device.ID.Hex(), F.PublicKeyB64)
				if assignErr == nil && ip != "" {
					resp.AssignedIP = ip
					resp.Conf = buildWGConf(ip, F.PublicKeyB64, server)
				}
			}
		}
	}

	sendObject(w, resp)
}

// API_WGUnregister handles DELETE /v3/wg/register.
// Clears the WireGuard key from the device record.
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
	if err := DB_UpdateDevice(device); err != nil {
		senderr(w, 500, "Failed to update device", slog.Any("err", err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// API_WGPeers handles GET /v3/wg/peers.
// Returns all devices with a WireGuard key registered (DeviceID + hex public key).
// IP assignment is owned by each wg-server; AllowedIPs are not included here.
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

	devices, err := DB_GetDevices(100000, 0)
	if err != nil {
		senderr(w, 500, "Failed to fetch devices", slog.Any("err", err))
		return
	}

	resp := types.WGPeersResponse{
		Peers: make([]types.WGPeer, 0),
	}
	for _, d := range devices {
		if d.WireGuardKey == "" {
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

// callWGAssign sends a POST to the wg-server's /v3/wg/assign endpoint and
// returns the IP assigned to the device. It is called synchronously during
// /v3/session so the client receives its IP in the same response.
func callWGAssign(baseURL, deviceID, pubKeyB64 string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"DeviceID":  deviceID,
		"PubKeyB64": pubKeyB64,
	})
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	resp, err := http.Post(baseURL+"/v3/wg/assign", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wg-server returned %d", resp.StatusCode)
	}

	var result struct {
		IP string `json:"IP"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	return result.IP, nil
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
	return fmt.Sprintf("%x", b), nil
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
