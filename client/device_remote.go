package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func createServerDeviceFull(cr *ConnectionRequest, serverID string, pubKey string, tag string) (*wgServerConfig, *types.Device, error) {
	serverOID, err := uuid.Parse(serverID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid ServerID: %w", err)
	}
	if tag == "" {
		tag = DefaultTunnelName
	}

	deviceTag := tag
	if tag == DefaultTunnelName {
		deviceTag = fmt.Sprintf("tunnel-%d", time.Now().UnixNano())
	}

	url := cr.Server.GetURL("/client/device/create")
	reqBody := &createDeviceRequest{
		DeviceToken: cr.DeviceToken,
		UID:         cr.UserID,
		Device: &types.Device{
			Tag:          deviceTag,
			WireGuardKey: pubKey,
			ServerID:     serverOID,
		},
	}
	authHeaders := map[string]string{
		"X-Device-Token": cr.DeviceToken,
		"X-UID":          cr.UserID,
	}
	responseBytes, code, reqErr := SendRequestToURL(nil, "POST", url, reqBody, 15000, cr.Server.ValidateCertificate, cr.Server.CertificatePath, authHeaders)
	if reqErr != nil {
		return nil, nil, fmt.Errorf("create device: %w", reqErr)
	}
	if code != 200 {
		var er ErrorResponse
		_ = json.Unmarshal(responseBytes, &er)
		if er.Error != "" {
			return nil, nil, fmt.Errorf("create device: code=%d: %s", code, er.Error)
		}
		return nil, nil, fmt.Errorf("create device: code=%d", code)
	}

	var resp createDeviceControllerResponse
	if err := json.Unmarshal(responseBytes, &resp); err != nil {
		return nil, nil, fmt.Errorf("decode create device response: %w", err)
	}
	if resp.Device == nil {
		return nil, nil, errors.New("controller returned no device")
	}

	return &wgServerConfig{
		WireGuardPubKey:  resp.ServerPubKey,
		WireGuardPort:    resp.ServerPort,
		ServerIP:         resp.ServerIP,
		WireGuardIP:      resp.Device.WireGuardIP,
		WireGuardSubnet:  resp.ServerSubnet,
		WireGuardSubnet6: resp.ServerSubnet6,
		WANCIDR:          resp.WANCIDR,
		EnableFirewall:   false,
	}, resp.Device, nil
}

type createDeviceRequest struct {
	DeviceToken string        `json:"DeviceToken"`
	UID         string        `json:"UID"`
	Device      *types.Device `json:"Device"`
}

type createDeviceControllerResponse struct {
	Device        *types.Device `json:"Device"`
	ServerPubKey  string        `json:"ServerPubKey"`
	ServerPort    string        `json:"ServerPort"`
	ServerIP      string        `json:"ServerIP"`
	ServerSubnet  string        `json:"ServerSubnet"`
	ServerSubnet6 string        `json:"ServerSubnet6"`
	WANCIDR       string        `json:"WANCIDR"`
}

// CreateDeviceResult is the one-shot WireGuard config returned when creating
// a remote device from this machine.
type CreateDeviceResult struct {
	WGConfig string        `json:"WGConfig"`
	Device   *types.Device `json:"Device"`
}

func CreateDeviceWithKeys(form *CreateDeviceWithKeysForm) (any, int) {
	if err := authorizeControlServer(form.Server); err != nil {
		return &ErrorResponse{Error: err.Error()}, 403
	}

	privKey, err := generateWGPrivKey()
	if err != nil {
		return &ErrorResponse{Error: "failed to generate WireGuard private key: " + err.Error()}, 500
	}

	pubKey, err := deriveWGPubKey(privKey)
	if err != nil {
		return &ErrorResponse{Error: "failed to derive WireGuard public key: " + err.Error()}, 500
	}

	serverOID, err := uuid.Parse(form.ServerID)
	if err != nil {
		return &ErrorResponse{Error: "invalid ServerID: " + err.Error()}, 400
	}

	url := form.Server.GetURL("/client/device/create")
	reqBody := &createDeviceRequest{
		DeviceToken: form.DeviceToken,
		UID:         form.UID,
		Device: &types.Device{
			Tag:          form.Tag,
			WireGuardKey: pubKey,
			ServerID:     serverOID,
		},
	}

	authHeaders := map[string]string{
		"X-Device-Token": form.DeviceToken,
		"X-UID":          form.UID,
	}
	responseBytes, code, reqErr := SendRequestToURL(nil, "POST", url, reqBody, 15000, form.Server.ValidateCertificate, form.Server.CertificatePath, authHeaders)
	if reqErr != nil {
		return &ErrorResponse{Error: "controller request failed: " + reqErr.Error()}, 500
	}
	if code != 200 {
		var er ErrorResponse
		_ = json.Unmarshal(responseBytes, &er)
		if er.Error == "" {
			er.Error = fmt.Sprintf("controller returned status %d", code)
		}
		return &er, code
	}

	var resp createDeviceControllerResponse
	if err := json.Unmarshal(responseBytes, &resp); err != nil {
		return &ErrorResponse{Error: "failed to parse controller response: " + err.Error()}, 500
	}

	if resp.Device == nil {
		return &ErrorResponse{Error: "controller returned no device"}, 500
	}

	wgConfig := fmt.Sprintf(
		"[Interface]\nPrivateKey = %s\nAddress = %s/32\nDNS = 1.1.1.1\n\n[Peer]\nPublicKey = %s\nEndpoint = %s:%s\nAllowedIPs = 0.0.0.0/0\nPersistentKeepalive = 25\n",
		privKey,
		resp.Device.WireGuardIP,
		resp.ServerPubKey,
		resp.ServerIP,
		resp.ServerPort,
	)

	return &CreateDeviceResult{
		WGConfig: wgConfig,
		Device:   resp.Device,
	}, 200
}
