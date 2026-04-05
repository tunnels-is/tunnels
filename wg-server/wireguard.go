package wgserver

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

var wgDevice *device.Device

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

	var tunInterface tun.Device = tunDev
	if cfg.PacketInspection {
		tunInterface = &inspectingTUN{tunDev}
		INFO("packet inspection enabled on ", cfg.WireGuardIface)
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

	wgDevice = device.NewDevice(tunInterface, NewLazyBind(conn.NewDefaultBind(), privCopy, pubBytes, func() {}), wgLogger)

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

func AddPeer(pubKeyHex string, allowedIPs ...string) error {
	conf := fmt.Sprintf("public_key=%s\n", pubKeyHex)
	for _, aip := range allowedIPs {
		conf += fmt.Sprintf("allowed_ip=%s\n", aip)
	}
	conf += "\n"
	return ipcSet(conf)
}

func AddPeerWithEndpoint(pubKeyHex, endpoint string, allowedIPs ...string) error {
	conf := fmt.Sprintf("public_key=%s\n", pubKeyHex)
	for _, aip := range allowedIPs {
		conf += fmt.Sprintf("allowed_ip=%s\n", aip)
	}
	conf += fmt.Sprintf("endpoint=%s\npersistent_keepalive_interval=15\n\n", endpoint)
	return ipcSet(conf)
}

func RemovePeer(pubKeyHex string) error {
	conf := fmt.Sprintf("public_key=%s\nremove=true\n\n", pubKeyHex)
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
