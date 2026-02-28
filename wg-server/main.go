package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	ossig "os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/signal"
)

var peerStore *PeerStore

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

	// Initialise the peer store. The store file lives next to the config file.
	storePath := filepath.Join(filepath.Dir(*configPath), "peers.json")
	var storeErr error
	peerStore, storeErr = NewPeerStore(storePath, cfg.WireGuardSubnet)
	if storeErr != nil {
		ERR("failed to initialise peer store: ", storeErr)
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
		mux.HandleFunc("/v3/wg/assign", handleAssign)
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

// handleAssign handles POST /v3/wg/assign.
// The controller calls this during /v3/session to get (or lazily assign) the
// device's IP on this wg-server. The response is returned to the client so it
// can configure its TUN interface address.
func handleAssign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID  string `json:"DeviceID"`
		PubKeyB64 string `json:"PubKeyB64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" || req.PubKeyB64 == "" {
		http.Error(w, "DeviceID and PubKeyB64 are required", http.StatusBadRequest)
		return
	}

	ip, err := peerStore.GetOrAssign(req.DeviceID, req.PubKeyB64)
	if err != nil {
		ERR("assign IP failed: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		IP string `json:"IP"`
	}{IP: ip})
}
