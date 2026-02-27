package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	ossig "os/signal"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/signal"
)

func main() {
	genConfig  := flag.Bool("config", false, "generate a default config file and exit")
	configPath := flag.String("configPath", "./wg-config.json", "path to config file (.json, .yaml, .yml)")
	showVersion := flag.Bool("version", false, "show version and exit")
	jsonLogs   := flag.Bool("json", false, "enable JSON-format logging")
	sourceInfo := flag.Bool("source", false, "include source file/line in log output")
	silent     := flag.Bool("silent", false, "disable all logging")
	logLevel   := flag.String("logLevel", "debug", "log level: debug, info, warn, error")
	flag.Parse()

	initLogging(*silent, *jsonLogs, *sourceInfo, *logLevel)

	if *showVersion {
		fmt.Println("wg-server v0.1.0")
		os.Exit(0)
	}

	if *genConfig {
		c := defaultConfig()
		c.WireGuardPrivKey = generatePrivKey()
		if err := SaveConfig(*configPath, c); err != nil {
			ERR("failed to write config: ", err)
			os.Exit(1)
		}
		pubKey, _ := derivePubKey(c.WireGuardPrivKey)
		INFO("config written to ", *configPath)
		INFO("server public key: ", pubKey)
		INFO("edit the config to set ControllerURL, AdminAPIKey, and InternetIface")
		os.Exit(0)
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		ERR("failed to load config from ", *configPath, ": ", err)
		os.Exit(1)
	}

	if cfg.WireGuardPrivKey == "" {
		ERR("WireGuardPrivKey is required; run with -config to generate a new config")
		os.Exit(1)
	}
	if cfg.ControllerURL == "" {
		ERR("ControllerURL is required in config")
		os.Exit(1)
	}
	if cfg.AdminAPIKey == "" {
		ERR("AdminAPIKey is required in config")
		os.Exit(1)
	}
	if cfg.InternetIface == "" {
		ERR("InternetIface is required in config (e.g. eth0)")
		os.Exit(1)
	}

	if err := setupWireGuard(cfg); err != nil {
		ERR("wireguard setup failed: ", err)
		os.Exit(1)
	}

	if err := setupNet(cfg); err != nil {
		ERR("network setup failed: ", err)
		os.Exit(1)
	}

	initSyncClient(cfg)

	if cfg.SyncListenAddr != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("/v3/wg/sync", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			go SyncPeers()
			w.WriteHeader(http.StatusOK)
		})
		go func() {
			if listenErr := http.ListenAndServe(cfg.SyncListenAddr, mux); listenErr != nil {
				ERR("sync listener error: ", listenErr)
			}
		}()
		INFO("sync listener started on ", cfg.SyncListenAddr)
	}

	ctx, cancel := context.WithCancel(context.Background())

	syncInterval := time.Duration(cfg.SyncIntervalSecs) * time.Second
	signal.NewSignal(
		"wg-sync",
		ctx,
		cancel,
		syncInterval,
		func(s string) { LOG(s) },
		SyncPeers,
	)

	INFO("wg-server started")

	sigCh := make(chan os.Signal, 1)
	ossig.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	INFO("shutting down...")
	cancel()

	if wgDevice != nil {
		wgDevice.Close()
	}
	cleanupNet(cfg)
	INFO("shutdown complete")
}
