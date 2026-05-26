package client

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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

	if conf.AutoDownloadUpdate {
		didUpdate := doStartupUpdate()
		if didUpdate {
			os.Exit(1)
		}
	}

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

	newConcurrentSignal("Updater", CancelContext, func() {
		doUpdate()
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
			Tun.needsReconnect.Store(true)

		case signal := <-concurrencyMonitor:
			ROUTINE(signal.tag)
			go signal.execute()

		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}
