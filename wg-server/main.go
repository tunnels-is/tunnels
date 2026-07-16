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

// meshReconcileInterval is how often the wg-server re-syncs its mesh peers with
// the controller. Defaults to 2 minutes; overridable via
// TUNNELS_MESH_RECONCILE_SECONDS (used by tests for fast convergence).
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

	// Drain any of wg-server's OWN leftover iptables rules before the preflight
	// conflict check. After a crash/ungraceful restart those self-installed
	// rules would otherwise trip preflight and block startup with the identical
	// config; flushWGRules deletes only the exact rules wg-server installs, so
	// preflight still catches genuinely foreign rules touching this config.
	flushWGRules(cfg)

	if err := preflightIPTables(cfg); err != nil {
		fmt.Println(err)
		return
	}

	// The peer store, active config and controller sync client MUST exist
	// before setupWireGuard brings the device up: a handshake can arrive the
	// moment the UDP port opens, and handleInitiation/reconcilePeer dereference
	// all three. Initializing them afterwards left a window where an early
	// handshake crashed the process on a nil peerStore.
	peerStore = NewPeerStore(cfg.WireGuardSubnet, cfg.WireGuardSubnet6)
	activeConfig.Store(cfg)
	initSyncClient(cfg)

	if err := setupWireGuard(cfg, logLevel); err != nil {
		ERR("wireguard setup failed: ", err)
		// setupWireGuard may have already started the flow cleaner before failing.
		stopFlowCleaner()
		if wgDevice != nil {
			wgDevice.Close()
		}
		return
	}

	if err := setupNet(cfg); err != nil {
		ERR("network setup failed: ", err)
		// Undo whatever partial rules/device state was created so a retry has a
		// clean table and no leaked TUN.
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

	// Server-to-server mesh. Non-fatal: a node still serves its own clients if
	// the mesh can't come up; only cross-server reachability is affected.
	if err := setupMesh(cfg, logLevel); err != nil {
		ERR("mesh setup failed (continuing without mesh): ", err)
	} else {
		go func() {
			reconcileMesh() // initial sync — in the goroutine so a slow controller can't delay startup
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
	// Only now that all receive routines have drained is it safe to erase the
	// server key material (no handleInitiation goroutine can still read it).
	if wgLazyBind != nil {
		wgLazyBind.WipeKeys()
	}
	stopFlowCleaner()
	cleanupMesh(cfg)
	cleanupNet(cfg)
	INFO("wg-server shutdown complete")
}
