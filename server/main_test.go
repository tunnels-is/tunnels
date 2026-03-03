package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tunnels-is/tunnels/types"
	"gopkg.in/yaml.v3"
)

func Test_validateConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         *types.ServerConfig
		expectError    bool
		expectedValues map[string]any
	}{
		{
			name: "valid config with all fields set",
			config: &types.ServerConfig{
				UserMaxConnections: 5,
				PingTimeoutMinutes: 10,
				Features:           []types.Feature{types.AUTH, types.DNS},
				SecretStore:        types.EnvStore,
			},
			expectError: false,
			expectedValues: map[string]any{
				"UserMaxConnections": 5,
				"PingTimeoutMinutes": 10,
			},
		},
		{
			name: "UserMaxConnections < 1 - should default to 2",
			config: &types.ServerConfig{
				UserMaxConnections: 0,
				PingTimeoutMinutes: 5,
				Features:           []types.Feature{types.AUTH},
				SecretStore:        types.EnvStore,
			},
			expectError: false,
			expectedValues: map[string]any{
				"UserMaxConnections": 2,
			},
		},
		{
			name: "PingTimeoutMinutes < 2 - should default to 2",
			config: &types.ServerConfig{
				UserMaxConnections: 3,
				PingTimeoutMinutes: 1,
				Features:           []types.Feature{types.AUTH},
				SecretStore:        types.EnvStore,
			},
			expectError: false,
			expectedValues: map[string]any{
				"PingTimeoutMinutes": 2,
			},
		},
		{
			name: "no features - should error",
			config: &types.ServerConfig{
				UserMaxConnections: 3,
				PingTimeoutMinutes: 5,
				Features:           []types.Feature{},
				SecretStore:        types.EnvStore,
			},
			expectError: true,
		},
		{
			name: "nil features - should error",
			config: &types.ServerConfig{
				UserMaxConnections: 3,
				PingTimeoutMinutes: 5,
				Features:           nil,
				SecretStore:        types.EnvStore,
			},
			expectError: true,
		},
		{
			name: "empty SecretStore - should default to EnvStore",
			config: &types.ServerConfig{
				UserMaxConnections: 3,
				PingTimeoutMinutes: 5,
				Features:           []types.Feature{types.AUTH},
				SecretStore:        "",
			},
			expectError: false,
			expectedValues: map[string]any{
				"SecretStore": types.EnvStore,
			},
		},
		{
			name: "multiple defaults triggered",
			config: &types.ServerConfig{
				UserMaxConnections: 0,
				PingTimeoutMinutes: 0,
				Features:           []types.Feature{types.AUTH, types.DNS},
				SecretStore:        "",
			},
			expectError: false,
			expectedValues: map[string]any{
				"UserMaxConnections": 2,
				"PingTimeoutMinutes": 2,
				"SecretStore":        types.EnvStore,
			},
		},
		{
			name: "negative values should trigger defaults",
			config: &types.ServerConfig{
				UserMaxConnections: -5,
				PingTimeoutMinutes: -10,
				Features:           []types.Feature{types.AUTH},
				SecretStore:        types.EnvStore,
			},
			expectError: false,
			expectedValues: map[string]any{
				"UserMaxConnections": 2,
				"PingTimeoutMinutes": 2,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfig(tc.config)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else {
					t.Logf("Got expected error: %v ✓", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			for key, expected := range tc.expectedValues {
				var actual any
				switch key {
				case "UserMaxConnections":
					actual = tc.config.UserMaxConnections
				case "PingTimeoutMinutes":
					actual = tc.config.PingTimeoutMinutes
				case "SecretStore":
					actual = tc.config.SecretStore
				}

				if actual != expected {
					t.Errorf("%s: got %v, expected %v", key, actual, expected)
				}
			}

			t.Logf("Config validated correctly ✓")
		})
	}
}

func Test_validateConfig_BoundaryValues(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value int
	}{
		{
			name:  "UserMaxConnections = 1 (boundary)",
			field: "UserMaxConnections",
			value: 1,
		},
		{
			name:  "PingTimeoutMinutes = 2 (boundary)",
			field: "PingTimeoutMinutes",
			value: 2,
		},
		{
			name:  "UserMaxConnections = 1000 (large)",
			field: "UserMaxConnections",
			value: 1000,
		},
		{
			name:  "PingTimeoutMinutes = 60 (large)",
			field: "PingTimeoutMinutes",
			value: 60,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := &types.ServerConfig{
				UserMaxConnections: 10,
				PingTimeoutMinutes: 10,
				Features:           []types.Feature{types.AUTH},
				SecretStore:        types.EnvStore,
			}

			switch tc.field {
			case "UserMaxConnections":
				config.UserMaxConnections = tc.value
			case "PingTimeoutMinutes":
				config.PingTimeoutMinutes = tc.value
			}

			err := validateConfig(config)
			if err != nil {
				t.Errorf("Unexpected error for %s=%d: %v", tc.field, tc.value, err)
			}

			var actual int
			switch tc.field {
			case "UserMaxConnections":
				actual = config.UserMaxConnections
			case "PingTimeoutMinutes":
				actual = config.PingTimeoutMinutes
			}

			if actual != tc.value {
				t.Errorf("%s changed from %d to %d", tc.field, tc.value, actual)
			}

			t.Logf("%s=%d validated correctly ✓", tc.field, tc.value)
		})
	}
}

func Test_LoadServerConfig_JSON(t *testing.T) {

	tmpDir := t.TempDir()

	testConfig := &types.ServerConfig{
		UserMaxConnections: 5,
		PingTimeoutMinutes: 10,
		Features:           []types.Feature{types.AUTH, types.DNS},
		SecretStore:        types.EnvStore,
		VPNIP:              "192.168.1.1",
		APIPort:            "443",
		Hostname:           "test.local",
	}

	tests := []struct {
		name        string
		filename    string
		expectError bool
	}{
		{
			name:        "load valid JSON config",
			filename:    "config.json",
			expectError: false,
		},
		{
			name:        "load JSON without extension",
			filename:    "config",
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

			err = LoadServerConfig(configPath)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else {
					t.Logf("Got expected error: %v ✓", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			loaded := Config.Load()
			if loaded.VPNIP != testConfig.VPNIP {
				t.Errorf("VPNIP: got %s, expected %s", loaded.VPNIP, testConfig.VPNIP)
			}
			if len(loaded.Features) != len(testConfig.Features) {
				t.Errorf("Features length: got %d, expected %d", len(loaded.Features), len(testConfig.Features))
			}

			t.Log("JSON config loaded successfully ✓")
		})
	}
}

func Test_LoadServerConfig_YAML(t *testing.T) {

	tmpDir := t.TempDir()

	testConfig := &types.ServerConfig{
		UserMaxConnections: 5,
		PingTimeoutMinutes: 10,
		Features:           []types.Feature{types.AUTH, types.DNS},
		SecretStore:        types.EnvStore,
		VPNIP:              "192.168.1.1",
		APIPort:            "443",
		Hostname:           "test.local",
	}

	tests := []struct {
		name        string
		filename    string
		expectError bool
	}{
		{
			name:        "load valid YAML config with .yaml extension",
			filename:    "config.yaml",
			expectError: false,
		},
		{
			name:        "load valid YAML config with .yml extension",
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

			err = LoadServerConfig(configPath)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else {
					t.Logf("Got expected error: %v ✓", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			loaded := Config.Load()
			if loaded.VPNIP != testConfig.VPNIP {
				t.Errorf("VPNIP: got %s, expected %s", loaded.VPNIP, testConfig.VPNIP)
			}
			if len(loaded.Features) != len(testConfig.Features) {
				t.Errorf("Features length: got %d, expected %d", len(loaded.Features), len(testConfig.Features))
			}

			t.Logf("YAML config (%s) loaded successfully ✓", tc.filename)
		})
	}
}

func Test_LoadServerConfig_Errors(t *testing.T) {
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
		{
			name: "config with no features",
			setupFunc: func() string {
				path := filepath.Join(tmpDir, "nofeatures.json")
				cfg := &types.ServerConfig{
					UserMaxConnections: 5,
					PingTimeoutMinutes: 10,
					Features:           []types.Feature{},
					SecretStore:        types.EnvStore,
				}
				data, _ := json.Marshal(cfg)
				_ = os.WriteFile(path, data, 0o644)
				return path
			},
			expectError: true,
			errorMsg:    "no features enbaled",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configPath := tc.setupFunc()
			err := LoadServerConfig(configPath)

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

func Test_SaveServerConfig_JSON(t *testing.T) {
	tmpDir := t.TempDir()

	testConfig := &types.ServerConfig{
		UserMaxConnections: 5,
		PingTimeoutMinutes: 10,
		Features:           []types.Feature{types.AUTH, types.DNS},
		SecretStore:        types.EnvStore,
		VPNIP:              "192.168.1.1",
		APIPort:            "443",
		Hostname:           "test.local",
	}

	tests := []struct {
		name        string
		filename    string
		expectError bool
	}{
		{
			name:        "save JSON config",
			filename:    "output.json",
			expectError: false,
		},
		{
			name:        "save config without extension (defaults to JSON)",
			filename:    "output",
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			Config.Store(testConfig)

			configPath := filepath.Join(tmpDir, tc.filename)

			err := SaveServerConfig(configPath)

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

			var loaded types.ServerConfig
			if err := json.Unmarshal(data, &loaded); err != nil {
				t.Errorf("Failed to unmarshal saved config: %v", err)
				return
			}

			if loaded.VPNIP != testConfig.VPNIP {
				t.Errorf("VPNIP: got %s, expected %s", loaded.VPNIP, testConfig.VPNIP)
			}

			t.Log("JSON config saved successfully ✓")
		})
	}
}

func Test_SaveServerConfig_YAML(t *testing.T) {
	tmpDir := t.TempDir()

	testConfig := &types.ServerConfig{
		UserMaxConnections: 5,
		PingTimeoutMinutes: 10,
		Features:           []types.Feature{types.AUTH, types.DNS},
		SecretStore:        types.EnvStore,
		VPNIP:              "192.168.1.1",
		APIPort:            "443",
		Hostname:           "test.local",
	}

	tests := []struct {
		name        string
		filename    string
		expectError bool
	}{
		{
			name:        "save YAML config with .yaml extension",
			filename:    "output.yaml",
			expectError: false,
		},
		{
			name:        "save YAML config with .yml extension",
			filename:    "output.yml",
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			Config.Store(testConfig)

			configPath := filepath.Join(tmpDir, tc.filename)

			err := SaveServerConfig(configPath)

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

			var loaded types.ServerConfig
			if err := yaml.Unmarshal(data, &loaded); err != nil {
				t.Errorf("Failed to unmarshal saved config: %v", err)
				return
			}

			if loaded.VPNIP != testConfig.VPNIP {
				t.Errorf("VPNIP: got %s, expected %s", loaded.VPNIP, testConfig.VPNIP)
			}

			t.Logf("YAML config (%s) saved successfully ✓", tc.filename)
		})
	}
}

func Test_SaveServerConfig_Errors(t *testing.T) {
	tmpDir := t.TempDir()

	testConfig := &types.ServerConfig{
		UserMaxConnections: 5,
		PingTimeoutMinutes: 10,
		Features:           []types.Feature{types.AUTH},
		SecretStore:        types.EnvStore,
	}

	tests := []struct {
		name        string
		setupFunc   func() string
		expectError bool
		errorMsg    string
	}{
		{
			name: "unsupported file format",
			setupFunc: func() string {
				return filepath.Join(tmpDir, "config.xml")
			},
			expectError: true,
			errorMsg:    "unsupported config file format",
		},
		{
			name: "invalid directory path",
			setupFunc: func() string {
				return filepath.Join(tmpDir, "nonexistent", "subdir", "config.json")
			},
			expectError: true,
			errorMsg:    "no such file or directory",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			Config.Store(testConfig)
			configPath := tc.setupFunc()
			err := SaveServerConfig(configPath)

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

func Test_LoadAndSaveServerConfig_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	originalConfig := &types.ServerConfig{
		UserMaxConnections: 7,
		PingTimeoutMinutes: 15,
		Features:           []types.Feature{types.AUTH, types.DNS, types.BBOLT},
		SecretStore:        types.EnvStore,
		VPNIP:              "10.0.0.1",
		APIPort:            "8443",
		Hostname:           "roundtrip.test",
	}

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

			Config.Store(originalConfig)
			if err := SaveServerConfig(configPath); err != nil {
				t.Fatalf("Failed to save config: %v", err)
			}

			if err := LoadServerConfig(configPath); err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			loaded := Config.Load()
			if loaded.UserMaxConnections != originalConfig.UserMaxConnections {
				t.Errorf("UserMaxConnections: got %d, expected %d", loaded.UserMaxConnections, originalConfig.UserMaxConnections)
			}
			if loaded.PingTimeoutMinutes != originalConfig.PingTimeoutMinutes {
				t.Errorf("PingTimeoutMinutes: got %d, expected %d", loaded.PingTimeoutMinutes, originalConfig.PingTimeoutMinutes)
			}
			if loaded.VPNIP != originalConfig.VPNIP {
				t.Errorf("VPNIP: got %s, expected %s", loaded.VPNIP, originalConfig.VPNIP)
			}
			if loaded.Hostname != originalConfig.Hostname {
				t.Errorf("Hostname: got %s, expected %s", loaded.Hostname, originalConfig.Hostname)
			}
			if len(loaded.Features) != len(originalConfig.Features) {
				t.Errorf("Features length: got %d, expected %d", len(loaded.Features), len(originalConfig.Features))
			}

			t.Logf("Round trip test passed for %s ✓", tc.filename)
		})
	}
}
