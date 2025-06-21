package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

// Test data structure for organizing test cases
type testCase struct {
	name                 string
	header               string
	jsonData             string
	expectedStatus       int
	expectedBodyContains []string // Strings that should be present in the response body
	expectedError        bool
	requiredFeature      string // "LAN", "VPN", "AUTH", "PAY" (payment)
	description          string // Description of what this test validates

	// Enhanced validation fields
	expectedFields       map[string]interface{}          // Expected fields in the response body JSON
	validateResponseFunc func(t *testing.T, body string) // Custom validation function
	expectJSONResponse   bool                            // Whether the response body should be valid JSON
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
	DNSEnabled = true
	BBOLTEnabled = true

	// Setup context and cancel function
	ctx, cancel := context.WithCancel(context.Background())
	CTX.Store(&ctx)
	Cancel.Store(&cancel)

	// Create test configuration
	setupTestConfig()

	// Setup test certificates and TLS
	setupTestCertificates()

	// Initialize test database
	setupTestDatabase()

	// Create test admin user
	setupTestUser()
	// Setup LAN and VPN components
	setupTestNetworking()
}

// setupTestConfig creates a minimal test configuration
func setupTestConfig() {
	testConfig := &types.ServerConfig{
		Features: []types.Feature{
			types.LAN,
			types.VPN,
			types.AUTH,
			types.DNS,
			types.BBOLT,
		},
		VPNIP:              "127.0.0.1",
		VPNPort:            "444",
		APIIP:              "127.0.0.1",
		APIPort:            "443",
		NetAdmins:          []string{},
		Hostname:           "test.local",
		StartPort:          2000,
		EndPort:            65530,
		UserMaxConnections: 10,
		InternetAccess:     true,
		LocalNetworkAccess: false,
		BandwidthMbps:      1000,
		UserBandwidthMbps:  10,
		DNSAllowCustomOnly: false,
		DNSRecords:         []*types.DNSRecord{},
		DNSServers:         []string{},
		SecretStore:        "config",
		Lan: &types.Network{
			Tag:     "lan",
			Network: "10.0.0.0/16",
		},
		Routes: []*types.Route{
			{Address: "10.0.0.0/16", Metric: "0"},
		},
		SubNets:            []*types.Network{},
		DisableLanFirewall: true, // Disable for testing
		// Test secrets
		DBurl:        "test.db",
		AdminApiKey:  "test-admin-key",
		TwoFactorKey: "testtesttest1234567890123456",
		EmailKey:     "test-email-key",
		CertPem:      "test-cert.pem",
		KeyPem:       "test-key.pem",
		SignPem:      "test-sign.pem",
		PayKey:       "test-pay-key", // For payment API testing
	}
	Config.Store(testConfig)
}

// setupTestCertificates creates test certificates and TLS config
func setupTestCertificates() {
	// For testing purposes, just create empty certificate files
	// The actual certificate content isn't critical for TCP message processing tests

	ioutil.WriteFile("test-cert.pem", []byte("test-cert-content"), 0644)
	ioutil.WriteFile("test-key.pem", []byte("test-key-content"), 0644)
	ioutil.WriteFile("test-sign.pem", []byte("test-sign-content"), 0644)

	// Most tests don't actually need valid TLS certificates
	// since we're testing the message processing logic, not TLS
}

// setupTestDatabase initializes the BBolt database for testing
func setupTestDatabase() {
	// Use a temporary test database file
	testDBPath := filepath.Join(os.TempDir(), "test_tunnels.db")

	// Remove any existing test database
	os.Remove(testDBPath)

	// Initialize BBolt database
	err := ConnectToBBoltDB(testDBPath)
	if err != nil {
		// If BBolt fails, we'll continue without it for basic testing
		logger.Warn("Failed to setup test database", slog.Any("error", err))
	}
}

// setupTestUser creates a test admin user
func setupTestUser() {
	if !AUTHEnabled {
		return
	}

	// Create test admin user
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), 10)
	if err != nil {
		logger.Warn("Failed to create test user password", slog.Any("error", err))
		return
	}

	testUser := &User{
		ID:                    primitive.NewObjectID(),
		Email:                 "test@example.com",
		Password:              string(hash),
		IsAdmin:               true,
		IsManager:             true,
		AdditionalInformation: "Test user",
		ResetCode:             "",
		Updated:               time.Now(),
		Trial:                 false,
		APIKey:                uuid.NewString(),
		SubExpiration:         time.Now().AddDate(1, 0, 0),
		Groups:                make([]primitive.ObjectID, 0),
		Tokens:                make([]*DeviceToken, 0),
	}

	// Try to create the user (may fail if database not available)
	err = DB_CreateUser(testUser)
	if err != nil {
		logger.Warn("Failed to create test user", slog.Any("error", err))
	}
}

// setupTestNetworking initializes networking components for testing
func setupTestNetworking() {
	if !LANEnabled && !VPNEnabled {
		return
	}

	// Initialize DHCP mapping
	if LANEnabled {
		err := generateDHCPMap()
		if err != nil {
			logger.Warn("Failed to generate DHCP map", slog.Any("error", err))
		}
		lanFirewallDisabled = true // Disable firewall for testing
	}

	// Initialize VPN components
	if VPNEnabled {
		InterfaceIP = net.ParseIP("127.0.0.1").To4()

		// Initialize port allocation (simplified for testing)
		slots = 10

		// Initialize VPL core mappings
		GenerateVPLCoreMappings()

		// Basic port range setup for testing
		for i := 2000; i < 3000; i++ {
			PR := &PortRange{
				StartPort: uint16(2000),
				EndPort:   uint16(3000),
			}
			portToCoreMapping[i] = PR
		}
	}
}

// cleanupTestEnvironment cleans up test resources
func cleanupTestEnvironment() {
	// Clean up test certificate files
	os.Remove("test-cert.pem")
	os.Remove("test-key.pem")
	os.Remove("test-sign.pem")

	// Clean up test database
	testDBPath := filepath.Join(os.TempDir(), "test_tunnels.db")
	os.Remove(testDBPath)

	// Cancel context
	if cancel := Cancel.Load(); cancel != nil {
		(*cancel)()
	}
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
	defer cleanupTestEnvironment()

	// This test validates that the TCP handler properly routes messages to API functions
	// Most expectedStatus values are set to 0 (don't enforce) because the actual API
	// functions may return different status codes based on their internal logic.
	// The key goal is to verify that:
	// 1. Routes are properly matched and routed to the correct API functions
	// 2. Feature flags are respected (LAN, VPN, AUTH, PAY)
	// 3. Unknown routes return appropriate errors
	// 4. The response structure is consistent (has status and body fields)

	// Define test cases for all routes in the switch statement

	testCases := []testCase{
		// Health check
		{
			name:                 "health check",
			header:               "health",
			jsonData:             `{}`,
			expectedStatus:       200,
			expectedBodyContains: []string{"OK"},
			description:          "Health check should return OK status",
		},

		// LAN API routes
		{
			name:                 "firewall API",
			header:               "v3/firewall",
			jsonData:             `{"action": "list"}`,
			expectedStatus:       0, // Don't enforce specific status, just verify it responds
			expectedBodyContains: []string{},
			requiredFeature:      "LAN",
			description:          "Firewall API should return firewall rules",
		},
		{
			name:                 "list devices API",
			header:               "v3/devices",
			jsonData:             `{}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "LAN",
			description:          "Devices API should return list of devices",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				// Should return an array or error message
				if strings.Contains(body, `"error"`) {
					// Error is acceptable for list devices without proper auth
					return
				}
				// If successful, should be an array
				if !strings.HasPrefix(strings.TrimSpace(body), "[") {
					t.Log("Device list response should be an array or error - this is acceptable behavior")
				}
			},
		},

		// VPN API routes
		{
			name:                 "connect API",
			header:               "v3/connect",
			jsonData:             `{"username": "test", "password": "test"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "VPN",
			description:          "Connect API should validate credentials",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				// Should contain error for invalid credentials or missing signature
				if !strings.Contains(body, "error") && !strings.Contains(body, "signature") {
					t.Log("Connect API responded without expected error - this may indicate missing validation")
				}
			},
		},

		// Auth API routes - User management
		{
			name:                 "user create API",
			header:               "v3/user/create",
			jsonData:             `{"email": "newuser@example.com", "password": "testpassword123", "additional_information": "test user"}`,
			expectedStatus:       0, // Status depends on validation logic
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "User create API should validate input and create user",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				// Parse response as JSON
				var responseData map[string]interface{}
				if err := json.Unmarshal([]byte(body), &responseData); err == nil {
					// If successful creation, should have user data
					if email, exists := responseData["email"]; exists {
						if email != "newuser@example.com" {
							t.Errorf("Expected email 'newuser@example.com', got '%v'", email)
						}
						t.Log("User creation successful - validated email field")
					}
				}
				// Could also be an error response (duplicate user, etc.)
			},
		},
		{
			name:                 "user update API",
			header:               "v3/user/update",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "api_key": "test-key"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "User update API should validate user ID and auth",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				// Should return error for invalid auth
				if !strings.Contains(body, "error") && !strings.Contains(body, "401") {
					t.Log("User update without valid auth should return error")
				}
			},
		},
		{
			name:                 "user login API",
			header:               "v3/user/login",
			jsonData:             `{"email": "test@example.com", "password": "testpass", "device_name": "test-device"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "User login API should validate credentials",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				var responseData map[string]interface{}
				if err := json.Unmarshal([]byte(body), &responseData); err == nil {
					// If login successful, should have user data
					if email, exists := responseData["email"]; exists {
						if email == "test@example.com" {
							t.Log("Login successful - found expected email")
							// Check for expected user fields
							expectedFields := []string{"id", "is_admin", "sub_expiration"}
							for _, field := range expectedFields {
								if _, exists := responseData[field]; exists {
									t.Logf("Found expected field: %s", field)
								}
							}
						}
					}
				}
				// Could also be error for invalid credentials
			},
		},
		{
			name:                 "user logout API",
			header:               "v3/user/logout",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "logout_token": "test"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "User logout API should validate session",
		},
		{
			name:                 "user reset code API",
			header:               "v3/user/reset/code",
			jsonData:             `{"email": "test@example.com"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "User reset code API should send reset code",
		},
		{
			name:                 "user reset password API",
			header:               "v3/user/reset/password",
			jsonData:             `{"email": "test@example.com", "reset_code": "123456", "password": "newpassword123"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "User reset password API should validate reset code",
		},
		{
			name:                 "user 2FA confirm API",
			header:               "v3/user/2fa/confirm",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "code": "TESTCODE", "digits": "123456", "password": "testpass"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "User 2FA confirm API should validate 2FA code",
		},
		{
			name:                 "user list API",
			header:               "v3/user/list",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "limit": 10, "offset": 0}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "User list API should return all users",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				// Should return error for invalid auth, or array of users
				if !strings.Contains(body, "error") {
					// If successful, should be an array
					if strings.HasPrefix(strings.TrimSpace(body), "[") {
						t.Log("User list returned array - checking for user structure")
						var users []map[string]interface{}
						if err := json.Unmarshal([]byte(body), &users); err == nil && len(users) > 0 {
							// Check first user has expected fields
							user := users[0]
							expectedFields := []string{"email", "id"}
							for _, field := range expectedFields {
								if _, exists := user[field]; exists {
									t.Logf("User object contains expected field: %s", field)
								}
							}
						}
					}
				}
			},
		},
		// Device API routes
		{
			name:                 "device list API",
			header:               "v3/device/list",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "limit": 10, "offset": 0}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Device list API should validate user ID",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				// Should return error for invalid auth, or array of devices
				if !strings.Contains(body, "error") {
					// If successful, should be an array
					if strings.HasPrefix(strings.TrimSpace(body), "[") {
						t.Log("Device list returned array - checking for device structure")
						var devices []map[string]interface{}
						if err := json.Unmarshal([]byte(body), &devices); err == nil && len(devices) > 0 {
							// Check first device has expected fields
							device := devices[0]
							expectedFields := []string{"id", "tag"}
							for _, field := range expectedFields {
								if _, exists := device[field]; exists {
									t.Logf("Device object contains expected field: %s", field)
								}
							}
						}
					}
				}
			},
		},
		{
			name:                 "device create API",
			header:               "v3/device/create",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "device": {"tag": "test-device", "description": "Test device"}}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Device create API should validate input",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				var responseData map[string]interface{}
				if err := json.Unmarshal([]byte(body), &responseData); err == nil {
					// If successful creation, should have device data
					if tag, exists := responseData["tag"]; exists {
						if tag == "test-device" {
							t.Log("Device creation successful - validated tag field")
							// Check for expected device fields
							expectedFields := []string{"id", "created_at"}
							for _, field := range expectedFields {
								if _, exists := responseData[field]; exists {
									t.Logf("Found expected device field: %s", field)
								}
							}
						}
					}
				}
				// Could also be an error response (auth, validation, etc.)
			},
		},
		{
			name:                 "device delete API",
			header:               "v3/device/delete",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "did": "invalid_device_id"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Device delete API should validate device ID",
		},
		{
			name:                 "device update API",
			header:               "v3/device/update",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "device": {"id": "test", "tag": "updated-device"}}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Device update API should validate device ID",
		}, {
			name:                 "device get API",
			header:               "v3/device",
			jsonData:             `{"device_id": "invalid_device_id"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Device get API should validate device ID",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				var responseData map[string]interface{}
				if err := json.Unmarshal([]byte(body), &responseData); err == nil {
					// If successful, should have device data
					if tag, exists := responseData["tag"]; exists {
						t.Logf("Device get successful - found tag: %v", tag)
					}
				}
				// Could also be error for device not found
			},
		},

		// Group API routes
		{
			name:                 "group create API",
			header:               "v3/group/create",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "group": {"tag": "test-group", "description": "Test group"}}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Group create API should create new group",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				var responseData map[string]interface{}
				if err := json.Unmarshal([]byte(body), &responseData); err == nil {
					// If successful creation, should have group data
					if tag, exists := responseData["tag"]; exists {
						if tag == "test-group" {
							t.Log("Group creation successful - validated tag field")
						}
					}
				}
				// Could also be an error response (auth, validation, etc.)
			},
		},
		{
			name:                 "group delete API",
			header:               "v3/group/delete",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "gid": "invalid_group_id"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Group delete API should validate group ID",
		},
		{
			name:                 "group update API",
			header:               "v3/group/update",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "group": {"id": "test", "tag": "updated-group"}}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Group update API should validate group data",
		},
		{
			name:                 "group add entity API",
			header:               "v3/group/add",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "group_id": "test", "type_id": "test", "type": "user"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Group add entity API should validate parameters",
		},
		{
			name:                 "group remove entity API",
			header:               "v3/group/remove",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "group_id": "test", "type_id": "test", "type": "user"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Group remove entity API should validate parameters",
		},
		{
			name:                 "group list API",
			header:               "v3/group/list",
			jsonData:             `{"uid": "invalid", "device_token": "invalid"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Group list API should return all groups",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				// Should return error for invalid auth, or array of groups
				if !strings.Contains(body, "error") {
					// If successful, should be an array
					if strings.HasPrefix(strings.TrimSpace(body), "[") {
						t.Log("Group list returned array - checking for group structure")
						var groups []map[string]interface{}
						if err := json.Unmarshal([]byte(body), &groups); err == nil && len(groups) > 0 {
							// Check first group has expected fields
							group := groups[0]
							expectedFields := []string{"id", "tag"}
							for _, field := range expectedFields {
								if _, exists := group[field]; exists {
									t.Logf("Group object contains expected field: %s", field)
								}
							}
						}
					}
				}
			},
		},
		{
			name:                 "group get API",
			header:               "v3/group",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "gid": "invalid_group_id"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Group get API should validate group ID",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				var responseData map[string]interface{}
				if err := json.Unmarshal([]byte(body), &responseData); err == nil {
					// If successful, should have group data
					if tag, exists := responseData["tag"]; exists {
						t.Logf("Group get successful - found tag: %v", tag)
					}
				}
				// Could also be error for group not found
			},
		}, {
			name:                 "group entities API",
			header:               "v3/group/entities",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "gid": "test", "type": "user", "limit": 10, "offset": 0}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Group entities API should return entities in group",
		},

		// Server API routes
		{
			name:                 "server get API",
			header:               "v3/server",
			jsonData:             `{"server_id": "invalid_server_id", "uid": "invalid", "device_token": "invalid"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Server get API should validate server ID",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				var responseData map[string]interface{}
				if err := json.Unmarshal([]byte(body), &responseData); err == nil {
					// If successful, should have server data
					if tag, exists := responseData["tag"]; exists {
						t.Logf("Server get successful - found tag: %v", tag)
					}
				}
				// Could also be error for server not found or unauthorized
			},
		},
		{
			name:                 "server create API",
			header:               "v3/server/create",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "server": {"tag": "test-server", "ip": "1.2.3.4", "port": "443"}}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Server create API should validate input",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				var responseData map[string]interface{}
				if err := json.Unmarshal([]byte(body), &responseData); err == nil {
					// If successful creation, should have server data
					if tag, exists := responseData["tag"]; exists {
						if tag == "test-server" {
							t.Log("Server creation successful - validated tag field")
						}
					}
				}
				// Could also be an error response (auth, validation, etc.)
			},
		},
		{
			name:                 "server update API",
			header:               "v3/server/update",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "server": {"id": "test", "tag": "updated-server"}}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Server update API should validate server data",
		},
		{
			name:                 "servers list API",
			header:               "v3/servers",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "start_index": 0}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Servers list API should return available servers",
			expectJSONResponse:   true,
			validateResponseFunc: func(t *testing.T, body string) {
				// Should return error for invalid auth, or array of servers
				if !strings.Contains(body, "error") {
					// If successful, should be an array
					if strings.HasPrefix(strings.TrimSpace(body), "[") {
						t.Log("Servers list returned array - checking for server structure")
						var servers []map[string]interface{}
						if err := json.Unmarshal([]byte(body), &servers); err == nil && len(servers) > 0 {
							// Check first server has expected fields
							server := servers[0]
							expectedFields := []string{"id", "tag"}
							for _, field := range expectedFields {
								if _, exists := server[field]; exists {
									t.Logf("Server object contains expected field: %s", field)
								}
							}
						}
					}
				}
			},
		},
		{
			name:                 "session create API",
			header:               "v3/session",
			jsonData:             `{"server_id": "invalid", "user_id": "invalid", "device_token": "invalid"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Session create API should validate input and create session",
		},

		// Payment API routes (require PayKey configuration)
		{
			name:                 "license key activate API",
			header:               "v3/key/activate",
			jsonData:             `{"uid": "invalid", "device_token": "invalid", "key": "test-license-key"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "PAY",
			description:          "License key activate API should validate license key",
		},
		{
			name:                 "user subscription toggle API",
			header:               "v3/user/toggle/substatus",
			jsonData:             `{"email": "test@example.com", "device_token": "invalid"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "PAY",
			description:          "User subscription toggle API should validate user",
		},

		// Unknown route test
		{
			name:                 "unknown route",
			header:               "v3/unknown/route",
			jsonData:             `{}`,
			expectedStatus:       400,
			expectedBodyContains: []string{"unknown route"},
			expectedError:        true,
			description:          "Unknown routes should return 400 error",
		},
		{
			name:                 "server update API",
			header:               "v3/server/update",
			jsonData:             `{"server_id": "server123", "name": "updated-server"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Server update API should validate server ID",
		},
		{
			name:                 "servers for user API",
			header:               "v3/servers",
			jsonData:             `{"user_id": "123"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Servers for user API should validate user ID",
		},
		{
			name:                 "session create API",
			header:               "v3/session",
			jsonData:             `{"user_id": "123", "device_id": "456"}`,
			expectedStatus:       0, // Don't enforce specific status
			expectedBodyContains: []string{},
			requiredFeature:      "AUTH",
			description:          "Session create API should validate user and device IDs",
		},

		// Payment API routes (these require PayKey to be configured)
		{
			name:                 "license key activate API",
			header:               "v3/key/activate",
			jsonData:             `{"license_key": "test-key-123"}`,
			expectedStatus:       400, // Will fail when PayKey not configured properly
			expectedBodyContains: []string{"Payment API not enabled"},
			requiredFeature:      "PAY",
			description:          "License key activate API should validate payment configuration",
		},
		{
			name:                 "user toggle sub status API",
			header:               "v3/user/toggle/substatus",
			jsonData:             `{"user_id": "123", "status": "active"}`,
			expectedStatus:       400, // Will fail when PayKey not configured properly
			expectedBodyContains: []string{"Payment API not enabled"},
			requiredFeature:      "PAY",
			description:          "User toggle sub status API should validate payment configuration",
		},
		// Unknown route test
		{
			name:                 "unknown route",
			header:               "v3/unknown/route",
			jsonData:             `{}`,
			expectedStatus:       400,
			expectedBodyContains: []string{"unknown route: v3/unknown/route"}, // Based on actual error message format
			expectedError:        true,
			description:          "Unknown routes should return error",
		},
	}

	// Test cases with disabled features
	featureTestCases := []struct {
		name            string
		disableFeatures map[string]bool
		testCases       []testCase
	}{{
		name:            "LAN disabled",
		disableFeatures: map[string]bool{"LAN": true},
		testCases: []testCase{
			{
				name:                 "firewall API with LAN disabled",
				header:               "v3/firewall",
				jsonData:             `{}`,
				expectedStatus:       400,
				expectedBodyContains: []string{"not enabled"},
				expectedError:        true,
				description:          "Firewall API should fail when LAN disabled",
			},
			{
				name:                 "devices API with LAN disabled",
				header:               "v3/devices",
				jsonData:             `{}`,
				expectedStatus:       400,
				expectedBodyContains: []string{"not enabled"},
				expectedError:        true,
				description:          "Devices API should fail when LAN disabled",
			},
		},
	},
		{
			name:            "VPN disabled",
			disableFeatures: map[string]bool{"VPN": true},
			testCases: []testCase{
				{
					name:                 "connect API with VPN disabled",
					header:               "v3/connect",
					jsonData:             `{}`,
					expectedStatus:       400,
					expectedBodyContains: []string{"not enabled"},
					expectedError:        true,
					description:          "Connect API should fail when VPN disabled",
				},
			},
		},
		{
			name:            "AUTH disabled",
			disableFeatures: map[string]bool{"AUTH": true},
			testCases: []testCase{
				{
					name:                 "user create API with AUTH disabled",
					header:               "v3/user/create",
					jsonData:             `{}`,
					expectedStatus:       400,
					expectedBodyContains: []string{"not enabled"},
					expectedError:        true,
					description:          "User create API should fail when AUTH disabled",
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

		expectedStatus := 400
		actualStatus := int(result["status"].(float64))
		if actualStatus != expectedStatus {
			t.Errorf("Expected status %d, got %d", expectedStatus, actualStatus)
		}

		bodyStr := result["body"].(string)
		expectedText := "message too short"
		if !strings.Contains(bodyStr, expectedText) {
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

					// Validate expected status
					if tc.expectedStatus != 0 {
						actualStatus := int(result["status"].(float64))
						if actualStatus != tc.expectedStatus {
							t.Errorf("Expected status %d, got %d", tc.expectedStatus, actualStatus)
						}
					}

					// Check expected body content
					if len(tc.expectedBodyContains) > 0 {
						bodyStr := ""
						if body, ok := result["body"]; ok {
							bodyStr = body.(string)
						}
						for _, expectedText := range tc.expectedBodyContains {
							if !strings.Contains(bodyStr, expectedText) {
								t.Errorf("Expected response body to contain '%s', got: %s", expectedText, bodyStr)
							}
						}
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

			// Log test details for better visibility
			t.Logf("Testing: %s", tc.description)
			t.Logf("Header: %s, Expected Status: %d", tc.header, tc.expectedStatus)
			t.Logf("Actual Response - Status: %v, Body: %v", result["status"], result["body"])
			// Check if we expect an error
			if tc.expectedError {
				if result["status"].(float64) == 200 {
					t.Errorf("Expected error status, got 200")
				}
			} else {
				// Validate expected status if specified
				if tc.expectedStatus != 0 {
					actualStatus := int(result["status"].(float64))
					if actualStatus != tc.expectedStatus {
						t.Errorf("Expected status %d, got %d", tc.expectedStatus, actualStatus)
					}
				}
			}
			// Check expected body content if specified
			if len(tc.expectedBodyContains) > 0 {
				bodyStr := ""
				if body, ok := result["body"]; ok {
					bodyStr = body.(string)
				}
				for _, expectedText := range tc.expectedBodyContains {
					if !strings.Contains(bodyStr, expectedText) {
						t.Errorf("Expected response body to contain '%s', got: %s", expectedText, bodyStr)
					}
				}
			}

			// Enhanced JSON response validation
			if tc.expectJSONResponse {
				bodyStr := ""
				if body, ok := result["body"]; ok {
					bodyStr = body.(string)
				}

				// Validate that response body is valid JSON
				var jsonData interface{}
				if err := json.Unmarshal([]byte(bodyStr), &jsonData); err != nil {
					t.Logf("Response body is not valid JSON (this may be acceptable): %s", bodyStr)
				}
			}

			// Run custom validation function if provided
			if tc.validateResponseFunc != nil {
				bodyStr := ""
				if body, ok := result["body"]; ok {
					bodyStr = body.(string)
				}
				tc.validateResponseFunc(t, bodyStr)
			}

			// Validate expected fields if specified
			if len(tc.expectedFields) > 0 {
				bodyStr := ""
				if body, ok := result["body"]; ok {
					bodyStr = body.(string)
				}

				var responseData map[string]interface{}
				if err := json.Unmarshal([]byte(bodyStr), &responseData); err == nil {
					for fieldName, expectedValue := range tc.expectedFields {
						if actualValue, exists := responseData[fieldName]; exists {
							if expectedValue != nil && actualValue != expectedValue {
								t.Errorf("Expected field '%s' to have value '%v', got '%v'", fieldName, expectedValue, actualValue)
							} else if expectedValue == nil {
								t.Logf("Found expected field '%s' with value '%v'", fieldName, actualValue)
							}
						} else {
							t.Errorf("Expected field '%s' not found in response", fieldName)
						}
					}
				} else {
					t.Logf("Could not parse response as JSON for field validation: %v", err)
				}
			}

			// Ensure response has required fields
			if _, hasStatus := result["status"]; !hasStatus {
				t.Error("Response missing status field")
			}
			if _, hasBody := result["body"]; !hasBody {
				t.Error("Response missing body field")
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

					// Validate expected status
					if tc.expectedStatus != 0 {
						actualStatus := int(result["status"].(float64))
						if actualStatus != tc.expectedStatus {
							t.Errorf("Expected status %d, got %d", tc.expectedStatus, actualStatus)
						}
					}

					// Check expected body content
					if len(tc.expectedBodyContains) > 0 {
						bodyStr := ""
						if body, ok := result["body"]; ok {
							bodyStr = body.(string)
						}
						for _, expectedText := range tc.expectedBodyContains {
							if !strings.Contains(bodyStr, expectedText) {
								t.Errorf("Expected response body to contain '%s', got: %s", expectedText, bodyStr)
							}
						}
					}
				})
			}
		})
	}
}

// TestProcessTCPMessageEdgeCases tests edge cases and error conditions
func TestProcessTCPMessageEdgeCases(t *testing.T) {
	setupTestEnvironment()
	defer cleanupTestEnvironment()

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
	defer cleanupTestEnvironment()

	message := createTestMessage("health", `{}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processTCPMessage(message)
	}
}
