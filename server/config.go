package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	"gopkg.in/yaml.v3"
)

const minSecretLen = 32

func validateServerConfig(c *types.ServerConfig) error {
	if c == nil {
		return errors.New("server config is nil")
	}
	if len(c.CookieSigningKey) < minSecretLen {
		return fmt.Errorf("CookieSigningKey must be set and at least %d characters: it seeds the AES-256 key that seals admin session cookies; generate a config with -createConfig or set a strong random value", minSecretLen)
	}
	if len(c.TwoFactorKey) < minSecretLen {
		return fmt.Errorf("TwoFactorKey must be set and at least %d characters: it seeds the AES-256 key that encrypts 2FA and recovery codes at rest; generate a config with -createConfig or set a strong random value", minSecretLen)
	}
	if c.AdminAPIKey != "" && len(c.AdminAPIKey) < 16 {
		return fmt.Errorf("AdminAPIKey must be at least 16 characters when set")
	}
	return nil
}

func parseServerConfig(path string) (*types.ServerConfig, error) {
	nb, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := new(types.ServerConfig)

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(nb, &cfg)
	case ".json", "":
		err = json.Unmarshal(nb, &cfg)
	default:
		return nil, fmt.Errorf("unsupported config file format: %s (supported: .json, .yaml, .yml)", ext)
	}
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func LoadServerConfig(path string) (err error) {
	cfg, err := parseServerConfig(path)
	if err != nil {
		return err
	}
	Config.Store(cfg)
	return nil
}

func SaveServerConfig(path string) (err error) {
	cfg := Config.Load()

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		encoder := yaml.NewEncoder(f)
		encoder.SetIndent(2)
		if err := encoder.Encode(cfg); err != nil {
			return err
		}
		_ = encoder.Close()
	case ".json", "":
		encoder := json.NewEncoder(f)
		encoder.SetIndent("", "    ")
		if err := encoder.Encode(cfg); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported config file format: %s (supported: .json, .yaml, .yml)", ext)
	}

	return nil
}

func LoadWGConfig(path string) error {
	nb, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	bootstrap := new(types.WGBootstrap)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(nb, bootstrap)
	case ".json", "":
		err = json.Unmarshal(nb, bootstrap)
	default:
		return fmt.Errorf("unsupported wg config file format: %s (supported: .json, .yaml, .yml)", ext)
	}
	if err != nil {
		return err
	}
	WGConfig.Store(bootstrap)
	return nil
}

func SaveWGConfig(path string) error {
	bootstrap := WGConfig.Load()
	if bootstrap == nil {
		return fmt.Errorf("no wg config loaded")
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		encoder := yaml.NewEncoder(f)
		encoder.SetIndent(2)
		if err := encoder.Encode(bootstrap); err != nil {
			return err
		}
		_ = encoder.Close()
	case ".json", "":
		encoder := json.NewEncoder(f)
		encoder.SetIndent("", "    ")
		if err := encoder.Encode(bootstrap); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported wg config file format: %s (supported: .json, .yaml, .yml)", ext)
	}
	return nil
}

func makeConfig(ipOverride string, mode string, skipWGVerify bool) error {
	writeServer := mode == "all" || mode == "auth"
	writeWG := mode == "all" || mode == "wg"

	if writeServer {
		if err := writeServerConfig(ipOverride, mode); err != nil {
			return err
		}
	}
	if writeWG {
		if err := writeWGConfig(skipWGVerify); err != nil {
			return err
		}
	}
	return nil
}

func writeServerConfig(ipOverride, mode string) error {
	if err := LoadServerConfig(serverConfigPath); err == nil {
		return nil
	}

	interfaceIP, err := resolveInterfaceIP(ipOverride)
	if err != nil {
		return err
	}

	newConfig := &types.ServerConfig{
		APIIP:            interfaceIP,
		APIPort:          "443",
		DBurl:            "",
		AdminAPIKey:      uuid.NewString(),
		TwoFactorKey:     strings.ReplaceAll(uuid.NewString(), "-", ""),
		CookieSigningKey: strings.ReplaceAll(uuid.NewString(), "-", ""),
		CertPem:          "./cert.pem",
		KeyPem:           "./key.pem",
	}
	Config.Store(newConfig)
	return SaveServerConfig(serverConfigPath)
}

func wgBootstrapSkipVerify(createCert string) bool {
	return strings.EqualFold(strings.TrimSpace(createCert), "selfsign")
}

func writeWGConfig(skipVerify bool) error {
	if err := LoadWGConfig(wgConfigPath); err == nil {
		return nil
	}

	WGConfig.Store(&types.WGBootstrap{InsecureSkipVerify: skipVerify})
	return SaveWGConfig(wgConfigPath)
}
