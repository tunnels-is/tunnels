package wgserver

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

func meshReconcileInterval() time.Duration {
	if s := os.Getenv("TUNNELS_MESH_RECONCILE_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 2 * time.Minute
}

var (
	peerStore    *PeerStore
	activeConfig atomic.Pointer[Config]
)

func Init(ctx context.Context, controllerURL, apiKey, configPath string, insecureSkipVerify bool, logLevel string, showNewRules bool, done chan<- struct{}) {
	defer func() {
		if done != nil {
			close(done)
		}
	}()

	logger = slog.Default()

	INFO("fetching config from controller at ", controllerURL)

	var cfg *Config
	for {
		var err error
		cfg, err = FetchConfig(controllerURL, apiKey, configPath, insecureSkipVerify)
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

	if showNewRules {
		fmt.Println("=== wg-server --showNewRules: rules that would be installed ===")
		fmt.Printf("Config: serverID=%s subnet=%s subnet6=%s iface=%s port=%d publicIP=%s internetIface=%s\n",
			cfg.ServerID,
			cfg.WireGuardSubnet,
			cfgOrDash(cfg.WireGuardSubnet6),
			cfg.WireGuardIface,
			cfg.WireGuardPort,
			cfgOrDash(cfg.PublicIP),
			cfgOrDash(cfg.InternetIface),
		)
		for _, line := range PreviewRules(cfg) {
			fmt.Println("  " + line)
		}
		os.Exit(0)
	}

	flushWGRules(cfg)

	if err := preflightIPTables(cfg); err != nil {
		fmt.Println(err)
		return
	}

	peerStore = NewPeerStore(cfg.WireGuardSubnet, cfg.WireGuardSubnet6)
	activeConfig.Store(cfg)
	initSyncClient(cfg)

	if err := setupWireGuard(cfg, logLevel); err != nil {
		ERR("wireguard setup failed: ", err)

		stopFlowCleaner()
		if wgDevice != nil {
			wgDevice.Close()
		}
		return
	}

	if err := setupNet(cfg); err != nil {
		ERR("network setup failed: ", err)

		stopFlowCleaner()
		cleanupNet(cfg)
		if wgLazyBind != nil {
			wgLazyBind.Shutdown()
		}
		if wgDevice != nil {
			wgDevice.Close()
			wgLazyBind.WipeKeys()
		}
		return
	}

	if err := setupMesh(cfg, logLevel); err != nil {
		ERR("mesh setup failed (continuing without mesh): ", err)
	} else {
		go func() {
			reconcileMesh()
			t := time.NewTicker(meshReconcileInterval())
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					reconcileMesh()
				}
			}
		}()
	}

	INFO("wg-server started")

	<-ctx.Done()

	INFO("wg-server shutting down...")

	if wgLazyBind != nil {
		wgLazyBind.Shutdown()
	}
	if wgDevice != nil {
		wgDevice.Close()
	}

	if wgLazyBind != nil {
		wgLazyBind.WipeKeys()
	}
	stopFlowCleaner()
	cleanupMesh(cfg)
	cleanupNet(cfg)
	INFO("wg-server shutdown complete")
}
