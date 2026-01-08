package client

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func printInfo() {
	log.Println("")
	log.Println("")
	log.Println("==============================================================")
	log.Println("======================= TUNNELS.IS ===========================")
	log.Println("==============================================================")
	log.Println("NOTE: If the app closes without any logs/errors you might need to delete your config and try again")
	log.Println("")
}

func printInfo2() {
	conf := CONFIG.Load()
	s := STATE.Load()
	log.Println("")
	log.Println("=======================================================================")
	log.Println("======================= HELPFUL INFORMATION ===========================")
	log.Println("=======================================================================")
	log.Println("")
	log.Printf("APP: https://%s:%s\n", conf.APIIP, conf.APIPort)
	log.Println("")
	log.Println("BASE PATH:", s.BasePath)
	log.Println("")
	log.Println("- Tunnels request network admin permissions to run.")
	log.Println("- Remember to configure your DNS servers if you want to prevent DNS leaks.")
	log.Println("- Remember to turn all logging off if you are concerned about privacy.")
	log.Println("- There is a --basePath flag that can let you reconfigure the base directory for logs and configs, the default location is where you placed tunnels.")
	log.Println("")
	log.Println("=======================================================================")
	log.Println("=======================================================================")
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
		// CLI mode always forces console log only
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
	printInfo2()

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

	if conf.OpenUI {
		newConcurrentSignal("OpenUI", CancelContext, func() {
			popUI()
		})
	}

	newConcurrentSignal("LogMapCleaner", CancelContext, func() {
		CleanUniqueLogMap()
	})

	newConcurrentSignal("CleanPortAllocs", CancelContext, func() {
		CleanPortsForAllConnections()
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

		case Tun := <-interfaceMonitor:
			go Tun.ReadFromTunnelInterface()
		case Tun := <-tunnelMonitor:
			go Tun.ReadFromServeTunnel()

		case signal := <-concurrencyMonitor:
			DEBUG(signal.tag)
			go signal.execute()

		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}
