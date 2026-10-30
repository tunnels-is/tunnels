package client

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/tunnels-is/tunnels/version"
)

var ErrTunnelConnected = errors.New("tunnel is connected")

var uiLogHandler atomic.Value

// SetUILogHandler ...
func SetUILogHandler(fn func(string)) {
	if fn == nil {
		uiLogHandler.Store(func(string) {})
		return
	}
	uiLogHandler.Store(fn)
}

func emitUILog(line string) {
	v := uiLogHandler.Load()
	if v == nil {
		return
	}
	fn, ok := v.(func(string))
	if !ok || fn == nil {
		return
	}
	fn(line)
}

// SnapshotLogs ...
func SnapshotLogs() []string {
	PollLogMu.Lock()
	defer PollLogMu.Unlock()
	out := make([]string, len(PollLogBuf))
	copy(out, PollLogBuf)
	return out
}

// GetUsers ...
func GetUsers() ([]*User, error) {
	return getUsers()
}

// SaveUser ...
func SaveUser(u *User) error {
	return saveUser(u)
}

// DeleteUser ...
func DeleteUser(hash string) error {
	return delUser(hash)
}

// ActivateAccount ...
func ActivateAccount(userID string) error {
	return activateAccountByUserID(userID)
}

// GetLocalDevices ...
func GetLocalDevices(userID string) ([]LocalDeviceInfo, error) {
	if userID != "" {
		if err := activateAccountByUserID(userID); err != nil {
			return nil, err
		}
	}
	list, err := listLocalDeviceInfo()
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []LocalDeviceInfo{}
	}
	return list, nil
}

// ControllerRequest ...
func ControllerRequest(server *ControlServer, path string, body any, uid, deviceToken string) ([]byte, int, error) {
	if err := authorizeControlServer(server); err != nil {
		return nil, 403, err
	}

	var extra map[string]string
	if deviceToken != "" {
		extra = map[string]string{
			"X-Device-Token": deviceToken,
			"X-UID":          uid,
		}
	}

	url := server.GetURL(path)
	resp, code, err := SendRequestToURL(
		nil,
		"POST",
		url,
		body,
		20000,
		server.ValidateCertificate,
		server.CertificatePath,
		extra,
	)
	if err != nil {
		return resp, code, err
	}
	if code == 0 {
		return resp, 500, errors.New("unable to contact controller")
	}
	return resp, code, nil
}

// ControllerError ...
func ControllerError(body []byte, fallback string) string {
	if len(body) == 0 {
		if fallback != "" {
			return fallback
		}
		return "unknown error"
	}
	var er ErrorResponse
	if err := json.Unmarshal(body, &er); err == nil && er.Error != "" {
		return er.Error
	}
	s := strings.TrimSpace(string(body))
	if s != "" && s[0] != '{' && s[0] != '[' {
		return s
	}
	if fallback != "" {
		return fallback
	}
	return s
}

func getSystemTimezone() string {
	if tz := os.Getenv("TZ"); tz != "" && tz != ":/etc/localtime" {
		return strings.TrimPrefix(tz, ":")
	}

	if b, err := os.ReadFile("/etc/timezone"); err == nil {
		if name := strings.TrimSpace(string(b)); name != "" {
			return name
		}
	}
	if link, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(link, "zoneinfo/"); i != -1 {
			return link[i+len("zoneinfo/"):]
		}
	}

	if resolved, err := filepath.EvalSymlinks("/etc/localtime"); err == nil {
		if i := strings.Index(resolved, "zoneinfo/"); i != -1 {
			return resolved[i+len("zoneinfo/"):]
		}
	}
	return ""
}

// GetFullState ...
func GetFullState() (s *StateResponse) {
	defer RecoverAndLog()
	state := STATE.Load()
	s = new(StateResponse)
	s.Version = version.Version
	s.APIVersion = version.ApiVersion
	s.Timezone = getSystemTimezone()
	s.Config = CONFIG.Load()
	s.State = state

	tunnelMetaMapRange(func(tun *TunnelMeta) bool {
		s.Tunnels = append(s.Tunnels, tun)
		return true
	})

	tunnelMapRange(func(tun *TUN) bool {
		s.ActiveTunnels = append(s.ActiveTunnels, tun)
		return true
	})
	return
}
