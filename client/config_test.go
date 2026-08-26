package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tunnels-is/tunnels/types"
	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	conf := DefaultConfig()

	if conf == nil {
		t.Fatal("DefaultConfig should not return nil")
	}

	if conf.DebugLogging {
		t.Error("DebugLogging should be false by default")
	}
	if conf.InfoLogging {
		t.Error("InfoLogging should be false by default")
	}
	if conf.ErrorLogging {
		t.Error("ErrorLogging should be false by default")
	}
	if conf.ConnectionTracer {
		t.Error("ConnectionTracer should be false by default")
	}

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

	if conf.LogBlockedDomains {
		t.Error("LogBlockedDomains should be false by default")
	}
	if conf.LogAllDomains {
		t.Error("LogAllDomains should be false by default")
	}
	if conf.DNSstats {
		t.Error("DNSstats should be false by default")
	}

	if conf.DNSBlockLists == nil {
		t.Error("DNSBlockLists should not be nil")
	}
	if conf.DNSWhiteLists == nil {
		t.Error("DNSWhiteLists should not be nil")
	}

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

	t.Logf("Default config validation passed")
}

func TestWriteConfigToDisk_JSON(t *testing.T) {

	originalState := STATE.Load()
	originalConfig := CONFIG.Load()
	defer func() {
		STATE.Store(originalState)
		CONFIG.Store(originalConfig)
	}()

	tmpDir := t.TempDir()
	testConfig := DefaultConfig()

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

			testState := &stateV2{
				ConfigFileName: configPath,
			}
			STATE.Store(testState)
			CONFIG.Store(testConfig)

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

			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Error("Config file was not created")
				return
			}

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

			t.Log("JSON config saved successfully ✓")
		})
	}
}

func TestWriteConfigToDisk_YAML(t *testing.T) {

	originalState := STATE.Load()
	originalConfig := CONFIG.Load()
	defer func() {
		STATE.Store(originalState)
		CONFIG.Store(originalConfig)
	}()

	tmpDir := t.TempDir()
	testConfig := DefaultConfig()

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

			testState := &stateV2{
				ConfigFileName: configPath,
			}
			STATE.Store(testState)
			CONFIG.Store(testConfig)

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

			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Error("Config file was not created")
				return
			}

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

			t.Logf("YAML config (%s) saved successfully ✓", tc.filename)
		})
	}
}

func TestReadConfigFileFromDisk_JSON(t *testing.T) {

	originalState := STATE.Load()
	originalConfig := CONFIG.Load()
	defer func() {
		STATE.Store(originalState)
		CONFIG.Store(originalConfig)
	}()

	tmpDir := t.TempDir()
	testConfig := DefaultConfig()

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

			data, err := json.MarshalIndent(testConfig, "", "    ")
			if err != nil {
				t.Fatalf("Failed to marshal test config: %v", err)
			}

			if err := os.WriteFile(configPath, data, 0o644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			testState := &stateV2{
				ConfigFileName: configPath,
			}
			STATE.Store(testState)

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

			t.Log("JSON config loaded successfully ✓")
		})
	}
}

func TestReadConfigFileFromDisk_YAML(t *testing.T) {

	originalState := STATE.Load()
	originalConfig := CONFIG.Load()
	defer func() {
		STATE.Store(originalState)
		CONFIG.Store(originalConfig)
	}()

	tmpDir := t.TempDir()
	testConfig := DefaultConfig()

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

			data, err := yaml.Marshal(testConfig)
			if err != nil {
				t.Fatalf("Failed to marshal test config: %v", err)
			}

			if err := os.WriteFile(configPath, data, 0o644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			testState := &stateV2{
				ConfigFileName: configPath,
			}
			STATE.Store(testState)

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

			t.Logf("YAML config (%s) loaded successfully ✓", tc.filename)
		})
	}
}

func TestConfigFileErrors(t *testing.T) {

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

func TestTunnelConfigFormats(t *testing.T) {
	originalState := STATE.Load()
	defer STATE.Store(originalState)

	formats := []struct {
		name      string
		extension string
		marshal   func(v any) ([]byte, error)
	}{
		{"JSON", ".json", func(v any) ([]byte, error) { return json.MarshalIndent(v, "", "    ") }},
		{"YAML", ".yaml", yaml.Marshal},
		{"YML", ".yml", yaml.Marshal},
		{"CONF", ".conf", func(v any) ([]byte, error) { return json.MarshalIndent(v, "", "    ") }},
	}

	for _, fmt := range formats {
		t.Run(fmt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tunnelsPath := filepath.Join(tmpDir, "tunnels") + string(filepath.Separator)
			if err := os.MkdirAll(tunnelsPath, 0o755); err != nil {
				t.Fatalf("Failed to create tunnels directory: %v", err)
			}

			testState := &stateV2{
				TunnelsPath: tunnelsPath,
				TunnelType:  string(types.DefaultTun),
			}
			STATE.Store(testState)

			testTunnel := &TunnelMETA{
				Tag:           "test-tunnel-" + fmt.name,
				DNSBlocking:   true,
				AutoConnect:   true,
				AutoReconnect: true,
				ConfigFormat:  fmt.extension,
			}

			data, err := fmt.marshal(testTunnel)
			if err != nil {
				t.Fatalf("Failed to marshal tunnel: %v", err)
			}

			filePath := filepath.Join(tunnelsPath, testTunnel.Tag+fmt.extension)
			if err := os.WriteFile(filePath, data, 0o644); err != nil {
				t.Fatalf("Failed to write tunnel file: %v", err)
			}

			clearTunnelMap()

			if err := loadTunnelsFromDisk(); err != nil {
				t.Fatalf("Failed to load tunnels: %v", err)
			}

			loaded, ok := TunnelMetaMap.Load(testTunnel.Tag)
			if !ok {
				t.Fatalf("Tunnel %s was not loaded from %s file", testTunnel.Tag, fmt.extension)
			}

			if loaded.DNSBlocking != testTunnel.DNSBlocking {
				t.Errorf("DNSBlocking: got %v, expected %v", loaded.DNSBlocking, testTunnel.DNSBlocking)
			}
			if loaded.AutoReconnect != testTunnel.AutoReconnect {
				t.Errorf("AutoReconnect: got %v, expected %v", loaded.AutoReconnect, testTunnel.AutoReconnect)
			}
			if loaded.AutoConnect != testTunnel.AutoConnect {
				t.Errorf("AutoConnect: got %v, expected %v", loaded.AutoConnect, testTunnel.AutoConnect)
			}
			if loaded.ConfigFormat != fmt.extension {
				t.Errorf("ConfigFormat: got %s, expected %s", loaded.ConfigFormat, fmt.extension)
			}

			t.Logf("Tunnel %s format test passed", fmt.name)
		})
	}
}

func TestTunnelConfigFormatPreservation(t *testing.T) {
	originalState := STATE.Load()
	defer STATE.Store(originalState)

	formats := []string{".json", ".yaml", ".yml", ".conf"}

	for _, ext := range formats {
		t.Run("format_"+ext, func(t *testing.T) {
			tmpDir := t.TempDir()
			tunnelsPath := filepath.Join(tmpDir, "tunnels") + string(filepath.Separator)
			if err := os.MkdirAll(tunnelsPath, 0o755); err != nil {
				t.Fatalf("Failed to create tunnels directory: %v", err)
			}

			testState := &stateV2{
				TunnelsPath: tunnelsPath,
				TunnelType:  string(types.DefaultTun),
			}
			STATE.Store(testState)

			testTunnel := &TunnelMETA{
				Tag:          "format-test",
				ConfigFormat: ext,
			}

			var data []byte
			var err error
			if ext == ".yaml" || ext == ".yml" {
				data, err = yaml.Marshal(testTunnel)
			} else {
				data, err = json.MarshalIndent(testTunnel, "", "    ")
			}
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			if err := os.WriteFile(filepath.Join(tunnelsPath, "format-test"+ext), data, 0o644); err != nil {
				t.Fatalf("Failed to write file: %v", err)
			}

			clearTunnelMap()
			if err := loadTunnelsFromDisk(); err != nil {
				t.Fatalf("Failed to load tunnels: %v", err)
			}

			loaded, ok := TunnelMetaMap.Load("format-test")
			if !ok {
				t.Fatal("Tunnel was not loaded")
			}

			if err := writeTunnelsToDisk("format-test"); err != nil {
				t.Fatalf("Failed to write tunnel: %v", err)
			}

			savedPath := filepath.Join(tunnelsPath, "format-test"+ext)
			if _, err := os.Stat(savedPath); os.IsNotExist(err) {
				t.Errorf("Tunnel was not saved with correct extension %s", ext)
			}

			if loaded.ConfigFormat != ext {
				t.Errorf("ConfigFormat not preserved: got %s, expected %s", loaded.ConfigFormat, ext)
			}
		})
	}
}

func TestTunnelSkipsInvalidExtensions(t *testing.T) {
	originalState := STATE.Load()
	defer STATE.Store(originalState)

	tmpDir := t.TempDir()
	tunnelsPath := filepath.Join(tmpDir, "tunnels") + string(filepath.Separator)
	if err := os.MkdirAll(tunnelsPath, 0o755); err != nil {
		t.Fatalf("Failed to create tunnels directory: %v", err)
	}

	testState := &stateV2{
		TunnelsPath: tunnelsPath,
		TunnelType:  string(types.DefaultTun),
	}
	STATE.Store(testState)

	invalidFiles := []struct {
		name    string
		content string
	}{
		{"tunnel.txt", `{"Tag": "txt-tunnel", "IPv4Address": "10.0.0.1"}`},
		{"tunnel.xml", `{"Tag": "xml-tunnel", "IPv4Address": "10.0.0.2"}`},
		{"tunnel.bak", `{"Tag": "bak-tunnel", "IPv4Address": "10.0.0.3"}`},
		{"README.md", `# Tunnel Config Directory`},
		{".hidden", `{"Tag": "hidden-tunnel", "IPv4Address": "10.0.0.4"}`},
		{"tunnel.toml", `Tag = "toml-tunnel"`},
		{"tunnel.ini", `[tunnel]\nTag=ini-tunnel`},
		{"tunnel.conf.bak", `{"Tag": "conf-bak-tunnel", "IPv4Address": "10.0.0.5"}`},
	}

	for _, f := range invalidFiles {
		if err := os.WriteFile(filepath.Join(tunnelsPath, f.name), []byte(f.content), 0o644); err != nil {
			t.Fatalf("Failed to write %s: %v", f.name, err)
		}
	}

	validTunnel := &TunnelMETA{Tag: "valid-tunnel"}
	data, _ := json.Marshal(validTunnel)
	if err := os.WriteFile(filepath.Join(tunnelsPath, "valid-tunnel.json"), data, 0o644); err != nil {
		t.Fatalf("Failed to write valid tunnel: %v", err)
	}

	clearTunnelMap()
	if err := loadTunnelsFromDisk(); err != nil {
		t.Fatalf("Failed to load tunnels: %v", err)
	}

	if _, ok := TunnelMetaMap.Load("valid-tunnel"); !ok {
		t.Error("Valid tunnel was not loaded")
	}

	invalidTags := []string{"txt-tunnel", "xml-tunnel", "bak-tunnel", "hidden-tunnel", "toml-tunnel", "ini-tunnel", "conf-bak-tunnel"}
	for _, tag := range invalidTags {
		if _, ok := TunnelMetaMap.Load(tag); ok {
			t.Errorf("Invalid tunnel %s should not have been loaded", tag)
		}
	}

	t.Log("Invalid extension filtering test passed")
}

func TestTunnelSkipsEmptyTag(t *testing.T) {
	originalState := STATE.Load()
	defer STATE.Store(originalState)

	tmpDir := t.TempDir()
	tunnelsPath := filepath.Join(tmpDir, "tunnels") + string(filepath.Separator)
	if err := os.MkdirAll(tunnelsPath, 0o755); err != nil {
		t.Fatalf("Failed to create tunnels directory: %v", err)
	}

	testState := &stateV2{
		TunnelsPath: tunnelsPath,
		TunnelType:  string(types.DefaultTun),
	}
	STATE.Store(testState)

	emptyTagTunnel := &TunnelMETA{Tag: ""}
	data, _ := json.Marshal(emptyTagTunnel)
	if err := os.WriteFile(filepath.Join(tunnelsPath, "empty-tag.json"), data, 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	validTunnel := &TunnelMETA{Tag: "valid-tag"}
	data, _ = json.Marshal(validTunnel)
	if err := os.WriteFile(filepath.Join(tunnelsPath, "valid-tag.json"), data, 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	clearTunnelMap()
	if err := loadTunnelsFromDisk(); err != nil {
		t.Fatalf("Failed to load tunnels: %v", err)
	}

	if _, ok := TunnelMetaMap.Load(""); ok {
		t.Error("Tunnel with empty tag should not have been loaded")
	}

	if _, ok := TunnelMetaMap.Load("valid-tag"); !ok {
		t.Error("Valid tunnel was not loaded")
	}

	t.Log("Empty tag filtering test passed")
}

func TestTunnelInvalidContent(t *testing.T) {
	originalState := STATE.Load()
	defer STATE.Store(originalState)

	tmpDir := t.TempDir()
	tunnelsPath := filepath.Join(tmpDir, "tunnels") + string(filepath.Separator)
	if err := os.MkdirAll(tunnelsPath, 0o755); err != nil {
		t.Fatalf("Failed to create tunnels directory: %v", err)
	}

	testState := &stateV2{
		TunnelsPath: tunnelsPath,
		TunnelType:  string(types.DefaultTun),
	}
	STATE.Store(testState)

	if err := os.WriteFile(filepath.Join(tunnelsPath, "invalid.json"), []byte("{invalid json}"), 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	clearTunnelMap()
	err := loadTunnelsFromDisk()
	if err == nil {
		t.Error("Expected error for invalid JSON content")
	}

	t.Log("Invalid content error handling test passed")
}

func TestTunnelInvalidYAMLContent(t *testing.T) {
	originalState := STATE.Load()
	defer STATE.Store(originalState)

	tmpDir := t.TempDir()
	tunnelsPath := filepath.Join(tmpDir, "tunnels") + string(filepath.Separator)
	if err := os.MkdirAll(tunnelsPath, 0o755); err != nil {
		t.Fatalf("Failed to create tunnels directory: %v", err)
	}

	testState := &stateV2{
		TunnelsPath: tunnelsPath,
		TunnelType:  string(types.DefaultTun),
	}
	STATE.Store(testState)

	if err := os.WriteFile(filepath.Join(tunnelsPath, "invalid.yaml"), []byte("invalid:\n  yaml:\n    - [unclosed"), 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	clearTunnelMap()
	err := loadTunnelsFromDisk()
	if err == nil {
		t.Error("Expected error for invalid YAML content")
	}

	t.Log("Invalid YAML content error handling test passed")
}

func TestTunnelDirectoriesAreSkipped(t *testing.T) {
	originalState := STATE.Load()
	defer STATE.Store(originalState)

	tmpDir := t.TempDir()
	tunnelsPath := filepath.Join(tmpDir, "tunnels") + string(filepath.Separator)
	if err := os.MkdirAll(tunnelsPath, 0o755); err != nil {
		t.Fatalf("Failed to create tunnels directory: %v", err)
	}

	subDir := filepath.Join(tunnelsPath, "subdir.json")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	testState := &stateV2{
		TunnelsPath: tunnelsPath,
		TunnelType:  string(types.DefaultTun),
	}
	STATE.Store(testState)

	validTunnel := &TunnelMETA{Tag: "valid"}
	data, _ := json.Marshal(validTunnel)
	if err := os.WriteFile(filepath.Join(tunnelsPath, "valid.json"), data, 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	clearTunnelMap()
	if err := loadTunnelsFromDisk(); err != nil {
		t.Fatalf("Failed to load tunnels: %v", err)
	}

	if _, ok := TunnelMetaMap.Load("valid"); !ok {
		t.Error("Valid tunnel was not loaded")
	}

	t.Log("Directory skipping test passed")
}

func clearTunnelMap() {
	TunnelMetaMap.Range(func(key string, value *TunnelMETA) bool {
		TunnelMetaMap.Delete(key)
		return true
	})
}

func TestConfigRoundTrip(t *testing.T) {

	originalState := STATE.Load()
	originalConfig := CONFIG.Load()
	defer func() {
		STATE.Store(originalState)
		CONFIG.Store(originalConfig)
	}()

	tmpDir := t.TempDir()
	testConfig := DefaultConfig()
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

			testState := &stateV2{
				ConfigFileName: configPath,
			}
			STATE.Store(testState)
			CONFIG.Store(testConfig)

			if err := writeConfigToDisk(); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}

			if err := ReadConfigFileFromDisk(); err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			loaded := CONFIG.Load()
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
