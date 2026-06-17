package wgserver

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

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

	if err := preflightIPTables(cfg); err != nil {
		fmt.Println(err)
		return
	}

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
	// Order matters: Shutdown() closes LazyBind.done, which unblocks the
	// reinjectReceive goroutine that wireguard-go's RoutineReceiveIncoming is
	// parked in. wgDevice.Close() waits on every receive routine to finish
	// (device.state.stopping.Wait()); if reinjectReceive hasn't been told to
	// exit, Close deadlocks. LazyBind.Close() (called transitively by
	// wgDevice.Close() during BindClose) is intentionally non-destructive and
	// does NOT close LazyBind.done — that's reserved for Shutdown — so the
	// signal has to come from us, here, first.
	if wgLazyBind != nil {
		wgLazyBind.Shutdown()
	}
	if wgDevice != nil {
		wgDevice.Close()
	}
	stopFlowCleaner()
	cleanupNet(cfg)
	INFO("wg-server shutdown complete")
}
