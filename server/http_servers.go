package main

import (
	"fmt"
	"log/slog"
	mrand "math/rand/v2"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func handleAdminServerGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(getServerRequest)
	err := decodeBody(r, F)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	server, err := findServerByID(F.ServerID)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment", slog.Any("error", err))
		return
	}
	if server == nil {
		sendError(w, 404, "server not found")
		return
	}

	attachWANs(server)
	sendObject(w, server)
}

func handleAdminServerList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(listServersRequest)
	err := decodeBody(r, F)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	servers, err := findAllServers(100, int64(F.StartIndex))
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	attachWANs(servers...)
	sendObject(w, servers)
}

func handleAdminServerDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(deleteServerRequest)
	err := decodeBody(r, F)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = deleteServerByID(F.ServerID)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func sanitizeServerForClient(s *types.Server) *types.Server {
	c := *s
	c.APIKey = ""
	return &c
}

func findServersForUser(user *User, offset int64) ([]*types.Server, error) {
	servers, err := findServersWithoutGroups(10000, offset)
	if err != nil {
		return nil, err
	}
	if len(user.Groups) > 0 {
		grouped, err := findServersByGroups(user.Groups, 10000, offset)
		if err != nil {
			return nil, err
		}
		servers = append(servers, grouped...)
	}
	return servers, nil
}

func handleClientServersByCountry(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(serversByCountryRequest)
	err := decodeBody(r, F)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if F.Country == "" {
		sendError(w, 400, "Country is required")
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		sendError(w, 401, "Unauthorized")
		return
	}

	all, err := findServersForUser(user, 0)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	matched := make([]*types.Server, 0)
	for _, s := range all {
		if strings.EqualFold(s.Country, F.Country) {
			matched = append(matched, s)
		}
	}

	mrand.Shuffle(len(matched), func(i, j int) {
		matched[i], matched[j] = matched[j], matched[i]
	})
	if len(matched) > 10 {
		matched = matched[:10]
	}

	for i, s := range matched {
		matched[i] = sanitizeServerForClient(s)
	}

	attachWANs(matched...)
	sendObject(w, matched)
}

func handleClientServers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(listServersRequest)
	err := decodeBody(r, F)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		sendError(w, 401, "Unauthorized")
		return
	}

	servers := make([]*types.Server, 0)
	pservers, err := findServersWithoutGroups(100, int64(F.StartIndex))
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}
	servers = append(servers, pservers...)

	if len(user.Groups) > 0 {
		puservers, err := findServersByGroups(user.Groups, 100, int64(F.StartIndex))
		if err != nil {
			sendError(w, 500, "Unknown error, please try again in a moment")
			return
		}
		servers = append(servers, puservers...)
	}

	for i, s := range servers {
		servers[i] = sanitizeServerForClient(s)
	}

	attachWANs(servers...)
	sendObject(w, servers)
}

func applyWGDefaults(s *types.Server) {
	if s.APIKey == "" {
		s.APIKey = uuid.NewString()
	}
	if s.WireGuardPort == 0 {
		s.WireGuardPort = 51820
	}
	if s.WireGuardMeshPort == 0 {
		s.WireGuardMeshPort = s.WireGuardPort + 1
	}
	if s.WireGuardIface == "" {
		s.WireGuardIface = "wg0"
	}
}

func validateServerWGFields(s *types.Server) error {
	if err := validateCIDR(s.WireGuardSubnet); err != nil {
		return fmt.Errorf("invalid WireGuardSubnet: %w", err)
	}
	if err := validateCIDR(s.WireGuardSubnet6); err != nil {
		return fmt.Errorf("invalid WireGuardSubnet6: %w", err)
	}
	if s.WireGuardIface != "" && !types.ValidIfaceName(s.WireGuardIface) {
		return fmt.Errorf("invalid WireGuardIface %q", s.WireGuardIface)
	}
	if s.InternetIface != "" && !types.ValidIfaceName(s.InternetIface) {
		return fmt.Errorf("invalid InternetIface %q", s.InternetIface)
	}
	return nil
}

func handleAdminServerUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	F := new(updateServerRequest)
	err := decodeBody(r, F)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if F.Server == nil {
		sendError(w, 400, "Server is required")
		return
	}
	if err := validateServerWGFields(F.Server); err != nil {
		sendError(w, 400, err.Error())
		return
	}
	applyWGDefaults(F.Server)
	if err := validateServerMesh(F.Server); err != nil {
		sendError(w, 400, err.Error())
		return
	}

	_, err = updateServer(F.Server)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminServerCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(createServerRequest)
	err := decodeBody(r, F)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if F.Server == nil {
		sendError(w, 400, "Server is required")
		return
	}
	if err := validateServerWGFields(F.Server); err != nil {
		sendError(w, 400, err.Error())
		return
	}
	applyWGDefaults(F.Server)
	F.Server.ID = uuid.New()
	if err := validateServerMesh(F.Server); err != nil {
		sendError(w, 400, err.Error())
		return
	}

	F.Server.Groups = make([]uuid.UUID, 0)
	err = createServer(F.Server)
	if err != nil {
		sendError(w, 500, "Uknown error, please try again in a moment", slog.Any("err", err))
		return
	}

	sendObject(w, F.Server)
}

func handleClientServerGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(getServerRequest)
	err := decodeBody(r, F)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	server, err := findServerByID(F.ServerID)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment", slog.Any("error", err))
		return
	}
	if server == nil {
		sendError(w, 404, "Server not found")
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		sendError(w, 401, "Unauthorized")
		return
	}

	if !hasSharedOrNoGroup(user.Groups, server.Groups) {
		sendError(w, 401, "unauthorized")
		return
	}

	sc := sanitizeServerForClient(server)
	attachWANs(sc)
	sendObject(w, sc)
}
