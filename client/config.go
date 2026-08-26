package client

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tunnels-is/tunnels/types"
	"gopkg.in/yaml.v3"
)

var configFileMu sync.Mutex

func writeConfigToDisk() (err error) {
	defer RecoverAndLog()
	configFileMu.Lock()
	defer configFileMu.Unlock()
	conf := CONFIG.Load()
	s := STATE.Load()

	var cb []byte
	ext := strings.ToLower(filepath.Ext(s.ConfigFileName))

	switch ext {
	case ".yaml", ".yml":
		cb, err = yaml.Marshal(conf)
		if err != nil {
			ERROR("Unable to marshal config into YAML bytes: ", err)
			return err
		}
	case ".json", ".conf", "":
		cb, err = json.MarshalIndent(conf, "", "    ")
		if err != nil {
			ERROR("Unable to marshal config into JSON bytes: ", err)
			return err
		}
	default:
		err = fmt.Errorf("unsupported config file format: %s (supported: .json, .yaml, .yml, .conf)", ext)
		ERROR(err)
		return err
	}

	err = writeFileWithBackup(s.ConfigFileName, cb)
	if err != nil {
		ERROR("Unable to write config to disk: ", err)
		return err
	}

	return
}

func ReadConfigFileFromDisk() (err error) {
	state := STATE.Load()
	config, err := os.ReadFile(state.ConfigFileName)
	if err != nil {
		return err
	}

	Conf := new(configV2)
	ext := strings.ToLower(filepath.Ext(state.ConfigFileName))

	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(config, Conf)
		if err != nil {
			ERROR("Unable to unmarshal YAML config file: ", err)
			return
		}
	case ".json", ".conf", "":
		err = json.Unmarshal(config, Conf)
		if err != nil {
			ERROR("Unable to unmarshal JSON config file: ", err)
			return
		}
	default:
		err = fmt.Errorf("unsupported config file format: %s (supported: .json, .yaml, .yml, .conf)", ext)
		ERROR(err)
		return
	}

	applyMissingKillSwitchDefaults(config, Conf)

	if len(Conf.ControlServers) < 1 {
		Conf.ControlServers = append(Conf.ControlServers, &ControlServer{
			ID:                  "tunnels",
			Host:                "api.tunnels.is",
			Port:                "443",
			CertificatePath:     "",
			ValidateCertificate: true,
		})
		err = writeConfigToDisk()
		if err != nil {
			ERROR("unable to add api.tunnels.is to default config")
		}
	}

	CONFIG.Store(Conf)

	return
}

func loadConfigFromDisk(newConfig bool) error {
	defer RecoverAndLog()
	DEBUG("Loading configurations from file")
	if !newConfig {
		return ReadConfigFileFromDisk()
	} else {
		err := ReadConfigFileFromDisk()
		if err == nil {
			return nil
		}
	}

	DEBUG("Generating a new default config")
	CONFIG.Store(DefaultConfig())
	return writeConfigToDisk()
}

func DefaultConfig() *configV2 {
	conf := &configV2{
		DebugLogging:      false,
		InfoLogging:       false,
		ErrorLogging:      false,
		ConnectionTracer:  false,
		DNSServerIP:       "127.0.0.1",
		DNSServerPort:     "53",
		DNS1Default:       "1.1.1.1",
		DNS2Default:       "8.8.8.8",
		LogBlockedDomains: false,
		LogAllDomains:     false,
		BandwidthGraphs:   true,
		DNSstats:          false,
		DNSHTTPSAutomatic: true,
		DNSBlockLists:     GetDefaultBlockLists(),
		DNSWhiteLists:     GetDefaultWhiteLists(),
		KillSwitchIPv4:    false,
		KillSwitchIPv6:    true,
	}
	conf.ControlServers = append(conf.ControlServers, &ControlServer{
		ID:                  "tunnels",
		Host:                "api.tunnels.is",
		Port:                "443",
		CertificatePath:     "",
		ValidateCertificate: true,
	})
	return conf
}

func writeTunnelsToDisk(tag string) (outErr error) {
	s := STATE.Load()
	if s.TunnelsPath == "" {
		DEBUG("writeTunnelsToDisk: no tunnels path (no active account), skip")
		return nil
	}
	TunnelMetaMap.Range(func(key string, value *TunnelMETA) bool {
		t := value
		if tag != "" {
			if t.Tag != tag {
				return true
			}
		}

		ext := t.ConfigFormat
		if ext == "" {
			ext = tunnelFileSuffix
		}

		var tb []byte
		var err error

		switch ext {
		case ".yaml", ".yml":
			tb, err = yaml.Marshal(value)
			if err != nil {
				ERROR("Unable to transform tunnel to YAML:", err)
				outErr = err
				return false
			}
		case ".json", ".conf", "":
			tb, err = json.MarshalIndent(value, "", "    ")
			if err != nil {
				ERROR("Unable to transform tunnel to JSON:", err)
				outErr = err
				return false
			}
		default:
			err = fmt.Errorf("unsupported tunnel file format: %s (supported: .json, .yaml, .yml, .conf)", ext)
			ERROR(err)
			outErr = err
			return false
		}

		err = writeFileWithBackup(s.TunnelsPath+t.Tag+ext, tb)
		if err != nil {
			ERROR("Unable to save tunnel to disk:", err)
			outErr = err
			return false
		}

		return true
	})

	return
}

func loadTunnelsFromDisk() (err error) {
	s := STATE.Load()
	if s.TunnelsPath == "" {
		DEBUG("loadTunnelsFromDisk: no tunnels path (no active account), skip")
		return nil
	}
	foundDefault := false
	err = filepath.WalkDir(s.TunnelsPath, func(path string, d fs.DirEntry, err error) error {
		if d == nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		isSupportedFormat := ext == ".conf" || ext == ".json" || ext == ".yaml" || ext == ".yml"

		if !isSupportedFormat {
			return nil
		}

		tb, ferr := os.ReadFile(path)
		if ferr != nil {
			ERROR("Unable to read tunnel file:", ferr)
			return ferr
		}

		tunnel := new(TunnelMETA)
		var merr error

		switch ext {
		case ".yaml", ".yml":
			merr = yaml.Unmarshal(tb, tunnel)
			if merr != nil {
				ERROR("Unable to unmarshal YAML tunnel file:", merr)
				return merr
			}
		case ".json", ".conf", "":
			merr = json.Unmarshal(tb, tunnel)
			if merr != nil {
				ERROR("Unable to unmarshal JSON tunnel file:", merr)
				return merr
			}
		default:
			ERROR("Unsupported tunnel file format:", ext)
			return fmt.Errorf("unsupported tunnel file format: %s", ext)
		}

		if tunnel.Tag == "" {
			ERROR("Skipping tunnel file with empty Tag:", path)
			return nil
		}

		tunnel.ConfigFormat = ext
		TunnelMetaMap.Store(tunnel.Tag, tunnel)
		DEBUG("Loaded tunnel:", tunnel.Tag)
		if tunnel.Tag == DefaultTunnelName {
			foundDefault = true
		}

		return nil
	})
	if err != nil {
		ERROR("Unable to walk tunnel path:", err)
		return err
	}

	if !foundDefault {
		state := STATE.Load()
		newTun := createDefaultTunnelMeta(types.TunnelType(state.TunnelType))
		TunnelMetaMap.Store(newTun.Tag, newTun)
		_ = writeTunnelsToDisk(newTun.Tag)
	}
	return nil
}

func SetConfig(config *configV2) (err error) {
	defer RecoverAndLog()

	if err := validateDNSListConfig(config); err != nil {
		return err
	}

	oldConf := CONFIG.Load()

	dnsChange := oldConf.DNSServerIP != config.DNSServerIP ||
		oldConf.DNSServerPort != config.DNSServerPort

	if dnsChange {
		dnsserver := UDPDNSServer.Load()
		_ = dnsserver.Shutdown()
	}

	CONFIG.Store(config)
	reloadBlockLists(false)
	reloadWhiteLists(false)
	if ksErr := applyConfiguredKillSwitch(); ksErr != nil {
		ERROR("kill switch apply after config save: ", ksErr)
	}
	err = writeConfigToDisk()
	INFO("Config saved")

	return err
}

// validateDNSListConfig rejects blocklist/whitelist tags that could escape the
// list directories on disk (path traversal via Tag).
func validateDNSListConfig(config *configV2) error {
	if config == nil {
		return nil
	}
	for _, bl := range config.DNSBlockLists {
		if bl == nil || bl.Tag == "" {
			continue
		}
		if !safeListTag(bl.Tag) {
			return fmt.Errorf("invalid DNS blocklist tag %q: only a-z A-Z 0-9 _ - allowed", bl.Tag)
		}
	}
	for _, wl := range config.DNSWhiteLists {
		if wl == nil || wl.Tag == "" {
			continue
		}
		if !safeListTag(wl.Tag) {
			return fmt.Errorf("invalid DNS whitelist tag %q: only a-z A-Z 0-9 _ - allowed", wl.Tag)
		}
	}
	return nil
}
