package wgserver

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

var (
	peerStore    *PeerStore
	activeConfig atomic.Pointer[Config]
)

// Init starts the wg-server feature. It fetches config from the controller
// (retrying until successful or ctx is cancelled), sets up WireGuard and
// networking, then blocks until ctx is done before cleaning up.
func Init(ctx context.Context, controllerURL, apiKey string, insecureSkipVerify bool, logLevel string) {
	logger = slog.Default()

	INFO("fetching config from controller at ", controllerURL)

	var cfg *Config
	for {
		var err error
		cfg, err = FetchConfig(controllerURL, apiKey, insecureSkipVerify)
		if err == nil {
			break
		}
		INFO("failed to fetch config from controller: ", err, " (retrying in 5s)")
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}

	subnet6Log := ""
	if cfg.WireGuardSubnet6 != "" {
		subnet6Log = " subnet6=" + cfg.WireGuardSubnet6
	}
	INFO("config fetched, serverID=", cfg.ServerID,
		" subnet=", cfg.WireGuardSubnet, subnet6Log, " iface=", cfg.WireGuardIface)

	if err := setupWireGuard(cfg, logLevel); err != nil {
		ERR("wireguard setup failed: ", err)
		return
	}

	if err := setupNet(cfg); err != nil {
		ERR("network setup failed: ", err)
		return
	}

	peerStore = NewPeerStore(cfg.WireGuardSubnet, cfg.WireGuardSubnet6)
	activeConfig.Store(cfg)
	initSyncClient(cfg)

	INFO("wg-server started")

	<-ctx.Done()

	INFO("wg-server shutting down...")
	if wgDevice != nil {
		wgDevice.Close()
	}
	cleanupNet(cfg)
	INFO("wg-server shutdown complete")
}
