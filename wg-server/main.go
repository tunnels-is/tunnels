package main

import (
	"flag"
	"fmt"
	"os"
	ossig "os/signal"
	"sync/atomic"
	"syscall"
)

var (
	peerStore    *PeerStore
	activeConfig atomic.Pointer[Config]
)

func main() {
	apiKey := flag.String("key", "", "per-server API key (from POST /ui/wg/server-config)")
	controllerIP := flag.String("ip", "", "controller IP address (e.g. 74.63.223.157)")
	showVersion := flag.Bool("version", false, "show version and exit")
	jsonLogs := flag.Bool("json", false, "enable JSON-format logging")
	sourceInfo := flag.Bool("source", false, "include source file/line in log output")
	silent := flag.Bool("silent", false, "disable all logging")
	logLevel := flag.String("logLevel", "debug", "log level: debug, info, warn, error")
	insecure := flag.Bool("insecure", false, "skip TLS certificate verification")
	flag.Parse()

	initLogging(*silent, *jsonLogs, *sourceInfo, *logLevel)

	if *showVersion {
		fmt.Println("wg-server v0.1.0")
		os.Exit(0)
	}

	if *apiKey == "" {
		ERR("--key is required (per-server API key from the controller)")
		os.Exit(1)
	}
	if *controllerIP == "" {
		ERR("--ip is required (controller IP address)")
		os.Exit(1)
	}

	controllerURL := "https://" + *controllerIP

	INFO("fetching config from controller at ", controllerURL)
	cfg, err := FetchConfig(controllerURL, *apiKey, *insecure)
	if err != nil {
		ERR("failed to fetch config from controller: ", err)
		os.Exit(1)
	}
	cfg.LogJSON = *jsonLogs
	cfg.Silent = *silent
	cfg.LogLevel = *logLevel

	INFO("config fetched from controller, serverID=", cfg.ServerID,
		" subnet=", cfg.WireGuardSubnet, " iface=", cfg.WireGuardIface)

	if err := setupWireGuard(cfg); err != nil {
		ERR("wireguard setup failed: ", err)
		os.Exit(1)
	}

	if err := setupNet(cfg); err != nil {
		ERR("network setup failed: ", err)
		os.Exit(1)
	}

	var storeErr error
	peerStore, storeErr = NewPeerStore("peers.json", cfg.WireGuardSubnet)
	if storeErr != nil {
		ERR("failed to initialise peer store: ", storeErr)
		os.Exit(1)
	}

	activeConfig.Store(cfg)
	initSyncClient(cfg)

	INFO("wg-server started")

	sigCh := make(chan os.Signal, 1)
	ossig.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	INFO("shutting down...")

	if wgDevice != nil {
		wgDevice.Close()
	}
	cleanupNet(cfg)
	INFO("shutdown complete")
}
