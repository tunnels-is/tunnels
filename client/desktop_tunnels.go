package client

import (
	"errors"
	"os"

	"github.com/tunnels-is/tunnels/types"
)

// CloneConfig returns a shallow copy of the live config with copied slices.
// Nested pointer elements (servers, lists, records) are still shared; copy
// those before mutating an individual item.
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

// CreateTunnel allocates a new random tunnel and persists it.
// CreateTunnel allocates a new random tunnel and persists it.
func CreateTunnel() (*TunnelMeta, error) {
	return createRandomTunnel()
}

// SaveTunnel validates and writes tunnel metadata. oldTag is the previous
// identifier when renaming.
// SaveTunnel validates and writes tunnel metadata. oldTag is the previous
// identifier when renaming.
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

// DeleteTunnel removes a tunnel from disk and memory.
// DeleteTunnel removes a tunnel from disk and memory.
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

// SetTunnelPeers replaces the allow-list for a tunnel and announces it if
// the tunnel is currently connected.
// SetTunnelPeers replaces the allow-list for a tunnel and announces it if
// the tunnel is currently connected.
func SetTunnelPeers(tag string, allowedHosts []string, allowAll bool) ([]string, error) {
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
	meta.AllowAll = allowAll
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
			if err := t.AnnounceAllowedHosts(hosts, allowAll); err != nil {
				DEBUG("peer list announce failed: ", err)
			}
		}
		return false
	})

	return hosts, nil
}

// DisconnectTunnel stops reconnects and tears down the tunnel.
// DisconnectTunnel stops reconnects and tears down the tunnel.
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

// UpdateBlockLists re-downloads every configured DNS block list.
