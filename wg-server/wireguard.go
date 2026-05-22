package wgserver

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/netip"
	"strings"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

var (
	wgDevice   *device.Device
	wgLazyBind *LazyBind
	aclStore   *ACLStore
)

func wgDeviceLogLevel(level string) int {
	switch level {
	case "silent":
		return device.LogLevelSilent
	case "error", "warn":
		return device.LogLevelError
	case "info", "debug":
		return device.LogLevelVerbose
	default:
		return device.LogLevelVerbose
	}
}

func setupWireGuard(cfg *Config, logLevel string) error {
	wgLogger := device.NewLogger(wgDeviceLogLevel(logLevel), "[wg] ")

	tunDev, err := tun.CreateTUN(cfg.WireGuardIface, device.DefaultMTU)
	if err != nil {
		return fmt.Errorf("CreateTUN %q: %w", cfg.WireGuardIface, err)
	}

	aclStore = NewACLStore()

	var tunInterface tun.Device = tunDev
	if cfg.PacketInspection {
		insp, err := newInspectingTUN(tunDev, aclStore, cfg)
		if err != nil {
			return fmt.Errorf("inspector setup: %w", err)
		}
		tunInterface = insp
		INFO("packet inspection enabled on ", cfg.WireGuardIface,
			" — peer ACLs active, control port udp/", aclControlPort)
	}

	if len(cfg.WireGuardPrivKey) != 32 {
		return fmt.Errorf("invalid WireGuardPrivKey: expected 32 bytes, got %d", len(cfg.WireGuardPrivKey))
	}
	pubBytes, err := curve25519.X25519(cfg.WireGuardPrivKey, curve25519.Basepoint)
	if err != nil {
		return fmt.Errorf("derive pubkey for LazyBind: %w", err)
	}

	// Copy private key for LazyBind — it must retain this for handshake decryption.
	// The original in cfg.WireGuardPrivKey is zeroed at the end of this function.
	privCopy := make([]byte, 32)
	copy(privCopy, cfg.WireGuardPrivKey)

	innerBind, err := buildInnerBind(cfg)
	if err != nil {
		return fmt.Errorf("build bind: %w", err)
	}
	wgLazyBind = NewLazyBind(innerBind, privCopy, pubBytes, cfg.HandshakeBufferSize, cfg.HandshakeRatePerIP, func() {})
	wgDevice = device.NewDevice(tunInterface, wgLazyBind, wgLogger)

	// Build IPC config using []byte so key material can be zeroed after use.
	privKeyHex := make([]byte, hex.EncodedLen(32))
	hex.Encode(privKeyHex, cfg.WireGuardPrivKey)
	conf := fmt.Appendf(nil, "private_key=%s\nlisten_port=%d\n\n", privKeyHex, cfg.WireGuardPort)
	zeroBytes(privKeyHex)

	if err := ipcSetBytes(conf); err != nil {
		zeroBytes(conf)
		return fmt.Errorf("IpcSet device config: %w", err)
	}
	zeroBytes(conf)

	if err := wgDevice.Up(); err != nil {
		return fmt.Errorf("device Up: %w", err)
	}

	pubKeyB64, err := derivePubKey(cfg.WireGuardPrivKey)
	if err != nil {
		return fmt.Errorf("derive pubkey: %w", err)
	}

	// Zero the config's copy — LazyBind has its own independent copy.
	zeroBytes(cfg.WireGuardPrivKey)

	INFO("wireguard device started, pubkey=", pubKeyB64, " port=", cfg.WireGuardPort)
	return nil
}

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

// sanitizeIPC strips newlines and carriage returns to prevent IPC directive injection.
func sanitizeIPC(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func AddPeer(pubKeyHex string, allowedIPs ...string) error {
	conf := fmt.Sprintf("public_key=%s\n", sanitizeIPC(pubKeyHex))
	for _, aip := range allowedIPs {
		conf += fmt.Sprintf("allowed_ip=%s\n", sanitizeIPC(aip))
	}
	conf += "\n"
	return ipcSet(conf)
}

func AddPeerWithEndpoint(pubKeyHex, endpoint string, allowedIPs ...string) error {
	conf := fmt.Sprintf("public_key=%s\n", sanitizeIPC(pubKeyHex))
	for _, aip := range allowedIPs {
		conf += fmt.Sprintf("allowed_ip=%s\n", sanitizeIPC(aip))
	}
	conf += fmt.Sprintf("endpoint=%s\npersistent_keepalive_interval=15\n\n", sanitizeIPC(endpoint))
	return ipcSet(conf)
}

func RemovePeer(pubKeyHex string) error {
	conf := fmt.Sprintf("public_key=%s\nremove=true\n\n", sanitizeIPC(pubKeyHex))
	return ipcSet(conf)
}

func ipcSet(conf string) error {
	if wgDevice == nil {
		return fmt.Errorf("wireguard device not initialized")
	}
	return wgDevice.IpcSetOperation(bufio.NewReader(strings.NewReader(conf)))
}

func ipcSetBytes(conf []byte) error {
	if wgDevice == nil {
		return fmt.Errorf("wireguard device not initialized")
	}
	return wgDevice.IpcSetOperation(bufio.NewReader(bytes.NewReader(conf)))
}

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

// buildInnerBind returns the underlying conn.Bind for wireguard-go.
// When cfg.PublicIP is set we pin the listening socket to that address; this
// lets multiple wg-server instances coexist on the same UDP port across
// different public IPs on the same host. Empty PublicIP falls back to
// wireguard-go's default wildcard bind.
func buildInnerBind(cfg *Config) (conn.Bind, error) {
	if cfg.PublicIP == "" {
		return conn.NewDefaultBind(), nil
	}
	addr, err := netip.ParseAddr(cfg.PublicIP)
	if err != nil {
		return nil, fmt.Errorf("parse PublicIP %q: %w", cfg.PublicIP, err)
	}
	INFO("WireGuard bind pinned to ", addr, ":", cfg.WireGuardPort)
	return newPinnedBind(addr), nil
}

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
