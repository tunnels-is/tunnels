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
		"\n",
		scheme, conf.APIIP, conf.APIPort, s.BasePath,
	)
}

func InitService() error {
	defer RecoverAndLog()

	if err := InitBaseFoldersAndPaths(); err != nil {
		return err
	}
	state := STATE.Load()

	cfgError := loadConfigFromDisk(false)
	if cfgError != nil {
		if state.RequireConfig {
			return cfgError
		}
		_ = loadConfigFromDisk(true)
	}
	conf := CONFIG.Load()

	if conf.CLIConfig != nil && conf.CLIConfig.UserID != "" {
		if err := activateAccountByUserID(conf.CLIConfig.UserID); err != nil {
			ERROR("unable to activate CLI account workspace:", err)
		}
	}

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

		LogFile, err = os.OpenFile(state.LogFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
	}

	INFO("Operating specific initializations")
	_ = OSSpecificInit()

	INFO("Checking permissins")
	AdminCheck()

	printInfo()

	if err := applyConfiguredKillSwitch(); err != nil {
		SECURITY("kill switch: failed to apply on startup: ", err)
	}

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

var reconnectStops sync.Map

func stopReconnect(tag string) {
	if v, ok := reconnectStops.LoadAndDelete(tag); ok {
		close(v.(chan struct{}))
	}
}

func stopAllReconnects() {
	reconnectStops.Range(func(k, _ any) bool {
		if actual, ok := reconnectStops.LoadAndDelete(k); ok {
			close(actual.(chan struct{}))
		}
		return true
	})
}

func disconnectTunnelByTag(tag string) {
	tunnelMapRange(func(t *TUN) bool {
		if m := t.meta.Load(); m != nil && m.Tag == tag {
			_ = Disconnect(t.ID, false)
			return false
		}
		return true
	})
}

func handleTunnelDeath(t *TUN) {
	defer RecoverAndLog()
	if t == nil || t.GetState() < TUN_Connected {
		return
	}
	meta := t.meta.Load()
	if meta == nil {
		return
	}

	cr := t.CR
	keepPath := meta.AutoReconnect && cr != nil && t.osTUN != nil && t.osTUN.CanReuse()
	if !keepPath {
		Disconnect(t.ID, false)
		_ = applyConfiguredKillSwitch()
		if !meta.AutoReconnect || cr == nil {
			return
		}
	} else {
		t.SetState(TUN_Connecting)
		_ = applyConfiguredKillSwitch()
	}

	stopCh := make(chan struct{})
	if _, exists := reconnectStops.LoadOrStore(meta.Tag, stopCh); exists {
		return
	}
	defer reconnectStops.CompareAndDelete(meta.Tag, stopCh)

	backoff := 2 * time.Second
	for attempt := 1; ; attempt++ {
		select {
		case <-CancelContext.Done():
			if keepPath {
				Disconnect(t.ID, false)
			}
			return
		case <-stopCh:
			if keepPath {
				Disconnect(t.ID, false)
			}
			return
		default:
		}
		code, err := PublicConnect(cr)
		if err == nil && code == 200 {

			select {
			case <-stopCh:
				disconnectTunnelByTag(meta.Tag)
				return
			default:
			}
			INFO("auto-reconnect: ", meta.Tag, " reconnected")
			return
		}
		INFO("auto-reconnect: ", meta.Tag, " attempt ", attempt, " failed (code ", code, "): ", err)
		select {
		case <-CancelContext.Done():
			if keepPath {
				Disconnect(t.ID, false)
			}
			return
		case <-stopCh:
			if keepPath {
				Disconnect(t.ID, false)
			}
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}
