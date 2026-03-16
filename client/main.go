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

	fmt.Printf(
		"\n"+
			"\033[1;34m  ________                         ______       \n"+
			"  ___  __/___  _______________________  /_______\n"+
			"  __  /  _  / / /_  __ \\_  __ \\  _ \\_  /__  ___/\n"+
			"  _  /   / /_/ /_  / / /  / / /  __/  / _(__  ) \n"+
			"  /_/    \\__,_/ /_/ /_//_/ /_/\\___//_/  /____/\033[0m  \n"+
			"                                       tunnels.is\n"+
			"\n"+
			"\033[34m  ──────────────────────────────────────────────────────────────\033[0m\n"+
			"\n"+
			"  \033[1m🌐\033[0m  APP        %s://%s:%s\n"+
			"  \033[1m📁\033[0m  BASE PATH  %s\n"+
			"\n"+
			"\033[34m  ──────────────────────────────────────────────────────────────\033[0m\n"+
			"\n"+
			"  ·  requires network admin permissions\n"+
			"  ·  configure dns servers to prevent leaks\n"+
			"  ·  turn logging off for improved privacy\n"+
			"  ·  use --basePath to change the config directory\n"+
			"\n"+
			"  \033[33m⚠\033[0m  if the app closes without logs, delete your config and retry\n"+
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

	newConcurrentSignal("Pinger", CancelContext, func() {
		PingConnections()
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
