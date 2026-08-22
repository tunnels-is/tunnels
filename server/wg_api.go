package main

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

var wgIPAllocMu sync.Mutex

const (
	wgPeersDefaultLimit = 500
	wgPeersMaxLimit     = 5000
)

func API_WGPeers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	server := getServerFromContext(r.Context())
	if server == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	limit := wgPeersDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			senderr(w, 400, "limit must be a positive integer")
			return
		}
		if n > wgPeersMaxLimit {
			n = wgPeersMaxLimit
		}
		limit = n
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			senderr(w, 400, "offset must be a non-negative integer")
			return
		}
		offset = n
	}

	devices, err := DB_GetDevices(int64(limit), int64(offset))
	if err != nil {
		senderr(w, 500, "Failed to fetch devices", slog.Any("err", err))
		return
	}

	resp := types.WGPeersResponse{
		Peers:  make([]types.WGPeer, 0, len(devices)),
		Limit:  limit,
		Offset: offset,
	}

	userCache := make(map[uuid.UUID]*User)
	now := time.Now()
	for _, d := range devices {
		if d.WireGuardKey == "" || d.ServerID != server.ID {
			continue
		}
		owner, ok := userCache[d.UserID]
		if !ok {
			owner, _ = DB_findUserByID(d.UserID)
			userCache[d.UserID] = owner
		}
		if owner == nil || owner.Disabled {
			continue
		}
		if !owner.SubExpiration.IsZero() && now.After(owner.SubExpiration) {
			continue
		}
		if !hasSharedOrNoGroup(owner.Groups, server.Groups) {
			continue
		}
		hexKey, err := b64KeyToHex(d.WireGuardKey)
		if err != nil {
			continue
		}
		resp.Peers = append(resp.Peers, types.WGPeer{
			PublicKeyHex:  hexKey,
			DeviceID:      d.ID.String(),
			WireGuardIP:   d.WireGuardIP,
			WireGuardIPv6: d.WireGuardIPv6,
		})
	}

	if len(devices) < limit {
		resp.NextOffset = -1
	} else {
		resp.NextOffset = offset + limit
	}

	sendObject(w, resp)
}

func API_WGPeer(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	server := getServerFromContext(r.Context())
	if server == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	pubKeyB64 := r.URL.Query().Get("pubkey")
	if pubKeyB64 == "" {
		senderr(w, 400, "pubkey query parameter is required")
		return
	}
	raw, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil || len(raw) != 32 {
		senderr(w, 400, "pubkey must be a base64-encoded 32-byte key")
		return
	}

	dev, err := DB_FindDeviceByWGKey(pubKeyB64)
	if err != nil {
		senderr(w, 500, "Failed to look up device", slog.Any("err", err))
		return
	}
	if dev == nil {
		senderr(w, 404, "device not found")
		return
	}

	if dev.ServerID != server.ID {
		senderr(w, 401, "device not allowed on this server")
		return
	}

	user, err := DB_findUserByID(dev.UserID)
	if err != nil {
		senderr(w, 500, "error looking up user")
		return
	}
	if user == nil {
		senderr(w, 401, "user/device not allowed to connect")
		return
	}
	if user.Disabled {
		senderr(w, 403, "user account is disabled")
		return
	}

	if !user.SubExpiration.IsZero() && time.Now().After(user.SubExpiration) {
		senderr(w, 403, "user subscription has expired")
		return
	}

	if !hasSharedOrNoGroup(user.Groups, server.Groups) {
		senderr(w, 401, "user/device not allowed to connect")
		return
	}

	hexKey, err := b64KeyToHex(dev.WireGuardKey)
	if err != nil {
		senderr(w, 500, "Failed to encode device key", slog.Any("err", err))
		return
	}

	sendObject(w, types.WGPeer{
		PublicKeyHex:  hexKey,
		DeviceID:      dev.ID.String(),
		WireGuardIP:   dev.WireGuardIP,
		WireGuardIPv6: dev.WireGuardIPv6,
	})
}

func API_WGConfig(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	serverIDStr := r.URL.Query().Get("serverID")
	if serverIDStr == "" {
		senderr(w, 400, "serverID query parameter is required")
		return
	}
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		senderr(w, 400, "Invalid serverID")
		return
	}

	pubKey := r.URL.Query().Get("pubKey")
	if pubKey == "" {
		senderr(w, 400, "No pubkey given")
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized - no user in context")
		return
	}

	if !user.SubExpiration.IsZero() && time.Now().After(user.SubExpiration) {
		senderr(w, 403, "subscription expired")
		return
	}

	server, err := DB_FindServerByID(serverID)
	if err != nil || server == nil {
		senderr(w, 404, "Server not found")
		return
	}

	if !hasSharedOrNoGroup(user.Groups, server.Groups) {
		senderr(w, 401, "Unauthorized - no group access")
		return
	}

	d, err := DB_FindDeviceByWGKey(pubKey)
	if err != nil {
		senderr(w, 500, "Database error looking up device")
		return
	}

	deviceIP := ""
	deviceIPv6 := ""
	if d != nil && d.UserID != user.ID {
		// Occupancy oracle: do not distinguish "someone else's key"
		// from an unknown key.
		d = nil
	}
	if d != nil {

		if d.ServerID == serverID {
			deviceIP = d.WireGuardIP
			deviceIPv6 = d.WireGuardIPv6
		}
	}

	sendObject(w, map[string]any{
		"WireGuardPubKey":  server.WireGuardPubKey,
		"WireGuardPort":    strconv.Itoa(server.WireGuardPort),
		"ServerIP":         server.IP,
		"WireGuardIP":      deviceIP,
		"WireGuardIPv6":    deviceIPv6,
		"WireGuardSubnet":  server.WireGuardSubnet,
		"WireGuardSubnet6": server.WireGuardSubnet6,
		"WANCIDR":          wanCIDRForServer(server),
		"EnableFirewall":   server.EnableFirewall,
	})
}

func assignNextWireGuardIP(serverID uuid.UUID) (string, error) {
	server, err := DB_FindServerByID(serverID)
	if err != nil || server == nil {
		return "", fmt.Errorf("server not found")
	}
	if server.WireGuardSubnet == "" {
		return "", fmt.Errorf("server has no WireGuard subnet configured")
	}
	_, ipNet, err := net.ParseCIDR(server.WireGuardSubnet)
	if err != nil {
		return "", fmt.Errorf("invalid subnet %q: %w", server.WireGuardSubnet, err)
	}

	devices, err := DB_GetAllDevices()
	if err != nil {
		return "", fmt.Errorf("list devices: %w", err)
	}

	used := make(map[uint32]bool)
	for _, d := range devices {
		if d.WireGuardIP == "" {
			continue
		}
		ip4 := net.ParseIP(d.WireGuardIP).To4()
		if ip4 != nil && ipNet.Contains(ip4) {
			used[wgIPToU32(ip4)] = true
		}
	}

	base := wgIPToU32(ipNet.IP.To4()) + 2
	for {
		next := wgU32ToIP(base)
		if !ipNet.Contains(next) {
			return "", fmt.Errorf("WireGuard subnet %s is exhausted", server.WireGuardSubnet)
		}
		if types.IsReservedWireGuardIPv4(ipNet, next) {
			base++
			continue
		}
		if !used[base] {
			return next.String(), nil
		}
		base++
	}
}

func wgIPToU32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func wgU32ToIP(n uint32) net.IP {
	return net.IP{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
}

func assignNextWireGuardIPv6(serverID uuid.UUID) (string, error) {
	server, err := DB_FindServerByID(serverID)
	if err != nil || server == nil {
		return "", fmt.Errorf("server not found")
	}
	if server.WireGuardSubnet6 == "" {
		return "", nil
	}
	prefix, err := netip.ParsePrefix(server.WireGuardSubnet6)
	if err != nil {
		return "", fmt.Errorf("invalid IPv6 subnet %q: %w", server.WireGuardSubnet6, err)
	}

	prefix = prefix.Masked()

	devices, err := DB_GetAllDevices()
	if err != nil {
		return "", fmt.Errorf("list devices: %w", err)
	}

	used := make(map[netip.Addr]bool)
	for _, d := range devices {
		if d.WireGuardIPv6 == "" {
			continue
		}
		if addr, parseErr := netip.ParseAddr(d.WireGuardIPv6); parseErr == nil && prefix.Contains(addr) {
			used[addr] = true
		}
	}

	candidate := prefix.Addr().Next().Next()
	for prefix.Contains(candidate) {
		if types.IsReservedWireGuardIPv6(prefix, candidate) {
			candidate = candidate.Next()
			continue
		}
		if !used[candidate] {
			return candidate.String(), nil
		}
		candidate = candidate.Next()
	}
	return "", fmt.Errorf("WireGuard IPv6 subnet %s is exhausted", server.WireGuardSubnet6)
}

func rejectServerWireGuardKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("WireGuard key required")
	}
	servers, err := DB_FindAllServers(10000, 0)
	if err != nil {
		return err
	}
	for _, s := range servers {
		if s == nil || s.WireGuardPubKey == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(s.WireGuardPubKey), []byte(key)) == 1 {
			return fmt.Errorf("WireGuard key is reserved")
		}
	}
	return nil
}

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

func HTTP_validateWGKey(r *http.Request) (*types.Server, bool) {
	key := r.Header.Get("X-WG-KEY")
	if key == "" {
		return nil, false
	}
	s, err := DB_FindServerByAPIKey(key)
	if err != nil || s == nil {
		return nil, false
	}
	return s, true
}

func validateCIDR(s string) error {
	if s == "" {
		return nil
	}
	_, _, err := net.ParseCIDR(s)
	return err
}

func API_WGServerConfigFetch(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	server := getServerFromContext(r.Context())
	if server == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	pubKeyB64 := r.Header.Get("X-WG-PubKey")
	if err := pinWireGuardPubKey(server, pubKeyB64); err != nil {
		senderr(w, http.StatusConflict, err.Error())
		return
	}

	resp := &types.WGServerConfigResponse{
		ServerID:          server.ID.String(),
		ServerIP:          server.IP,
		WireGuardPort:     server.WireGuardPort,
		WireGuardMeshPort: meshPortForServer(server),
		WireGuardSubnet:   server.WireGuardSubnet,
		WireGuardSubnet6:  server.WireGuardSubnet6,
		WireGuardIface:    server.WireGuardIface,
		InternetIface:     server.InternetIface,
		EnableFirewall:    server.EnableFirewall,
	}

	sendObject(w, resp)
}

// pinWireGuardPubKey records the node's pubkey on first bind. A different
// key is rejected until an admin rotates the server API key (which clears
// WireGuardPubKey). Same key on restart is a no-op.
func pinWireGuardPubKey(server *types.Server, pubKeyB64 string) error {
	if pubKeyB64 == "" {
		return nil
	}
	if server.WireGuardPubKey == "" {
		if err := DB_SetServerWireGuardPubKey(server.ID, pubKeyB64); err != nil {
			return err
		}
		server.WireGuardPubKey = pubKeyB64
		return nil
	}
	if pubKeyB64 == server.WireGuardPubKey {
		return nil
	}
	return fmt.Errorf("WireGuard public key is pinned; rotate the server API key to replace it")
}

func meshPortForServer(s *types.Server) int {
	if s.WireGuardMeshPort != 0 {
		return s.WireGuardMeshPort
	}
	return s.WireGuardPort + 1
}

func API_WGMesh(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	server := getServerFromContext(r.Context())
	if server == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	resp := types.WGMeshResponse{Peers: make([]types.WGMeshPeer, 0)}
	if server.MeshGroupID == "" {
		sendObject(w, resp)
		return
	}

	siblings, err := DB_FindServersByMeshGroup(server.MeshGroupID)
	if err != nil {
		senderr(w, 500, "Failed to fetch mesh peers", slog.Any("err", err))
		return
	}

	for _, s := range siblings {
		if s.ID == server.ID {
			continue
		}
		if s.WireGuardPubKey == "" || s.IP == "" || s.WireGuardSubnet == "" {
			continue
		}
		hexKey, err := b64KeyToHex(s.WireGuardPubKey)
		if err != nil {
			continue
		}
		subnets := []string{s.WireGuardSubnet}
		if s.WireGuardSubnet6 != "" {
			subnets = append(subnets, s.WireGuardSubnet6)
		}
		resp.Peers = append(resp.Peers, types.WGMeshPeer{
			PublicKeyHex:   hexKey,
			Endpoint:       net.JoinHostPort(s.IP, strconv.Itoa(meshPortForServer(s))),
			AllowedSubnets: subnets,
		})
	}

	sendObject(w, resp)
}
