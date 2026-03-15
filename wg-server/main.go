package wgserver

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
)

var (
	peerStore    *PeerStore
	activeConfig atomic.Pointer[Config]
)

// Init starts the wg-server feature. It fetches config from the controller
// (retrying until successful or ctx is cancelled), sets up WireGuard and
// networking, then blocks until ctx is done before cleaning up.
func Init(ctx context.Context, controllerURL, apiKey string, insecureSkipVerify bool) {
	logger = slog.Default()

	INFO("fetching config from controller at ", controllerURL)

	var cfg *Config
	var err error
	cfg, err = FetchConfig(controllerURL, apiKey, insecureSkipVerify)
	if err != nil {
		INFO("failed to fetch config from controller: ", err, " (retrying in 5s)")
		os.Exit(1)
	}

	INFO("config fetched, serverID=", cfg.ServerID,
		" subnet=", cfg.WireGuardSubnet, " iface=", cfg.WireGuardIface)

	if err := setupWireGuard(cfg); err != nil {
		ERR("wireguard setup failed: ", err)
		return
	}

	if err := setupNet(cfg); err != nil {
		ERR("network setup failed: ", err)
		return
	}

	peerStore = NewPeerStore(cfg.WireGuardSubnet)
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
