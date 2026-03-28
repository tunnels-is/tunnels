package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func seedNetworks() (*Network, error) {
	count, err := DB_CountNetworks()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		existing, err := DB_GetNetworks(1, 0)
		if err != nil {
			return nil, err
		}
		if len(existing) == 0 {
			return nil, fmt.Errorf("network count > 0 but no networks found")
		}
		return existing[0], nil
	}

	INFO("Seeding networks: generating /22 subnets from 10.0.0.0/8")
	networks := make([]*Network, 0, 16384)
	now := time.Now()
	for x := 0; x < 256; x++ {
		for y := 0; y < 256; y += 4 {
			networks = append(networks, &Network{
				ID:        uuid.New(),
				CIDR:      fmt.Sprintf("10.%d.%d.0/22", x, y),
				CreatedAt: now,
			})
		}
	}

	if err := DB_CreateNetworksBatch(networks); err != nil {
		return nil, err
	}
	INFO(fmt.Sprintf("Seeded %d networks", len(networks)))
	return networks[0], nil
}

func API_NetworkList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_NETWORKS)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if F.Limit == 0 {
		F.Limit = 200
	}
	networks, err := DB_GetNetworks(int64(F.Limit), int64(F.Offset))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if len(networks) == 0 {
		w.WriteHeader(204)
		return
	}
	sendObject(w, networks)
}

func API_NetworkUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_UPDATE_NETWORK)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if F.Network == nil {
		senderr(w, 400, "Missing network")
		return
	}
	if err := DB_UpdateNetwork(F.Network); err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	w.WriteHeader(200)
}

func API_WGServerConfigList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_WG_SERVER_CONFIGS)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	configs, err := DB_ListWGServerConfigs()
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if len(configs) == 0 {
		w.WriteHeader(204)
		return
	}

	for _, cfg := range configs {
		cfg.APIKey = ""
		cfg.WireGuardPrivKey = ""
	}
	sendObject(w, configs)
}
