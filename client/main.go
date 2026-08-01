package client

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func printInfo() {
	conf := CONFIG.Load()
	s := STATE.Load()

	scheme := "http"
	if EnableTLS {
		scheme = "https"
	}

	blue := "\033[1;34m"
	dim := "\033[34m"
	bold := "\033[1m"
	warn := "\033[33m"
	reset := "\033[0m"
	divider := dim + "  ────────────────────────────────────────────" + reset + "\n"

	fmt.Printf("\n"+
		blue+
		"   _____ _____ _____ _____ _____ __    _____\n"+
		"  |_   _|  |  |   | |   | |   __|  |  |   __|\n"+
		"    | | |  |  | | | | | | |   __|  |__|__   |\n"+
		"    |_| |_____|_|___|_|___|_____|_____|_____|\n"+
		reset+
		"                                    tunnels.is\n"+
		"\n"+
		divider+
		"\n"+
		"  "+bold+"🌐"+reset+"  APP        %s://%s:%s\n"+
		"  "+bold+"📁"+reset+"  BASE PATH  %s\n"+
		"\n"+
		divider+
		"\n"+
		"  ·  configure system dns servers to prevent leaks\n"+
		"  ·  --basePath to change the config directory\n"+
		"\n"+
		"  "+warn+"⚠"+reset+"  if the app closes without logs,\n"+
		"     delete your config and restart\n"+
		"\n",
		scheme, conf.APIIP, conf.APIPort, s.BasePath,
	)
}

func InitService() error {
	defer RecoverAndLog()

	InitBaseFoldersAndPaths()
	state := STATE.Load()

	cfgError := loadConfigFromDisk(false)
	if cfgError != nil {
		if state.RequireConfig {
			return cfgError
		}
		_ = loadConfigFromDisk(true)
	}
	conf := CONFIG.Load()

	loadTunnelsFromDisk()
	loadDefaultGateway()
	loadDefaultInterface()

	if conf.CLIConfig != nil {
		DEBUG("cli config loaded")
		wasChanged := false
		if conf.OpenUI {
			conf.OpenUI = false
			wasChanged = true
		}

		if !conf.ConsoleLogOnly {
			conf.ConsoleLogOnly = true
			wasChanged = true
		}
		if wasChanged {
			CONFIG.Store(conf)
		}
	}

	INFO("Starting Tunnels")

	if !conf.ConsoleLogOnly {
		var err error
		LogFile, err = CreateFile(state.LogFileName)
		if err != nil {
			return err
		}
	}

	INFO("Operating specific initializations")
	_ = OSSpecificInit()

	INFO("Checking permissins")
	AdminCheck()

	printInfo()

	if !conf.DisableDNS {
		InitDNSHandler()
		INFO("Starting DNS Proxy")
		doEvent(highPriorityChannel, func() {
			reloadBlockLists(false)
		})
		doEvent(highPriorityChannel, func() {
			reloadWhiteLists(false)
		})
	}

	INFO("Tunnels is ready")
	return nil
}

func LaunchTunnels() {
	defer RecoverAndLog()

	CancelContext, CancelFunc = context.WithCancel(GlobalContext)
	quit = make(chan os.Signal, 10)

	signal.Notify(
		quit,
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGILL,
	)

	newConcurrentSignal("LogProcessor", CancelContext, func() {
		StartLogQueueProcessor()
	})
	conf := CONFIG.Load()

	if conf.CLIConfig == nil {
		newConcurrentSignal("APIServer", CancelContext, func() {
			LaunchAPI()
		})
	}

	if !conf.DisableDNS {
		newConcurrentSignal("UDPDNSHandler", CancelContext, func() {
			StartUDPDNSHandler()
		})
		newConcurrentSignal("BlockListUpdater", CancelContext, func() {
			reloadBlockLists(true)
		})
		newConcurrentSignal("WhiteListUpdater", CancelContext, func() {
			reloadWhiteLists(true)
		})
		newConcurrentSignal("CleanDNSCache", CancelContext, func() {
			CleanDNSCache()
		})
	}

	newConcurrentSignal("LogMapCleaner", CancelContext, func() {
		CleanUniqueLogMap()
	})

	newConcurrentSignal("DefaultGateway", CancelContext, func() {
		GetDefaultGateway()
	})

	newConcurrentSignal("AutoConnect", CancelContext, func() {
		AutoConnect()
	})

mainLoop:
	for {

		select {
		case high := <-highPriorityChannel:
			go high.method()
			continue mainLoop
		case med := <-mediumPriorityChannel:
			go med.method()
			continue mainLoop
		case low := <-lowPriorityChannel:
			go low.method()
			continue mainLoop
		default:
		}

		select {
		case sig := <-quit:
			DEBUG("", "exit signal caught: ", sig.String())
			CancelFunc()
			CleanupOnClose()
			os.Exit(1)

		case Tun := <-tunnelMonitor:
			go handleTunnelDeath(Tun)

		case signal := <-concurrencyMonitor:
			ROUTINE(signal.tag)
			go signal.execute()

		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// reconnectStops tracks running auto-reconnect loops by tunnel tag so a
// user-initiated disconnect (or shutdown) can stop the loop it started.
var reconnectStops sync.Map // tag(string) -> chan struct{}

// stopReconnect signals the reconnect loop for tag (if any) to exit.
func stopReconnect(tag string) {
	if v, ok := reconnectStops.LoadAndDelete(tag); ok {
		close(v.(chan struct{}))
	}
}

// stopAllReconnects stops every running reconnect loop. Used when a
// user-initiated disconnect can't resolve a tag (e.g. the tunnel was
// mid-reconnect and had no live instance in the map).
func stopAllReconnects() {
	reconnectStops.Range(func(k, _ any) bool {
		// Close the value LoadAndDelete returns — not the Range snapshot value,
		// which a concurrent handleTunnelDeath may have replaced under the same
		// tag (closing the snapshot would double-close and leak the new one).
		if actual, ok := reconnectStops.LoadAndDelete(k); ok {
			close(actual.(chan struct{}))
		}
		return true
	})
}

// disconnectTunnelByTag disconnects the currently-connected tunnel with the
// given tag, if any. Used to undo a reconnect that raced a user disconnect.
func disconnectTunnelByTag(tag string) {
	tunnelMapRange(func(t *TUN) bool {
		if m := t.meta.Load(); m != nil && m.Tag == tag {
			_ = Disconnect(t.ID, false)
			return false
		}
		return true
	})
}

// handleTunnelDeath reacts to a tunnel whose WireGuard device died unexpectedly.
// For a kill-switch tunnel it blocks non-tunnel egress immediately (fail
// closed); for an auto-reconnect tunnel it re-establishes the connection with
// backoff, releasing the kill switch once the tunnel is back up. Runs in its own
// goroutine so the main loop is never blocked.
func handleTunnelDeath(t *TUN) {
	defer RecoverAndLog()
	if t == nil || t.GetState() < TUN_Connected {
		// Already being torn down intentionally — nothing to recover.
		return
	}
	meta := t.meta.Load()
	if meta == nil {
		return
	}

	killSwitch := meta.KillSwitch && meta.EnableDefaultRoute
	if killSwitch {
		if !killSwitchSupported() {
			SECURITY("kill switch is ENABLED for ", meta.Tag,
				" but is not enforced on this platform — traffic may leak after a tunnel drop")
		} else if err := engageKillSwitch(meta.Tag); err != nil {
			SECURITY("kill switch: could not block traffic after tunnel drop (", meta.Tag, "): ", err)
		} else {
			INFO("kill switch engaged after tunnel drop: ", meta.Tag)
		}
	}

	cr := t.CR
	// Tear the dead tunnel down cleanly (also moves it out of TUN_Connected).
	Disconnect(t.ID, false)

	if !meta.AutoReconnect || cr == nil {
		// No auto-reconnect: a kill-switch tunnel stays blocked (fail closed)
		// until the user reconnects or disconnects.
		return
	}

	// Register this reconnect loop so a user disconnect / shutdown can stop it.
	// If one is already running for this tag, don't start a second.
	stopCh := make(chan struct{})
	if _, exists := reconnectStops.LoadOrStore(meta.Tag, stopCh); exists {
		return
	}
	defer reconnectStops.CompareAndDelete(meta.Tag, stopCh)

	backoff := 2 * time.Second
	for attempt := 1; ; attempt++ {
		select {
		case <-CancelContext.Done():
			return
		case <-stopCh:
			return
		default:
		}
		code, err := PublicConnect(cr)
		if err == nil && code == 200 {
			// A user disconnect can arrive while PublicConnect is in flight
			// (the tunnel isn't in the map yet, so HTTP_Disconnect can't target
			// it). If the stop was signalled, undo the tunnel we just created.
			select {
			case <-stopCh:
				disconnectTunnelByTag(meta.Tag)
				releaseKillSwitch(meta.Tag)
				return
			default:
			}
			INFO("auto-reconnect: ", meta.Tag, " reconnected")
			if killSwitch {
				// The reinstalled tunnel default route now carries traffic; drop
				// this tunnel's kill-switch need (others keep theirs).
				releaseKillSwitch(meta.Tag)
			}
			return
		}
		INFO("auto-reconnect: ", meta.Tag, " attempt ", attempt, " failed (code ", code, "): ", err)
		select {
		case <-CancelContext.Done():
			return
		case <-stopCh:
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}
