package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)


type LocalDevice struct {
	ID               string    `json:"ID"`
	ServerID         string    `json:"ServerID"`
	Tag              string    `json:"Tag"`
	WireGuardPrivKey string    `json:"WireGuardPrivKey"`
	WireGuardPubKey  string    `json:"WireGuardPubKey,omitempty"`
	WireGuardIP      string    `json:"WireGuardIP,omitempty"`
	CreatedAt        time.Time `json:"CreatedAt"`
}


type LocalDeviceInfo struct {
	ID              string    `json:"ID"`
	ServerID        string    `json:"ServerID"`
	Tag             string    `json:"Tag"`
	WireGuardPubKey string    `json:"WireGuardPubKey,omitempty"`
	WireGuardIP     string    `json:"WireGuardIP,omitempty"`
	CreatedAt       time.Time `json:"CreatedAt"`
}

func (d *LocalDevice) info() LocalDeviceInfo {
	return LocalDeviceInfo{
		ID:              d.ID,
		ServerID:        d.ServerID,
		Tag:             d.Tag,
		WireGuardPubKey: d.WireGuardPubKey,
		WireGuardIP:     d.WireGuardIP,
		CreatedAt:       d.CreatedAt,
	}
}

func localDevicePath(id string) (string, error) {
	s := STATE.Load()
	if s.DevicesPath == "" || s.ActiveAccountHash == "" {
		return "", errors.New("no active account workspace")
	}
	if id == "" || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return "", fmt.Errorf("invalid device id")
	}
	return s.DevicesPath + id, nil
}

func saveLocalDevice(d *LocalDevice) error {
	if d == nil || d.ID == "" {
		return errors.New("device id required")
	}
	s := STATE.Load()
	if s.ActiveAccountHash == "" || s.DevicesPath == "" {
		return errors.New("no active account workspace")
	}
	path, err := localDevicePath(d.ID)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	blob, err := encryptAccountBlob(raw, s.ActiveAccountHash)
	if err != nil {
		return err
	}
	return os.WriteFile(path, blob, 0o600)
}

func loadLocalDevice(id string) (*LocalDevice, error) {
	s := STATE.Load()
	if s.ActiveAccountHash == "" {
		return nil, errors.New("no active account workspace")
	}
	path, err := localDevicePath(id)
	if err != nil {
		return nil, err
	}
	fb, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw, err := decryptAccountBlob(fb, s.ActiveAccountHash)
	if err != nil {
		return nil, err
	}
	d := new(LocalDevice)
	if err := json.Unmarshal(raw, d); err != nil {
		return nil, err
	}
	return d, nil
}

func listLocalDevices() ([]*LocalDevice, error) {
	s := STATE.Load()
	if s.DevicesPath == "" || s.ActiveAccountHash == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(s.DevicesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]*LocalDevice, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !e.Type().IsRegular() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".bak") || strings.HasPrefix(name, ".") {
			continue
		}
		d, err := loadLocalDevice(name)
		if err != nil {
			ERROR("unable to load local device:", name, err)
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

func listLocalDeviceInfo() ([]LocalDeviceInfo, error) {
	devs, err := listLocalDevices()
	if err != nil {
		return nil, err
	}
	out := make([]LocalDeviceInfo, 0, len(devs))
	for _, d := range devs {
		out = append(out, d.info())
	}
	return out, nil
}

func findLocalDeviceByServerID(serverID string) (*LocalDevice, error) {
	if serverID == "" {
		return nil, nil
	}
	devs, err := listLocalDevices()
	if err != nil {
		return nil, err
	}
	target := strings.ToLower(serverID)
	for _, d := range devs {
		if strings.ToLower(d.ServerID) == target {
			return d, nil
		}
	}
	return nil, nil
}

func shortDeviceTag() string {
	return "client-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}


func createLocalDeviceForServer(cr *ConnectionRequest, serverID, tag string) (*LocalDevice, *wgServerConfig, error) {
	priv, err := generateWGPrivKey()
	if err != nil {
		return nil, nil, err
	}
	pub, err := deriveWGPubKey(priv)
	if err != nil {
		return nil, nil, err
	}
	deviceTag := tag
	if deviceTag == "" || deviceTag == DefaultTunnelName {
		deviceTag = shortDeviceTag()
	}

	cfg, device, err := createServerDeviceFull(cr, serverID, pub, deviceTag)
	if err != nil {
		return nil, nil, err
	}
	if cfg.WireGuardIP == "" {
		return nil, nil, errors.New("controller did not assign a WireGuard IP")
	}

	if full, e := getServerWGConfig(cr, serverID, pub); e == nil && full != nil {
		cfg.EnableFirewall = full.EnableFirewall
		if full.WireGuardPubKey != "" {
			cfg.WireGuardPubKey = full.WireGuardPubKey
		}
		if full.WireGuardSubnet != "" {
			cfg.WireGuardSubnet = full.WireGuardSubnet
		}
		if full.WireGuardSubnet6 != "" {
			cfg.WireGuardSubnet6 = full.WireGuardSubnet6
		}
		if full.WANCIDR != "" {
			cfg.WANCIDR = full.WANCIDR
		}
	}

	id := uuid.NewString()
	if device != nil && device.ID != uuid.Nil {
		id = device.ID.String()
	}
	local := &LocalDevice{
		ID:               id,
		ServerID:         serverID,
		Tag:              deviceTag,
		WireGuardPrivKey: priv,
		WireGuardPubKey:  pub,
		WireGuardIP:      cfg.WireGuardIP,
		CreatedAt:        time.Now(),
	}
	if err := saveLocalDevice(local); err != nil {
		return nil, nil, fmt.Errorf("save local device: %w", err)
	}
	INFO("created local device", local.ID, "for server", serverID, "ip", cfg.WireGuardIP)
	return local, cfg, nil
}


func resolveLocalDeviceForServer(cr *ConnectionRequest, serverID, tag string) (*LocalDevice, *wgServerConfig, error) {
	existing, err := findLocalDeviceByServerID(serverID)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil && existing.WireGuardPrivKey != "" {
		pub := existing.WireGuardPubKey
		if pub == "" {
			pub, err = deriveWGPubKey(existing.WireGuardPrivKey)
			if err != nil {
				return nil, nil, err
			}
			existing.WireGuardPubKey = pub
			_ = saveLocalDevice(existing)
		}
		cfg, cfgErr := getServerWGConfig(cr, serverID, pub)
		if cfgErr != nil {
			return nil, nil, cfgErr
		}
		if cfg.WireGuardIP == "" {
			INFO("re-registering local device on controller:", existing.ID)
			cfg, _, cfgErr = createServerDeviceFull(cr, serverID, pub, existing.Tag)
			if cfgErr != nil {
				return nil, nil, cfgErr
			}
		}
		if cfg.WireGuardIP != "" {
			existing.WireGuardIP = cfg.WireGuardIP
			_ = saveLocalDevice(existing)
		}
		INFO("using local device", existing.ID, "for server", serverID)
		return existing, cfg, nil
	}
	return createLocalDeviceForServer(cr, serverID, tag)
}
