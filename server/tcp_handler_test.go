package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
)

// Test data structure for organizing test cases
type testCase struct {
	name           string
	header         string
	jsonData       string
	expectError    bool
	requiredFeature string // "LAN", "VPN", "AUTH", "PAY" (payment)
}

// setupTestEnvironment sets up the test environment with necessary global variables
func setupTestEnvironment() {
	// Initialize logger if not already initialized
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}
	
	// Set all feature flags to true for testing
	LANEnabled = true
	VPNEnabled = true
	AUTHEnabled = true
}

// createTestMessage creates a binary message with header and JSON data
func createTestMessage(header string, jsonData string) []byte {
	// Create 30-byte header (padded with null bytes if necessary)
	headerBytes := make([]byte, 30)
	copy(headerBytes, []byte(header))
	
	// Combine header and JSON data
	message := append(headerBytes, []byte(jsonData)...)
	return message
}

func TestProcessTCPMessage(t *testing.T) {
	setupTestEnvironment()
	
	// Define test cases for all routes in the switch statement
	testCases := []testCase{
		// Health check
		{
			name:     "health check",
			header:   "health",
			jsonData: `{}`,
		},
		
		// LAN API routes
		{
			name:            "firewall API",
			header:          "v3/firewall", 
			jsonData:        `{"action": "list"}`,
			requiredFeature: "LAN",
		},
		{
			name:            "list devices API",
			header:          "v3/devices",
			jsonData:        `{}`,
			requiredFeature: "LAN",
		},
		
		// VPN API routes
		{
			name:            "connect API",
			header:          "v3/connect",
			jsonData:        `{"username": "test", "password": "test"}`,
			requiredFeature: "VPN",
		},
		
		// Auth API routes - User management
		{
			name:            "user create API",
			header:          "v3/user/create",
			jsonData:        `{"username": "testuser", "password": "testpass", "email": "test@example.com"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "user update API",
			header:          "v3/user/update",
			jsonData:        `{"id": "123", "username": "updateduser"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "user login API",
			header:          "v3/user/login",
			jsonData:        `{"username": "testuser", "password": "testpass"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "user logout API",
			header:          "v3/user/logout",
			jsonData:        `{"session_id": "abc123"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "user reset code API",
			header:          "v3/user/reset/code",
			jsonData:        `{"email": "test@example.com"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "user reset password API",
			header:          "v3/user/reset/password",
			jsonData:        `{"code": "123456", "new_password": "newpass"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "user 2FA confirm API",
			header:          "v3/user/2fa/confirm",
			jsonData:        `{"user_id": "123", "code": "123456"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "user list API",
			header:          "v3/user/list",
			jsonData:        `{}`,
			requiredFeature: "AUTH",
		},
		
		// Device API routes
		{
			name:            "device list API",
			header:          "v3/device/list",
			jsonData:        `{"user_id": "123"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "device create API",
			header:          "v3/device/create",
			jsonData:        `{"name": "test-device", "user_id": "123"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "device delete API",
			header:          "v3/device/delete",
			jsonData:        `{"device_id": "456"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "device update API",
			header:          "v3/device/update",
			jsonData:        `{"device_id": "456", "name": "updated-device"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "device get API",
			header:          "v3/device",
			jsonData:        `{"device_id": "456"}`,
			requiredFeature: "AUTH",
		},
		
		// Group API routes
		{
			name:            "group create API",
			header:          "v3/group/create",
			jsonData:        `{"name": "test-group", "description": "Test group"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "group delete API",
			header:          "v3/group/delete",
			jsonData:        `{"group_id": "789"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "group update API",
			header:          "v3/group/update",
			jsonData:        `{"group_id": "789", "name": "updated-group"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "group add API",
			header:          "v3/group/add",
			jsonData:        `{"group_id": "789", "user_id": "123"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "group remove API",
			header:          "v3/group/remove",
			jsonData:        `{"group_id": "789", "user_id": "123"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "group list API",
			header:          "v3/group/list",
			jsonData:        `{}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "group get API",
			header:          "v3/group",
			jsonData:        `{"group_id": "789"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "group entities API",
			header:          "v3/group/entities",
			jsonData:        `{"group_id": "789"}`,
			requiredFeature: "AUTH",
		},
		
		// Server API routes
		{
			name:            "server get API",
			header:          "v3/server",
			jsonData:        `{"server_id": "server123"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "server create API",
			header:          "v3/server/create",
			jsonData:        `{"name": "test-server", "region": "us-east-1"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "server update API",
			header:          "v3/server/update",
			jsonData:        `{"server_id": "server123", "name": "updated-server"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "servers for user API",
			header:          "v3/servers",
			jsonData:        `{"user_id": "123"}`,
			requiredFeature: "AUTH",
		},
		{
			name:            "session create API",
			header:          "v3/session",
			jsonData:        `{"user_id": "123", "device_id": "456"}`,
			requiredFeature: "AUTH",
		},
		
		// Payment API routes (these require PayKey to be configured)
		{
			name:            "license key activate API",
			header:          "v3/key/activate",
			jsonData:        `{"license_key": "test-key-123"}`,
			requiredFeature: "PAY",
		},
		{
			name:            "user toggle sub status API",
			header:          "v3/user/toggle/substatus",
			jsonData:        `{"user_id": "123", "status": "active"}`,
			requiredFeature: "PAY",
		},
		
		// Unknown route test
		{
			name:        "unknown route",
			header:      "v3/unknown/route",
			jsonData:    `{}`,
			expectError: true,
		},
	}
	
	// Test cases with disabled features
	featureTestCases := []struct {
		name            string
		disableFeatures map[string]bool
		testCases       []testCase
	}{
		{
			name: "LAN disabled",
			disableFeatures: map[string]bool{"LAN": true},
			testCases: []testCase{
				{
					name:        "firewall API with LAN disabled",
					header:      "v3/firewall",
					jsonData:    `{}`,
					expectError: true,
				},
				{
					name:        "devices API with LAN disabled", 
					header:      "v3/devices",
					jsonData:    `{}`,
					expectError: true,
				},
			},
		},
		{
			name: "VPN disabled",
			disableFeatures: map[string]bool{"VPN": true},
			testCases: []testCase{
				{
					name:        "connect API with VPN disabled",
					header:      "v3/connect",
					jsonData:    `{}`,
					expectError: true,
				},
			},
		},
		{
			name: "AUTH disabled",
			disableFeatures: map[string]bool{"AUTH": true},
			testCases: []testCase{
				{
					name:        "user create API with AUTH disabled",
					header:      "v3/user/create",
					jsonData:    `{}`,
					expectError: true,
				},
			},
		},
	}
	
	// Test message too short
	t.Run("message too short", func(t *testing.T) {
		shortMessage := []byte("short")
		response := processTCPMessage(shortMessage)
		
		var result map[string]interface{}
		if err := json.Unmarshal(response, &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		
		if result["status"].(float64) != 400 {
			t.Errorf("Expected status 400, got %v", result["status"])
		}
		
		bodyStr := result["body"].(string)
		if !strings.Contains(bodyStr, "message too short") {
			t.Errorf("Expected error message about message being too short, got: %s", bodyStr)
		}
	})
	
	// Test all normal routes
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip payment tests unless we set up a mock PayKey
			if tc.requiredFeature == "PAY" {
				// For payment API tests, we need to test both with and without PayKey
				t.Run("without PayKey", func(t *testing.T) {
					message := createTestMessage(tc.header, tc.jsonData)
					response := processTCPMessage(message)
					
					var result map[string]interface{}
					if err := json.Unmarshal(response, &result); err != nil {
						t.Fatalf("Failed to unmarshal response: %v", err)
					}
					
					// Should return error when PayKey is not configured
					if result["status"].(float64) != 400 {
						t.Errorf("Expected status 400 when PayKey not configured, got %v", result["status"])
					}
					
					bodyStr := result["body"].(string)
					if !strings.Contains(bodyStr, "Payment API not enabled") {
						t.Errorf("Expected Payment API not enabled error, got: %s", bodyStr)
					}
				})
				return
			}
			
			message := createTestMessage(tc.header, tc.jsonData)
			response := processTCPMessage(message)
			
			if len(response) == 0 {
				t.Fatal("Expected non-empty response")
			}
			
			// Parse the response JSON
			var result map[string]interface{}
			if err := json.Unmarshal(response, &result); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			
			// Check if we expect an error
			if tc.expectError {
				if result["status"].(float64) == 200 {
					t.Errorf("Expected error status, got 200")
				}
			} else {
				// For successful calls, we should get some response
				// The actual API functions might return various status codes,
				// so we just verify we got a structured response
				if _, hasStatus := result["status"]; !hasStatus {
					t.Error("Response missing status field")
				}
				if _, hasBody := result["body"]; !hasBody {
					t.Error("Response missing body field")
				}
			}
		})
	}
	
	// Test feature-disabled scenarios
	for _, featureTest := range featureTestCases {
		t.Run(featureTest.name, func(t *testing.T) {
			// Temporarily disable features
			originalLAN := LANEnabled
			originalVPN := VPNEnabled
			originalAUTH := AUTHEnabled
			
			if featureTest.disableFeatures["LAN"] {
				LANEnabled = false
			}
			if featureTest.disableFeatures["VPN"] {
				VPNEnabled = false
			}
			if featureTest.disableFeatures["AUTH"] {
				AUTHEnabled = false
			}
			
			// Restore original values after test
			defer func() {
				LANEnabled = originalLAN
				VPNEnabled = originalVPN
				AUTHEnabled = originalAUTH
			}()
			
			for _, tc := range featureTest.testCases {
				t.Run(tc.name, func(t *testing.T) {
					message := createTestMessage(tc.header, tc.jsonData)
					response := processTCPMessage(message)
					
					var result map[string]interface{}
					if err := json.Unmarshal(response, &result); err != nil {
						t.Fatalf("Failed to unmarshal response: %v", err)
					}
					
					// Should return error when feature is disabled
					if result["status"].(float64) != 400 {
						t.Errorf("Expected status 400 when feature disabled, got %v", result["status"])
					}
					
					bodyStr := result["body"].(string)
					if !strings.Contains(bodyStr, "not enabled") {
						t.Errorf("Expected 'not enabled' error message, got: %s", bodyStr)
					}
				})
			}
		})
	}
}

// TestProcessTCPMessageEdgeCases tests edge cases and error conditions
func TestProcessTCPMessageEdgeCases(t *testing.T) {
	setupTestEnvironment()
	
	t.Run("empty header", func(t *testing.T) {
		message := createTestMessage("", `{}`)
		response := processTCPMessage(message)
		
		var result map[string]interface{}
		if err := json.Unmarshal(response, &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		
		// Empty header should be treated as unknown route
		if result["status"].(float64) != 400 {
			t.Errorf("Expected status 400 for empty header, got %v", result["status"])
		}
	})
	
	t.Run("header with whitespace", func(t *testing.T) {
		message := createTestMessage("  health  ", `{}`)
		response := processTCPMessage(message)
		
		var result map[string]interface{}
		if err := json.Unmarshal(response, &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		
		// Whitespace should be trimmed and work correctly
		if _, hasStatus := result["status"]; !hasStatus {
			t.Error("Expected valid response for trimmed header")
		}
	})
	
	t.Run("header with null bytes", func(t *testing.T) {
		headerBytes := make([]byte, 30)
		copy(headerBytes, []byte("health\x00\x00\x00"))
		message := append(headerBytes, []byte(`{}`)...)
		
		response := processTCPMessage(message)
		
		var result map[string]interface{}
		if err := json.Unmarshal(response, &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		
		// Null bytes should be trimmed and work correctly
		if _, hasStatus := result["status"]; !hasStatus {
			t.Error("Expected valid response for header with null bytes")
		}
	})
	
	t.Run("invalid JSON data", func(t *testing.T) {
		message := createTestMessage("health", `{invalid json}`)
		response := processTCPMessage(message)
		
		var result map[string]interface{}
		if err := json.Unmarshal(response, &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		
		// The API handler might handle invalid JSON differently,
		// but we should still get a structured response
		if _, hasStatus := result["status"]; !hasStatus {
			t.Error("Expected response with status field even for invalid JSON")
		}
	})
	
	t.Run("very long header", func(t *testing.T) {
		longHeader := strings.Repeat("a", 50) // Longer than 30 bytes
		message := createTestMessage(longHeader, `{}`)
		response := processTCPMessage(message)
		
		var result map[string]interface{}
		if err := json.Unmarshal(response, &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		
		// Long header should be truncated to 30 bytes and treated as unknown
		if result["status"].(float64) != 400 {
			t.Errorf("Expected status 400 for unknown route (truncated header), got %v", result["status"])
		}
	})
}

// BenchmarkProcessTCPMessage benchmarks the processTCPMessage function
func BenchmarkProcessTCPMessage(b *testing.B) {
	setupTestEnvironment()
	
	message := createTestMessage("health", `{}`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processTCPMessage(message)
	}
}
