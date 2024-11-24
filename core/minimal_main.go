package core

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"
)

func InitMinimal() error {
	defer RecoverAndLogToFile()

	INFO("loader", "Starting Tunnels")

	initializeMinimalGlobalVariables()

	_ = OSSpecificInit()
	AdminCheck()
	InitPaths()
	CreateBaseFolder()
	go StartLogQueueProcessor(routineMonitor)
	LoadConfig()
	if C.InfoLogging {
		printInfo()
	}

	if GLOBAL_STATE.C == nil {
		ERROR("", "Global state could not be set.. possible config issue")
		return errors.New("unable to create global state.. possible config error")
	}

	err := LoadCA()
	if err != nil {
		INFO("", "Could not load root CA")
		return errors.New("could not load root CA")
	}

	INFO("Tunnels is ready")
	return nil
}

func LaunchMinimal() {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	CancelContext, CancelFunc = context.WithCancel(GlobalContext)
	quit = make(chan os.Signal, 10)

	signal.Notify(
		quit,
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGILL,
	)

	// routineMonitor <- 1
	routineMonitor <- 3
	routineMonitor <- 4
	routineMonitor <- 5
	routineMonitor <- 6

	for {
		select {
		case sig := <-quit:
			DEBUG("", "exit signal caught: ", sig)
			CancelFunc()
			CleanupOnClose()
			os.Exit(1)

		case IF := <-interfaceMonitor:
			go IF.ReadFromTunnelInterface()
		case Tun := <-tunnelMonitor:
			go Tun.ReadFromServeTunnel()

		case ID := <-routineMonitor:
			if ID == 1 {
				go StartLogQueueProcessor(routineMonitor)
			} else if ID == 3 {
				go PingConnections(routineMonitor)
			} else if ID == 4 {
				go GetDefaultGateway(routineMonitor)
			} else if ID == 5 {
				go AutoConnect(routineMonitor)
			} else if ID == 6 {
				go CleanPortsForAllConnections(routineMonitor)
			}
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}
