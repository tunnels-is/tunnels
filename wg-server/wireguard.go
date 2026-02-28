package main

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

var wgDevice *device.Device

// setupWireGuard creates the WireGuard TUN interface and device, then configures
// the private key and listen port via the UAPI IPC protocol.
func setupWireGuard(cfg *Config) error {
	wgLogger := device.NewLogger(device.LogLevelVerbose, "[wg] ")

	tunDev, err := tun.CreateTUN(cfg.WireGuardIface, device.DefaultMTU)
	if err != nil {
		return fmt.Errorf("CreateTUN %q: %w", cfg.WireGuardIface, err)
	}

	wgDevice = device.NewDevice(tunDev, NewLazyBind(conn.NewDefaultBind(), SyncPeers), wgLogger)

	privKeyHex, err := b64ToHex(cfg.WireGuardPrivKey)
	if err != nil {
		return fmt.Errorf("invalid WireGuardPrivKey: %w", err)
	}

	conf := fmt.Sprintf("private_key=%s\nlisten_port=%d\n\n", privKeyHex, cfg.WireGuardPort)
	if err := ipcSet(conf); err != nil {
		return fmt.Errorf("IpcSet device config: %w", err)
	}

	if err := wgDevice.Up(); err != nil {
		return fmt.Errorf("device Up: %w", err)
	}

	pubKeyB64, err := derivePubKey(cfg.WireGuardPrivKey)
	if err != nil {
		return fmt.Errorf("derive pubkey: %w", err)
	}
	INFO("wireguard device started, pubkey=", pubKeyB64, " port=", cfg.WireGuardPort)
	return nil
}

// GetServerPubKeyB64 returns the server's WireGuard public key in base64 format.
func GetServerPubKeyB64(cfg *Config) (string, error) {
	return derivePubKey(cfg.WireGuardPrivKey)
}

// GetCurrentPeerKeys returns the set of hex-encoded public keys currently
// configured on the WireGuard device.
func GetCurrentPeerKeys() (map[string]struct{}, error) {
	data, err := ipcGet()
	if err != nil {
		return nil, err
	}
	result := make(map[string]struct{})
	for _, line := range strings.Split(data, "\n") {
		if after, ok := strings.CutPrefix(line, "public_key="); ok {
			result[strings.TrimSpace(after)] = struct{}{}
		}
	}
	return result, nil
}

// AddPeer adds or updates a single peer without disturbing existing peers or
// their established session keys.
func AddPeer(pubKeyHex, allowedIP string) error {
	conf := fmt.Sprintf("public_key=%s\nallowed_ip=%s\n\n", pubKeyHex, allowedIP)
	return ipcSet(conf)
}

// RemovePeer removes a single peer by its hex public key.
func RemovePeer(pubKeyHex string) error {
	conf := fmt.Sprintf("public_key=%s\nremove=true\n\n", pubKeyHex)
	return ipcSet(conf)
}

// ipcSet sends a UAPI configuration string to the WireGuard device.
// The conf string must NOT include the "set=1" prefix line; it starts
// directly with key=value pairs and ends with a blank line.
func ipcSet(conf string) error {
	if wgDevice == nil {
		return fmt.Errorf("wireguard device not initialized")
	}
	return wgDevice.IpcSetOperation(bufio.NewReader(strings.NewReader(conf)))
}

// ipcGet retrieves the current UAPI configuration from the WireGuard device.
func ipcGet() (string, error) {
	if wgDevice == nil {
		return "", fmt.Errorf("wireguard device not initialized")
	}
	var sb strings.Builder
	if err := wgDevice.IpcGetOperation(&sb); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// b64ToHex converts a base64-encoded 32-byte key to a lowercase hex string
// suitable for the WireGuard UAPI protocol.
func b64ToHex(b64 string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(b) != 32 {
		return "", fmt.Errorf("expected 32 bytes, got %d", len(b))
	}
	return hex.EncodeToString(b), nil
}
