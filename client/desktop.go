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

// ErrTunnelConnected is returned when a tunnel cannot be modified while up.
var ErrTunnelConnected = errors.New("tunnel is connected")

var uiLogHandler atomic.Value // func(string)

// SetUILogHandler registers an in-process log sink. The callback must not
// block: the log processor calls it on every line. Pass nil to clear.
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

// SnapshotLogs returns a copy of the in-memory log buffer.
func SnapshotLogs() []string {
	PollLogMu.Lock()
	defer PollLogMu.Unlock()
	out := make([]string, len(PollLogBuf))
	copy(out, PollLogBuf)
	return out
}

// GetUsers returns every saved account on disk.
func GetUsers() ([]*User, error) {
	return getUsers()
}

// SaveUser writes the account file and activates its workspace.
func SaveUser(u *User) error {
	return saveUser(u)
}

// DeleteUser removes a saved account by its folder hash.
func DeleteUser(hash string) error {
	return delUser(hash)
}

// ActivateAccount switches the on-disk workspace to userID's account.
func ActivateAccount(userID string) error {
	return activateAccountByUserID(userID)
}

// GetLocalDevices lists devices created on this machine for userID's account.
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

// ControllerRequest posts JSON to a configured control server. When
// deviceToken is set, X-Device-Token / X-UID are sent.
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

// ControllerError extracts an Error field from a controller JSON body.
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

// GetFullState snapshots config, tunnels, and runtime state.
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
