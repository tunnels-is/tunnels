package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// seedNetworks generates all /22 subnets from 10.0.0.0/8 on first startup.
// 256 * 64 = 16,384 networks total.
func seedNetworks() {
	count, err := DB_CountNetworks()
	if err != nil {
		ERR(err)
		return
	}
	if count > 0 {
		return
	}

	INFO("Seeding networks: generating /22 subnets from 10.0.0.0/8")
	networks := make([]*Network, 0, 16384)
	now := time.Now()
	for x := 0; x < 256; x++ {
		for y := 0; y < 256; y += 4 {
			networks = append(networks, &Network{
				ID:        primitive.NewObjectID(),
				CIDR:      fmt.Sprintf("10.%d.%d.0/22", x, y),
				CreatedAt: now,
			})
		}
	}

	if err := DB_CreateNetworksBatch(networks); err != nil {
		ERR(err)
		return
	}
	INFO(fmt.Sprintf("Seeded %d networks", len(networks)))
}

func API_NetworkList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_NETWORKS)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		senderr(w, 500, err.Error())
		return
	}
	if !user.IsAdmin && !user.IsManager {
		senderr(w, 401, "Unauthorized")
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
	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		senderr(w, 500, err.Error())
		return
	}
	if !user.IsAdmin && !user.IsManager {
		senderr(w, 401, "Unauthorized")
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
	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		senderr(w, 500, err.Error())
		return
	}
	if !user.IsAdmin && !user.IsManager {
		senderr(w, 401, "Unauthorized")
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
	// Redact sensitive fields before sending
	for _, cfg := range configs {
		cfg.APIKey = ""
		cfg.WireGuardPrivKey = ""
	}
	sendObject(w, configs)
}
