package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	ossig "os/signal"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/signal"
)

var peerStore *PeerStore

func main() {
	apiKey     := flag.String("key", "", "per-server API key (from POST /v3/wg/server-config)")
	controllerIP := flag.String("ip", "", "controller IP address (e.g. 74.63.223.157)")
	showVersion := flag.Bool("version", false, "show version and exit")
	jsonLogs   := flag.Bool("json", false, "enable JSON-format logging")
	sourceInfo := flag.Bool("source", false, "include source file/line in log output")
	silent     := flag.Bool("silent", false, "disable all logging")
	logLevel   := flag.String("logLevel", "debug", "log level: debug, info, warn, error")
	insecure   := flag.Bool("insecure", false, "skip TLS certificate verification")
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

	// Peer store lives in the current working directory.
	var storeErr error
	peerStore, storeErr = NewPeerStore("peers.json", cfg.WireGuardSubnet)
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
