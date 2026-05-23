package main

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

const (
	wgPeersDefaultLimit = 500
	wgPeersMaxLimit     = 5000
)

func API_WGPeers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	if _, ok := HTTP_validateWGKey(r); !ok {
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
	for _, d := range devices {
		if d.WireGuardKey == "" {
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

	if _, ok := HTTP_validateWGKey(r); !ok {
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
		senderr(w, 404, "peer not found")
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

	server, err := DB_FindServerByID(serverID)
	if err != nil || server == nil {
		senderr(w, 404, "Server not found")
		return
	}

	pubKey := r.URL.Query().Get("pubKey")
	if pubKey == "" {
		senderr(w, 401, "No pubkey given")
		return
	}

	d, err := DB_FindDeviceByWGKey(pubKey)
	if err != nil {
		senderr(w, 401, "Pubkey not on record")
		return
	}

	sendObject(w, map[string]string{
		"WireGuardPubKey":  server.WireGuardPubKey,
		"WireGuardPort":    strconv.Itoa(server.WireGuardPort),
		"ServerIP":         server.IP,
		"WireGuardIP":      d.WireGuardIP,
		"WireGuardIPv6":    d.WireGuardIPv6,
		"WireGuardSubnet":  server.WireGuardSubnet,
		"WireGuardSubnet6": server.WireGuardSubnet6,
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

	devices, err := DB_GetDevices(100000, 0)
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

// assignNextWireGuardIPv6 allocates the next free IPv6 address from the
// server's WireGuardSubnet6. Returns "" (no error) if no IPv6 subnet is
// configured on the server.
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

	devices, err := DB_GetDevices(100000, 0)
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
		if !used[candidate] {
			return candidate.String(), nil
		}
		candidate = candidate.Next()
	}
	return "", fmt.Errorf("WireGuard IPv6 subnet %s is exhausted", server.WireGuardSubnet6)
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

// validateCIDR returns an error if s is set but not a parseable CIDR. Empty
// strings are allowed (caller decides whether they're required).
func validateCIDR(s string) error {
	if s == "" {
		return nil
	}
	_, _, err := net.ParseCIDR(s)
	return err
}

func API_WGServerConfigFetch(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	server, ok := HTTP_validateWGKey(r)
	if !ok {
		senderr(w, 401, "Unauthorized")
		return
	}

	// Accept the wg-server's public key from the request header.
	pubKeyB64 := r.Header.Get("X-WG-PubKey")
	if pubKeyB64 != "" && pubKeyB64 != server.WireGuardPubKey {
		server.WireGuardPubKey = pubKeyB64
		_, _ = DB_UpdateServer(server)
	}

	resp := &types.WGServerConfigResponse{
		ServerID:           server.ID.String(),
		ServerIP:           server.IP,
		WireGuardPort:      server.WireGuardPort,
		WireGuardSubnet:    server.WireGuardSubnet,
		WireGuardSubnet6:   server.WireGuardSubnet6,
		WireGuardIface:     server.WireGuardIface,
		InternetIface:      server.InternetIface,
		PacketInspection:   server.PacketInspection,
		InsecureSkipVerify: server.InsecureSkipVerify,
	}

	sendObject(w, resp)
}

func API_WGServers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	if _, ok := HTTP_validateWGKey(r); !ok {
		senderr(w, 401, "Unauthorized")
		return
	}

	excludeID := r.URL.Query().Get("excludeID")

	servers, err := DB_FindAllServers()
	if err != nil {
		senderr(w, 500, "Failed to fetch servers", slog.Any("err", err))
		return
	}

	resp := types.WGServersResponse{Servers: make([]types.WGServerInfo, 0)}
	for _, s := range servers {
		if s.WireGuardPubKey == "" || s.WireGuardSubnet == "" {
			continue
		}
		if excludeID != "" && s.ID.String() == excludeID {
			continue
		}
		resp.Servers = append(resp.Servers, types.WGServerInfo{
			WireGuardPubKey:  s.WireGuardPubKey,
			WireGuardPort:    s.WireGuardPort,
			WireGuardSubnet:  s.WireGuardSubnet,
			WireGuardSubnet6: s.WireGuardSubnet6,
			IP:               s.IP,
		})
	}

	sendObject(w, resp)
}

func buildWGConf(assignedIP, assignedIPv6, clientPubKeyB64 string, server *types.Server) string {
	address := assignedIP + "/32"
	if assignedIPv6 != "" {
		address += ", " + assignedIPv6 + "/128"
	}

	allowedIPs := "0.0.0.0/0"
	if assignedIPv6 != "" {
		allowedIPs += ", ::/0"
	}

	dns := "1.1.1.1"
	if assignedIPv6 != "" {
		dns += ", 2606:4700:4700::1111"
	}

	return fmt.Sprintf(`[Interface]
Address = %s
# PrivateKey = <paste your private key here>
DNS = %s

[Peer]
PublicKey = %s
Endpoint = %s:%d
AllowedIPs = %s
PersistentKeepalive = 15
`, address, dns, server.WireGuardPubKey, server.IP, server.WireGuardPort, allowedIPs)
}
