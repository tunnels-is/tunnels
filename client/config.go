package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/types"
)

// writeConfigToDisk writes the current configuration to disk
func writeConfigToDisk() (err error) {
	defer RecoverAndLog()
	conf := CONFIG.Load()
	s := STATE.Load()

	cb, err := json.Marshal(conf)
	if err != nil {
		ERROR("Unable to marshal config into bytes: ", err)
		return err
	}

	err = RenameFile(s.ConfigFileName, s.ConfigFileName+".bak")
	if err != nil {
		ERROR("Unable to rename config file: ", err)
	}

	f, err := CreateFile(s.ConfigFileName)
	if err != nil {
		ERROR("Unable to create new config", err)
		return err
	}
	defer f.Close()

	_, err = f.Write(cb)
	if err != nil {
		ERROR("Unable to write config bytes to new config file: ", err)
		return err
	}

	return
}

// ReadConfigFileFromDisk reads the configuration file from disk
func ReadConfigFileFromDisk() (err error) {
	state := STATE.Load()
	config, err := os.ReadFile(state.ConfigFileName)
	if err != nil {
		return err
	}

	Conf := new(configV2)
	err = json.Unmarshal(config, Conf)
	if err != nil {
		ERROR("Unable to turn config file into config object: ", err)
		return
	}

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

// loadConfigFromDisk loads configuration from disk or creates default if not found
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

// DefaultConfig returns a new configV2 with default values
func DefaultConfig() *configV2 {
	conf := &configV2{
		DebugLogging:         true,
		InfoLogging:          true,
		ErrorLogging:         true,
		ConnectionTracer:     false,
		DNSServerIP:          "127.0.0.1",
		DNSServerPort:        "53",
		DNS1Default:          "1.1.1.1",
		DNS2Default:          "8.8.8.8",
		LogBlockedDomains:    true,
		LogAllDomains:        true,
		DNSstats:             true,
		DNSBlockLists:        GetDefaultBlockLists(),
		APIIP:                "127.0.0.1",
		APIPort:              "7777",
		RestartPostUpdate:    false,
		ExitPostUpdate:       false,
		AutoDownloadUpdate:   true,
		UpdateWhileConnected: false,
		DisableUpdates:       true,
	}
	conf.ControlServers = append(conf.ControlServers, &ControlServer{
		ID:                  "tunnels",
		Host:                "api.tunnels.is",
		Port:                "443",
		CertificatePath:     "",
		ValidateCertificate: true,
	})
	applyCertificateDefaultsToConfig(conf)
	return conf
}

// applyCertificateDefaultsToConfig sets default certificate configuration
func applyCertificateDefaultsToConfig(cfg *configV2) {
	if cfg.APIKey == "" {
		cfg.APIKey = "./api.key"
	}
	if cfg.APICert == "" {
		cfg.APICert = "./api.crt"
	}

	cfg.APICertType = certs.RSA

	if len(cfg.APICertIPs) < 1 {
		cfg.APICertIPs = []string{"127.0.0.1", "0.0.0.0"}
	}

	if len(cfg.APICertDomains) < 1 {
		cfg.APICertDomains = []string{"tunnels.app", "app.tunnels.is"}
	}
}

// writeTunnelsToDisk writes tunnel configurations to disk
func writeTunnelsToDisk(tag string) (outErr error) {
	s := STATE.Load()
	TunnelMetaMap.Range(func(key string, value *TunnelMETA) bool {
		t := value
		if tag != "" {
			if t.Tag != tag {
				return true
			}
		}
		tb, err := json.Marshal(value)
		if err != nil {
			ERROR("Unable to transform tunnel to json:", err)
			outErr = err
			return false
		}

		err = RenameFile(s.TunnelsPath+t.Tag+tunnelFileSuffix, s.TunnelsPath+t.Tag+tunnelFileSuffix+backupFileSuffix)
		if err != nil {
			ERROR("Unable to rename tunnel file:", err)
		}

		tf, err := CreateFile(s.TunnelsPath + t.Tag + tunnelFileSuffix)
		if err != nil {
			ERROR("Unable to save tunnel to disk:", err)
			outErr = err
			return false
		}

		_, err = tf.Write(tb)
		if err != nil {
			ERROR("Unable to write tunnel json to file:", err)
			outErr = err
			return false
		}
		tf.Sync()
		tf.Close()

		return true
	})

	return
}

// loadTunnelsFromDisk loads tunnel configurations from disk
func loadTunnelsFromDisk() (err error) {
	s := STATE.Load()
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

		if !strings.HasSuffix(path, tunnelFileSuffix) {
			return nil
		}

		tb, ferr := os.ReadFile(path)
		if ferr != nil {
			ERROR("Unable to read tunnel file:", err)
			return err
		}

		tunnel := new(TunnelMETA)
		merr := json.Unmarshal(tb, tunnel)
		if merr != nil {
			ERROR("Unable to marshal tunnel file:", err)
			return err
		}
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

// SetConfig updates and persists the configuration
func SetConfig(config *configV2) (err error) {
	defer RecoverAndLog()

	oldConf := CONFIG.Load()

	dnsChange := oldConf.DNSServerIP != config.DNSServerIP ||
		oldConf.DNSServerPort != config.DNSServerPort

	if dnsChange {
		dnsserver := UDPDNSServer.Load()
		_ = dnsserver.Shutdown()
	}

	apiChange := oldConf.APIPort != config.APIPort ||
		oldConf.APIIP != config.APIIP ||
		oldConf.APICert != config.APICert ||
		oldConf.APIKey != config.APIKey ||
		!slices.Equal(config.APICertDomains, oldConf.APICertDomains) ||
		!slices.Equal(config.APICertIPs, oldConf.APICertIPs)

	if apiChange {
		_ = API_SERVER.Shutdown(context.Background())
	}

	CONFIG.Store(config)
	reloadBlockLists(false)
	err = writeConfigToDisk()
	INFO("Config saved")
	DEBUG(fmt.Sprintf("%+v", *config))

	return err
}
