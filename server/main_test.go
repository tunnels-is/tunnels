package main

import (
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func Test_validateConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         *types.ServerConfig
		expectError    bool
		expectedValues map[string]interface{}
	}{
		{
			name: "valid config with all fields set",
			config: &types.ServerConfig{
				UserMaxConnections:  5,
				PingTimeoutMinutes:  10,
				DHCPTimeoutHours:    24,
				Features:            []types.Feature{types.VPN, types.LAN},
				SecretStore:         types.EnvStore,
			},
			expectError: false,
			expectedValues: map[string]interface{}{
				"UserMaxConnections": 5,
				"PingTimeoutMinutes": 10,
				"DHCPTimeoutHours":   24,
			},
		},
		{
			name: "UserMaxConnections < 1 - should default to 2",
			config: &types.ServerConfig{
				UserMaxConnections:  0,
				PingTimeoutMinutes:  5,
				DHCPTimeoutHours:    12,
				Features:            []types.Feature{types.VPN},
				SecretStore:         types.EnvStore,
			},
			expectError: false,
			expectedValues: map[string]interface{}{
				"UserMaxConnections": 2,
			},
		},
		{
			name: "PingTimeoutMinutes < 2 - should default to 2",
			config: &types.ServerConfig{
				UserMaxConnections:  3,
				PingTimeoutMinutes:  1,
				DHCPTimeoutHours:    12,
				Features:            []types.Feature{types.VPN},
				SecretStore:         types.EnvStore,
			},
			expectError: false,
			expectedValues: map[string]interface{}{
				"PingTimeoutMinutes": 2,
			},
		},
		{
			name: "DHCPTimeoutHours < 1 - should default to 1",
			config: &types.ServerConfig{
				UserMaxConnections:  3,
				PingTimeoutMinutes:  5,
				DHCPTimeoutHours:    0,
				Features:            []types.Feature{types.VPN},
				SecretStore:         types.EnvStore,
			},
			expectError: false,
			expectedValues: map[string]interface{}{
				"DHCPTimeoutHours": 1,
			},
		},
		{
			name: "no features - should error",
			config: &types.ServerConfig{
				UserMaxConnections:  3,
				PingTimeoutMinutes:  5,
				DHCPTimeoutHours:    12,
				Features:            []types.Feature{},
				SecretStore:         types.EnvStore,
			},
			expectError: true,
		},
		{
			name: "nil features - should error",
			config: &types.ServerConfig{
				UserMaxConnections:  3,
				PingTimeoutMinutes:  5,
				DHCPTimeoutHours:    12,
				Features:            nil,
				SecretStore:         types.EnvStore,
			},
			expectError: true,
		},
		{
			name: "empty SecretStore - should default to EnvStore",
			config: &types.ServerConfig{
				UserMaxConnections:  3,
				PingTimeoutMinutes:  5,
				DHCPTimeoutHours:    12,
				Features:            []types.Feature{types.VPN},
				SecretStore:         "",
			},
			expectError: false,
			expectedValues: map[string]interface{}{
				"SecretStore": types.EnvStore,
			},
		},
		{
			name: "multiple defaults triggered",
			config: &types.ServerConfig{
				UserMaxConnections:  0,
				PingTimeoutMinutes:  0,
				DHCPTimeoutHours:    0,
				Features:            []types.Feature{types.VPN, types.LAN, types.AUTH},
				SecretStore:         "",
			},
			expectError: false,
			expectedValues: map[string]interface{}{
				"UserMaxConnections": 2,
				"PingTimeoutMinutes": 2,
				"DHCPTimeoutHours":   1,
				"SecretStore":        types.EnvStore,
			},
		},
		{
			name: "negative values should trigger defaults",
			config: &types.ServerConfig{
				UserMaxConnections:  -5,
				PingTimeoutMinutes:  -10,
				DHCPTimeoutHours:    -24,
				Features:            []types.Feature{types.VPN},
				SecretStore:         types.EnvStore,
			},
			expectError: false,
			expectedValues: map[string]interface{}{
				"UserMaxConnections": 2,
				"PingTimeoutMinutes": 2,
				"DHCPTimeoutHours":   1,
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

			// Check expected values
			for key, expected := range tc.expectedValues {
				var actual interface{}
				switch key {
				case "UserMaxConnections":
					actual = tc.config.UserMaxConnections
				case "PingTimeoutMinutes":
					actual = tc.config.PingTimeoutMinutes
				case "DHCPTimeoutHours":
					actual = tc.config.DHCPTimeoutHours
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
			name:  "DHCPTimeoutHours = 1 (boundary)",
			field: "DHCPTimeoutHours",
			value: 1,
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
		{
			name:  "DHCPTimeoutHours = 8760 (one year)",
			field: "DHCPTimeoutHours",
			value: 8760,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := &types.ServerConfig{
				UserMaxConnections:  10,
				PingTimeoutMinutes:  10,
				DHCPTimeoutHours:    10,
				Features:            []types.Feature{types.VPN},
				SecretStore:         types.EnvStore,
			}

			// Set the specific field to test
			switch tc.field {
			case "UserMaxConnections":
				config.UserMaxConnections = tc.value
			case "PingTimeoutMinutes":
				config.PingTimeoutMinutes = tc.value
			case "DHCPTimeoutHours":
				config.DHCPTimeoutHours = tc.value
			}

			err := validateConfig(config)
			if err != nil {
				t.Errorf("Unexpected error for %s=%d: %v", tc.field, tc.value, err)
			}

			// Verify value wasn't changed
			var actual int
			switch tc.field {
			case "UserMaxConnections":
				actual = config.UserMaxConnections
			case "PingTimeoutMinutes":
				actual = config.PingTimeoutMinutes
			case "DHCPTimeoutHours":
				actual = config.DHCPTimeoutHours
			}

			if actual != tc.value {
				t.Errorf("%s changed from %d to %d", tc.field, tc.value, actual)
			}

			t.Logf("%s=%d validated correctly ✓", tc.field, tc.value)
		})
	}
}
