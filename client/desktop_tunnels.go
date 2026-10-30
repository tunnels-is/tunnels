package client

import (
	"errors"
	"os"

	"github.com/tunnels-is/tunnels/types"
)

// CloneConfig ...
func CloneConfig() *Config {
	src := CONFIG.Load()
	if src == nil {
		return &Config{}
	}
	dst := *src
	dst.ControlServers = append([]*ControlServer(nil), src.ControlServers...)
	dst.DNSBlockLists = append([]*BlockList(nil), src.DNSBlockLists...)
	dst.DNSWhiteLists = append([]*BlockList(nil), src.DNSWhiteLists...)
	dst.DNSRecords = append([]*types.DNSRecord(nil), src.DNSRecords...)
	return &dst
}

// CreateTunnel ...
func CreateTunnel() (*TunnelMeta, error) {
	return createRandomTunnel()
}

// SaveTunnel ...
func SaveTunnel(meta *TunnelMeta, oldTag string) error {
	if meta == nil {
		return errors.New("tunnel metadata is required")
	}

	connected := false
	tunnelMapRange(func(t *TUN) bool {
		if t.CR != nil && t.CR.Tag == meta.Tag {
			connected = true
			return false
		}
		return true
	})
	if connected {
		return ErrTunnelConnected
	}

	msgs := validateTunnelMeta(meta, oldTag)
	if len(msgs) > 0 {
		return &ValidationError{Messages: msgs}
	}

	TunnelMetaMap.Store(meta.Tag, meta)
	if err := writeTunnelsToDisk(meta.Tag); err != nil {
		return err
	}

	if oldTag != "" && oldTag != meta.Tag {
		TunnelMetaMap.Delete(oldTag)
		state := STATE.Load()
		ext := meta.ConfigFormat
		if ext == "" {
			ext = tunnelFileSuffix
		}
		if err := os.Remove(state.TunnelsPath + oldTag + ext); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTunnel ...
func DeleteTunnel(tag string) error {
	if !safeTunnelTag(tag) {
		return errors.New("invalid tunnel tag")
	}
	state := STATE.Load()
	ext := tunnelFileSuffix
	if stored, ok := TunnelMetaMap.Load(tag); ok {
		if stored.ConfigFormat != "" {
			ext = stored.ConfigFormat
		}
	}
	_ = os.Remove(state.TunnelsPath + tag + ext)
	TunnelMetaMap.Delete(tag)
	return nil
}

// SetTunnelPeers ...
func SetTunnelPeers(tag string, allowedHosts []string, _ bool) ([]string, error) {
	meta, ok := TunnelMetaMap.Load(tag)
	if !ok {
		return nil, errors.New("tunnel not found")
	}

	seen := make(map[string]struct{}, len(allowedHosts))
	hosts := make([]string, 0, len(allowedHosts))
	for _, h := range allowedHosts {
		entry, err := NormalizeAllowedHost(h)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[entry]; dup {
			continue
		}
		seen[entry] = struct{}{}
		hosts = append(hosts, entry)
	}

	meta.AllowedHosts = hosts
	meta.AllowAll = false
	TunnelMetaMap.Store(meta.Tag, meta)
	if err := writeTunnelsToDisk(meta.Tag); err != nil {
		return nil, err
	}

	tunnelMapRange(func(t *TUN) bool {
		m := t.meta.Load()
		if m == nil || m.Tag != tag {
			return true
		}
		if t.GetState() >= TunnelConnected {
			if err := t.AnnounceAllowedHosts(hosts, false); err != nil {
				DEBUG("peer list announce failed: ", err)
			}
		}
		return false
	})

	return hosts, nil
}

// DisconnectTunnel ...
func DisconnectTunnel(id, tag string) error {
	if tag == "" {
		tunnelMapRange(func(t *TUN) bool {
			if t.ID == id {
				if m := t.meta.Load(); m != nil {
					tag = m.Tag
				}
				return false
			}
			return true
		})
	}
	if tag != "" {
		stopReconnect(tag)
	} else {
		stopAllReconnects()
	}
	return Disconnect(id, false)
}
