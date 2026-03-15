package wgserver

import (
	"bufio"
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

func setupWireGuard(cfg *Config) error {
	wgLogger := device.NewLogger(device.LogLevelVerbose, "[wg] ")

	tunDev, err := tun.CreateTUN(cfg.WireGuardIface, device.DefaultMTU)
	if err != nil {
		return fmt.Errorf("CreateTUN %q: %w", cfg.WireGuardIface, err)
	}

	var tunInterface tun.Device = tunDev
	if cfg.PacketInspection {
		tunInterface = &inspectingTUN{tunDev}
		INFO("packet inspection enabled on ", cfg.WireGuardIface)
	}

	privBytes, err := base64.StdEncoding.DecodeString(cfg.WireGuardPrivKey)
	if err != nil || len(privBytes) != 32 {
		return fmt.Errorf("invalid WireGuardPrivKey: %w", err)
	}
	pubBytes, err := curve25519.X25519(privBytes, curve25519.Basepoint)
	if err != nil {
		return fmt.Errorf("derive pubkey for LazyBind: %w", err)
	}

	wgDevice = device.NewDevice(tunInterface, NewLazyBind(conn.NewDefaultBind(), privBytes, pubBytes, func() {}), wgLogger)

	privKeyHex := hex.EncodeToString(privBytes)

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

func AddPeer(pubKeyHex, allowedIP string) error {
	conf := fmt.Sprintf("public_key=%s\nallowed_ip=%s\n\n", pubKeyHex, allowedIP)
	return ipcSet(conf)
}

func AddPeerWithEndpoint(pubKeyHex, allowedIP, endpoint string) error {
	conf := fmt.Sprintf(
		"public_key=%s\nallowed_ip=%s\nendpoint=%s\npersistent_keepalive_interval=15\n\n",
		pubKeyHex, allowedIP, endpoint,
	)
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
