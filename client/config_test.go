package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/types"
	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	conf := DefaultConfig()

	if conf == nil {
		t.Fatal("DefaultConfig should not return nil")
	}

	// Test boolean defaults
	if !conf.DebugLogging {
		t.Error("DebugLogging should be true by default")
	}
	if !conf.InfoLogging {
		t.Error("InfoLogging should be true by default")
	}
	if !conf.ErrorLogging {
		t.Error("ErrorLogging should be true by default")
	}
	if conf.ConnectionTracer {
		t.Error("ConnectionTracer should be false by default")
	}

	// Test DNS defaults
	if conf.DNSServerIP != "127.0.0.1" {
		t.Errorf("DNSServerIP should be 127.0.0.1, got %s", conf.DNSServerIP)
	}
	if conf.DNSServerPort != "53" {
		t.Errorf("DNSServerPort should be 53, got %s", conf.DNSServerPort)
	}
	if conf.DNS1Default != "1.1.1.1" {
		t.Errorf("DNS1Default should be 1.1.1.1, got %s", conf.DNS1Default)
	}
	if conf.DNS2Default != "8.8.8.8" {
		t.Errorf("DNS2Default should be 8.8.8.8, got %s", conf.DNS2Default)
	}

	// Test API defaults
	if conf.APIIP != "127.0.0.1" {
		t.Errorf("APIIP should be 127.0.0.1, got %s", conf.APIIP)
	}
	if conf.APIPort != "7777" {
		t.Errorf("APIPort should be 7777, got %s", conf.APIPort)
	}

	// Test update defaults
	if conf.RestartPostUpdate {
		t.Error("RestartPostUpdate should be false by default")
	}
	if conf.ExitPostUpdate {
		t.Error("ExitPostUpdate should be false by default")
	}
	if !conf.AutoDownloadUpdate {
		t.Error("AutoDownloadUpdate should be true by default")
	}
	if conf.UpdateWhileConnected {
		t.Error("UpdateWhileConnected should be false by default")
	}
	if !conf.DisableUpdates {
		t.Error("DisableUpdates should be true by default")
	}

	// Test logging defaults
	if !conf.LogBlockedDomains {
		t.Error("LogBlockedDomains should be true by default")
	}
	if !conf.LogAllDomains {
		t.Error("LogAllDomains should be true by default")
	}
	if !conf.DNSstats {
		t.Error("DNSstats should be true by default")
	}

	// Test that block/white lists are initialized
	if conf.DNSBlockLists == nil {
		t.Error("DNSBlockLists should not be nil")
	}
	if conf.DNSWhiteLists == nil {
		t.Error("DNSWhiteLists should not be nil")
	}

	// Test control servers
	if len(conf.ControlServers) != 1 {
		t.Errorf("Should have 1 default control server, got %d", len(conf.ControlServers))
	} else {
		cs := conf.ControlServers[0]
		if cs.ID != "tunnels" {
			t.Errorf("Default control server ID should be 'tunnels', got %s", cs.ID)
		}
		if cs.Host != "api.tunnels.is" {
			t.Errorf("Default control server Host should be 'api.tunnels.is', got %s", cs.Host)
		}
		if cs.Port != "443" {
			t.Errorf("Default control server Port should be '443', got %s", cs.Port)
		}
		if !cs.ValidateCertificate {
			t.Error("Default control server should validate certificates")
		}
	}

	// Test certificate defaults
	if conf.APIKey != "./api.key" {
		t.Errorf("APIKey should be './api.key', got %s", conf.APIKey)
	}
	if conf.APICert != "./api.crt" {
		t.Errorf("APICert should be './api.crt', got %s", conf.APICert)
	}
	if conf.APICertType != certs.RSA {
		t.Errorf("APICertType should be RSA, got %v", conf.APICertType)
	}

	// Test certificate IPs
	if len(conf.APICertIPs) != 2 {
		t.Errorf("Should have 2 default cert IPs, got %d", len(conf.APICertIPs))
	} else {
		if conf.APICertIPs[0] != "127.0.0.1" {
			t.Errorf("First cert IP should be 127.0.0.1, got %s", conf.APICertIPs[0])
		}
		if conf.APICertIPs[1] != "0.0.0.0" {
			t.Errorf("Second cert IP should be 0.0.0.0, got %s", conf.APICertIPs[1])
		}
	}

	// Test certificate domains
	if len(conf.APICertDomains) != 2 {
		t.Errorf("Should have 2 default cert domains, got %d", len(conf.APICertDomains))
	} else {
		if conf.APICertDomains[0] != "tunnels.app" {
			t.Errorf("First cert domain should be tunnels.app, got %s", conf.APICertDomains[0])
		}
		if conf.APICertDomains[1] != "app.tunnels.is" {
			t.Errorf("Second cert domain should be app.tunnels.is, got %s", conf.APICertDomains[1])
		}
	}

	t.Logf("Default config validation passed")
}

func TestApplyCertificateDefaultsToConfig(t *testing.T) {
	// Test with empty config
	cfg := &configV2{}
	applyCertificateDefaultsToConfig(cfg)

	if cfg.APIKey != "./api.key" {
		t.Errorf("APIKey should be set to './api.key', got %s", cfg.APIKey)
	}
	if cfg.APICert != "./api.crt" {
		t.Errorf("APICert should be set to './api.crt', got %s", cfg.APICert)
	}
	if cfg.APICertType != certs.RSA {
		t.Errorf("APICertType should be RSA, got %v", cfg.APICertType)
	}
	if len(cfg.APICertIPs) != 2 {
		t.Errorf("Should have 2 cert IPs, got %d", len(cfg.APICertIPs))
	}
	if len(cfg.APICertDomains) != 2 {
		t.Errorf("Should have 2 cert domains, got %d", len(cfg.APICertDomains))
	}

	// Test with existing values (should not override)
	cfg2 := &configV2{
		APIKey:  "/custom/key.pem",
		APICert: "/custom/cert.pem",
	}
	applyCertificateDefaultsToConfig(cfg2)

	if cfg2.APIKey != "/custom/key.pem" {
		t.Errorf("APIKey should not be overridden, got %s", cfg2.APIKey)
	}
	if cfg2.APICert != "/custom/cert.pem" {
		t.Errorf("APICert should not be overridden, got %s", cfg2.APICert)
	}

	// Test with existing cert IPs (should not override)
	cfg3 := &configV2{
		APICertIPs: []string{"192.168.1.1"},
	}
	applyCertificateDefaultsToConfig(cfg3)

	if len(cfg3.APICertIPs) != 1 {
		t.Errorf("APICertIPs should not be overridden, got %d entries", len(cfg3.APICertIPs))
	}
	if cfg3.APICertIPs[0] != "192.168.1.1" {
		t.Errorf("APICertIPs should not be overridden, got %s", cfg3.APICertIPs[0])
	}

	// Test with existing cert domains (should not override)
	cfg4 := &configV2{
		APICertDomains: []string{"custom.domain.com"},
	}
	applyCertificateDefaultsToConfig(cfg4)

	if len(cfg4.APICertDomains) != 1 {
		t.Errorf("APICertDomains should not be overridden, got %d entries", len(cfg4.APICertDomains))
	}
	if cfg4.APICertDomains[0] != "custom.domain.com" {
		t.Errorf("APICertDomains should not be overridden, got %s", cfg4.APICertDomains[0])
	}

	t.Logf("Certificate defaults application test passed")
}

func TestWriteConfigToDisk_JSON(t *testing.T) {
	// Save original state and config
	originalState := STATE.Load()
	originalConfig := CONFIG.Load()
	defer func() {
		STATE.Store(originalState)
		CONFIG.Store(originalConfig)
	}()

	tmpDir := t.TempDir()
	testConfig := DefaultConfig()
	testConfig.APIIP = "192.168.1.100"
	testConfig.APIPort = "8888"

	tests := []struct {
		name        string
		filename    string
		expectError bool
	}{
		{
			name:        "write JSON config",
			filename:    "config.json",
			expectError: false,
		},
		{
			name:        "write config with .conf extension (defaults to JSON)",
			filename:    "config.conf",
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(tmpDir, tc.filename)

			// Setup state with test config path
			testState := &stateV2{
				ConfigFileName: configPath,
			}
			STATE.Store(testState)
			CONFIG.Store(testConfig)

			// Test writing
			err := writeConfigToDisk()

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify file was created
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Error("Config file was not created")
				return
			}

			// Load and verify content
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Errorf("Failed to read saved config: %v", err)
				return
			}

			var loaded configV2
			if err := json.Unmarshal(data, &loaded); err != nil {
				t.Errorf("Failed to unmarshal saved config: %v", err)
				return
			}

			if loaded.APIIP != testConfig.APIIP {
				t.Errorf("APIIP: got %s, expected %s", loaded.APIIP, testConfig.APIIP)
			}
			if loaded.APIPort != testConfig.APIPort {
				t.Errorf("APIPort: got %s, expected %s", loaded.APIPort, testConfig.APIPort)
			}

			t.Log("JSON config saved successfully ✓")
		})
	}
}

func TestWriteConfigToDisk_YAML(t *testing.T) {
	// Save original state and config
	originalState := STATE.Load()
	originalConfig := CONFIG.Load()
	defer func() {
		STATE.Store(originalState)
		CONFIG.Store(originalConfig)
	}()

	tmpDir := t.TempDir()
	testConfig := DefaultConfig()
	testConfig.APIIP = "192.168.1.100"
	testConfig.APIPort = "8888"

	tests := []struct {
		name        string
		filename    string
		expectError bool
	}{
		{
			name:        "write YAML config with .yaml extension",
			filename:    "config.yaml",
			expectError: false,
		},
		{
			name:        "write YAML config with .yml extension",
			filename:    "config.yml",
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(tmpDir, tc.filename)

			// Setup state with test config path
			testState := &stateV2{
				ConfigFileName: configPath,
			}
			STATE.Store(testState)
			CONFIG.Store(testConfig)

			// Test writing
			err := writeConfigToDisk()

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify file was created
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Error("Config file was not created")
				return
			}

			// Load and verify content
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Errorf("Failed to read saved config: %v", err)
				return
			}

			var loaded configV2
			if err := yaml.Unmarshal(data, &loaded); err != nil {
				t.Errorf("Failed to unmarshal saved config: %v", err)
				return
			}

			if loaded.APIIP != testConfig.APIIP {
				t.Errorf("APIIP: got %s, expected %s", loaded.APIIP, testConfig.APIIP)
			}
			if loaded.APIPort != testConfig.APIPort {
				t.Errorf("APIPort: got %s, expected %s", loaded.APIPort, testConfig.APIPort)
			}

			t.Logf("YAML config (%s) saved successfully ✓", tc.filename)
		})
	}
}

func TestReadConfigFileFromDisk_JSON(t *testing.T) {
	// Save original state and config
	originalState := STATE.Load()
	originalConfig := CONFIG.Load()
	defer func() {
		STATE.Store(originalState)
		CONFIG.Store(originalConfig)
	}()

	tmpDir := t.TempDir()
	testConfig := DefaultConfig()
	testConfig.APIIP = "10.0.0.1"
	testConfig.APIPort = "9999"

	tests := []struct {
		name        string
		filename    string
		expectError bool
	}{
		{
			name:        "load JSON config",
			filename:    "config.json",
			expectError: false,
		},
		{
			name:        "load .conf config (JSON format)",
			filename:    "config.conf",
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(tmpDir, tc.filename)

			// Write test config to file
			data, err := json.MarshalIndent(testConfig, "", "    ")
			if err != nil {
				t.Fatalf("Failed to marshal test config: %v", err)
			}

			if err := os.WriteFile(configPath, data, 0o644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			// Setup state with test config path
			testState := &stateV2{
				ConfigFileName: configPath,
			}
			STATE.Store(testState)

			// Test loading
			err = ReadConfigFileFromDisk()

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify loaded config
			loaded := CONFIG.Load()
			if loaded.APIIP != testConfig.APIIP {
				t.Errorf("APIIP: got %s, expected %s", loaded.APIIP, testConfig.APIIP)
			}
			if loaded.APIPort != testConfig.APIPort {
				t.Errorf("APIPort: got %s, expected %s", loaded.APIPort, testConfig.APIPort)
			}

			t.Log("JSON config loaded successfully ✓")
		})
	}
}

func TestReadConfigFileFromDisk_YAML(t *testing.T) {
	// Save original state and config
	originalState := STATE.Load()
	originalConfig := CONFIG.Load()
	defer func() {
		STATE.Store(originalState)
		CONFIG.Store(originalConfig)
	}()

	tmpDir := t.TempDir()
	testConfig := DefaultConfig()
	testConfig.APIIP = "10.0.0.1"
	testConfig.APIPort = "9999"

	tests := []struct {
		name        string
		filename    string
		expectError bool
	}{
		{
			name:        "load YAML config with .yaml extension",
			filename:    "config.yaml",
			expectError: false,
		},
		{
			name:        "load YAML config with .yml extension",
			filename:    "config.yml",
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(tmpDir, tc.filename)

			// Write test config to file
			data, err := yaml.Marshal(testConfig)
			if err != nil {
				t.Fatalf("Failed to marshal test config: %v", err)
			}

			if err := os.WriteFile(configPath, data, 0o644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			// Setup state with test config path
			testState := &stateV2{
				ConfigFileName: configPath,
			}
			STATE.Store(testState)

			// Test loading
			err = ReadConfigFileFromDisk()

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify loaded config
			loaded := CONFIG.Load()
			if loaded.APIIP != testConfig.APIIP {
				t.Errorf("APIIP: got %s, expected %s", loaded.APIIP, testConfig.APIIP)
			}
			if loaded.APIPort != testConfig.APIPort {
				t.Errorf("APIPort: got %s, expected %s", loaded.APIPort, testConfig.APIPort)
			}

			t.Logf("YAML config (%s) loaded successfully ✓", tc.filename)
		})
	}
}

func TestConfigFileErrors(t *testing.T) {
	// Save original state
	originalState := STATE.Load()
	defer STATE.Store(originalState)

	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setupFunc   func() string
		expectError bool
		errorMsg    string
	}{
		{
			name: "unsupported file format",
			setupFunc: func() string {
				path := filepath.Join(tmpDir, "config.xml")
				_ = os.WriteFile(path, []byte("<config></config>"), 0o644)
				return path
			},
			expectError: true,
			errorMsg:    "unsupported config file format",
		},
		{
			name: "file does not exist",
			setupFunc: func() string {
				return filepath.Join(tmpDir, "nonexistent.json")
			},
			expectError: true,
			errorMsg:    "no such file or directory",
		},
		{
			name: "invalid JSON content",
			setupFunc: func() string {
				path := filepath.Join(tmpDir, "invalid.json")
				_ = os.WriteFile(path, []byte("{invalid json}"), 0o644)
				return path
			},
			expectError: true,
			errorMsg:    "invalid character",
		},
		{
			name: "invalid YAML content",
			setupFunc: func() string {
				path := filepath.Join(tmpDir, "invalid.yaml")
				_ = os.WriteFile(path, []byte("invalid:\n  yaml:\n    - [unclosed"), 0o644)
				return path
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configPath := tc.setupFunc()

			testState := &stateV2{
				ConfigFileName: configPath,
			}
			STATE.Store(testState)

			err := ReadConfigFileFromDisk()

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else {
					t.Logf("Got expected error: %v ✓", err)
					if tc.errorMsg != "" && !strings.Contains(err.Error(), tc.errorMsg) {
						t.Logf("Warning: expected error message to contain '%s', got '%s'", tc.errorMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLoadAndSaveTunnels_JSON(t *testing.T) {
	// Save original state
	originalState := STATE.Load()
	defer STATE.Store(originalState)

	tmpDir := t.TempDir()
	tunnelsPath := filepath.Join(tmpDir, "tunnels") + string(filepath.Separator)
	err := os.MkdirAll(tunnelsPath, 0o755)
	if err != nil {
		t.Fatalf("Failed to create tunnels directory: %v", err)
	}

	// Setup state
	testState := &stateV2{
		TunnelsPath: tunnelsPath,
		TunnelType:  string(types.DefaultTun),
	}
	STATE.Store(testState)

	// Create test tunnel
	testTunnel := &TunnelMETA{
		Tag:           "test-tunnel",
		IPv4Address:   "10.0.0.1",
		DNSBlocking:   true,
		AutoConnect:   false,
		AutoReconnect: false,
	}
	TunnelMetaMap.Store(testTunnel.Tag, testTunnel)

	// Test writing
	err = writeTunnelsToDisk(testTunnel.Tag)
	if err != nil {
		t.Fatalf("Failed to write tunnel: %v", err)
	}

	// Clear map and test loading
	TunnelMetaMap.Delete(testTunnel.Tag)

	err = loadTunnelsFromDisk()
	if err != nil {
		t.Fatalf("Failed to load tunnels: %v", err)
	}

	// Verify loaded tunnel
	loaded, ok := TunnelMetaMap.Load(testTunnel.Tag)
	if !ok {
		t.Error("Tunnel was not loaded")
		return
	}

	if loaded.IPv4Address != testTunnel.IPv4Address {
		t.Errorf("IPv4Address: got %s, expected %s", loaded.IPv4Address, testTunnel.IPv4Address)
	}
	if loaded.DNSBlocking != testTunnel.DNSBlocking {
		t.Errorf("DNSBlocking: got %v, expected %v", loaded.DNSBlocking, testTunnel.DNSBlocking)
	}

	t.Log("Tunnel JSON save/load cycle completed successfully ✓")
}

func TestConfigRoundTrip(t *testing.T) {
	// Save original state and config
	originalState := STATE.Load()
	originalConfig := CONFIG.Load()
	defer func() {
		STATE.Store(originalState)
		CONFIG.Store(originalConfig)
	}()

	tmpDir := t.TempDir()
	testConfig := DefaultConfig()
	testConfig.APIIP = "172.16.0.1"
	testConfig.APIPort = "12345"
	testConfig.DNSServerIP = "8.8.8.8"
	testConfig.DNSServerPort = "5353"

	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "JSON round trip",
			filename: "roundtrip.json",
		},
		{
			name:     "YAML round trip",
			filename: "roundtrip.yaml",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(tmpDir, tc.filename)

			// Setup and save
			testState := &stateV2{
				ConfigFileName: configPath,
			}
			STATE.Store(testState)
			CONFIG.Store(testConfig)

			if err := writeConfigToDisk(); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}

			// Load back
			if err := ReadConfigFileFromDisk(); err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Verify all fields match
			loaded := CONFIG.Load()
			if loaded.APIIP != testConfig.APIIP {
				t.Errorf("APIIP: got %s, expected %s", loaded.APIIP, testConfig.APIIP)
			}
			if loaded.APIPort != testConfig.APIPort {
				t.Errorf("APIPort: got %s, expected %s", loaded.APIPort, testConfig.APIPort)
			}
			if loaded.DNSServerIP != testConfig.DNSServerIP {
				t.Errorf("DNSServerIP: got %s, expected %s", loaded.DNSServerIP, testConfig.DNSServerIP)
			}
			if loaded.DNSServerPort != testConfig.DNSServerPort {
				t.Errorf("DNSServerPort: got %s, expected %s", loaded.DNSServerPort, testConfig.DNSServerPort)
			}

			t.Logf("Round trip test passed for %s ✓", tc.filename)
		})
	}
}
