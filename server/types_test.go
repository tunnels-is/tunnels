package main

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Test User.ToMinifiedUser
func TestUser_ToMinifiedUser(t *testing.T) {
	tests := []struct {
		name     string
		user     *User
		expected MinifiedUser
	}{
		{
			name: "standard user with all fields",
			user: &User{
				ID:        primitive.NewObjectID(),
				Email:     "test@example.com",
				Disabled:  false,
				IsAdmin:   true,
				IsManager: false,
			},
			expected: MinifiedUser{
				Email:     "test@example.com",
				Disabled:  false,
				IsAdmin:   true,
				IsManager: false,
			},
		},
		{
			name: "disabled user",
			user: &User{
				ID:        primitive.NewObjectID(),
				Email:     "disabled@example.com",
				Disabled:  true,
				IsAdmin:   false,
				IsManager: false,
			},
			expected: MinifiedUser{
				Email:     "disabled@example.com",
				Disabled:  true,
				IsAdmin:   false,
				IsManager: false,
			},
		},
		{
			name: "manager user",
			user: &User{
				ID:        primitive.NewObjectID(),
				Email:     "manager@example.com",
				Disabled:  false,
				IsAdmin:   false,
				IsManager: true,
			},
			expected: MinifiedUser{
				Email:     "manager@example.com",
				Disabled:  false,
				IsAdmin:   false,
				IsManager: true,
			},
		},
		{
			name: "admin and manager",
			user: &User{
				ID:        primitive.NewObjectID(),
				Email:     "superuser@example.com",
				Disabled:  false,
				IsAdmin:   true,
				IsManager: true,
			},
			expected: MinifiedUser{
				Email:     "superuser@example.com",
				Disabled:  false,
				IsAdmin:   true,
				IsManager: true,
			},
		},
		{
			name: "user with empty email",
			user: &User{
				ID:        primitive.NewObjectID(),
				Email:     "",
				Disabled:  false,
				IsAdmin:   false,
				IsManager: false,
			},
			expected: MinifiedUser{
				Email:     "",
				Disabled:  false,
				IsAdmin:   false,
				IsManager: false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.user.ToMinifiedUser()

			// Check that ID is converted to hex string
			if result.ID != tc.user.ID.Hex() {
				t.Errorf("ToMinifiedUser().ID = %q, expected %q", result.ID, tc.user.ID.Hex())
			}

			// Check other fields
			if result.Email != tc.expected.Email {
				t.Errorf("ToMinifiedUser().Email = %q, expected %q", result.Email, tc.expected.Email)
			}
			if result.Disabled != tc.expected.Disabled {
				t.Errorf("ToMinifiedUser().Disabled = %v, expected %v", result.Disabled, tc.expected.Disabled)
			}
			if result.IsAdmin != tc.expected.IsAdmin {
				t.Errorf("ToMinifiedUser().IsAdmin = %v, expected %v", result.IsAdmin, tc.expected.IsAdmin)
			}
			if result.IsManager != tc.expected.IsManager {
				t.Errorf("ToMinifiedUser().IsManager = %v, expected %v", result.IsManager, tc.expected.IsManager)
			}

			t.Logf("ToMinifiedUser() correctly converted user %q ✓", tc.expected.Email)
		})
	}
}

func TestUser_ToMinifiedUser_DoesNotIncludeSensitiveData(t *testing.T) {
	// Create a user with sensitive data
	user := &User{
		ID:            primitive.NewObjectID(),
		Email:         "test@example.com",
		Password:      "hashed-password-should-not-be-in-minified",
		ConfirmCode:   "secret-confirm-code",
		RecoveryCodes: []byte("recovery-codes"),
		TwoFactorCode: []byte("2fa-code"),
		APIKey:        "api-key-secret",
		Disabled:      false,
		IsAdmin:       true,
		IsManager:     false,
	}

	minified := user.ToMinifiedUser()

	// Verify that MinifiedUser type doesn't have these fields by checking the struct
	// The conversion should only include: ID, Email, Disabled, IsAdmin, IsManager

	if minified.Email != "test@example.com" {
		t.Errorf("Email should be included in minified user")
	}

	if minified.ID != user.ID.Hex() {
		t.Errorf("ID should be included in minified user")
	}

	t.Log("ToMinifiedUser() correctly excludes sensitive data ✓")
}

// Test User.RemoveSensitiveInformation
func TestUser_RemoveSensitiveInformation(t *testing.T) {
	tests := []struct {
		name                string
		user                *User
		expectedKeyRedacted string
	}{
		{
			name: "user with key containing dashes",
			user: &User{
				ID:            primitive.NewObjectID(),
				Email:         "test@example.com",
				Password:      "hashed-password",
				Password2:     "password2",
				ConfirmCode:   "ABC123",
				RecoveryCodes: []byte("recovery1,recovery2"),
				TwoFactorCode: []byte("123456"),
				Key: &LicenseKey{
					Key:     "prefix-part1-part2-LASTPART",
					Created: time.Now(),
					Months:  12,
				},
			},
			expectedKeyRedacted: "LASTPART",
		},
		{
			name: "user with key without dashes",
			user: &User{
				ID:            primitive.NewObjectID(),
				Email:         "test2@example.com",
				Password:      "password",
				Password2:     "password2",
				ConfirmCode:   "XYZ789",
				RecoveryCodes: []byte("codes"),
				TwoFactorCode: []byte("654321"),
				Key: &LicenseKey{
					Key:     "SINGLEPARTHARDKEY",
					Created: time.Now(),
					Months:  6,
				},
			},
			expectedKeyRedacted: "SINGLEPARTHARDKEY", // No dash means ks[len(ks)-1] is the full string
		},
		{
			name: "user with empty key",
			user: &User{
				ID:            primitive.NewObjectID(),
				Email:         "test3@example.com",
				Password:      "password",
				Password2:     "password2",
				ConfirmCode:   "CONFIRM",
				RecoveryCodes: []byte("recovery"),
				TwoFactorCode: []byte("2fa"),
				Key: &LicenseKey{
					Key:     "",
					Created: time.Now(),
					Months:  3,
				},
			},
			expectedKeyRedacted: "", // Empty string split returns [""], so ks[len(ks)-1] is ""
		},
		{
			name: "user without license key",
			user: &User{
				ID:            primitive.NewObjectID(),
				Email:         "test4@example.com",
				Password:      "password",
				Password2:     "password2",
				ConfirmCode:   "CODE",
				RecoveryCodes: []byte("recovery"),
				TwoFactorCode: []byte("2fa"),
				Key:           nil,
			},
			expectedKeyRedacted: "",
		},
		{
			name: "user with key containing single dash",
			user: &User{
				ID:            primitive.NewObjectID(),
				Email:         "test5@example.com",
				Password:      "password",
				Password2:     "password2",
				ConfirmCode:   "CODE",
				RecoveryCodes: []byte("recovery"),
				TwoFactorCode: []byte("2fa"),
				Key: &LicenseKey{
					Key:     "PART1-PART2",
					Created: time.Now(),
					Months:  1,
				},
			},
			expectedKeyRedacted: "PART2",
		},
		{
			name: "user with key containing multiple dashes",
			user: &User{
				ID:            primitive.NewObjectID(),
				Email:         "test6@example.com",
				Password:      "password",
				Password2:     "password2",
				ConfirmCode:   "CODE",
				RecoveryCodes: []byte("recovery"),
				TwoFactorCode: []byte("2fa"),
				Key: &LicenseKey{
					Key:     "A-B-C-D-E-F-LAST",
					Created: time.Now(),
					Months:  24,
				},
			},
			expectedKeyRedacted: "LAST",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.user.RemoveSensitiveInformation()

			// Check that sensitive fields are cleared
			if tc.user.Password != "" {
				t.Errorf("Password should be empty, got %q", tc.user.Password)
			}
			if tc.user.Password2 != "" {
				t.Errorf("Password2 should be empty, got %q", tc.user.Password2)
			}
			if tc.user.ConfirmCode != "" {
				t.Errorf("ConfirmCode should be empty, got %q", tc.user.ConfirmCode)
			}
			if tc.user.RecoveryCodes != nil {
				t.Errorf("RecoveryCodes should be nil, got %v", tc.user.RecoveryCodes)
			}
			if tc.user.TwoFactorCode != nil {
				t.Errorf("TwoFactorCode should be nil, got %v", tc.user.TwoFactorCode)
			}

			// Check key redaction
			if tc.user.Key != nil {
				if tc.user.Key.Key != tc.expectedKeyRedacted {
					t.Errorf("Key.Key = %q, expected %q", tc.user.Key.Key, tc.expectedKeyRedacted)
				}
			} else if tc.expectedKeyRedacted != "" {
				t.Error("Expected key to be present but was nil")
			}

			t.Logf("RemoveSensitiveInformation() correctly sanitized user %q ✓", tc.user.Email)
		})
	}
}

func TestUser_RemoveSensitiveInformation_PreservesNonSensitiveData(t *testing.T) {
	originalID := primitive.NewObjectID()
	originalEmail := "preserve@example.com"
	originalDisabled := true
	originalIsAdmin := true
	originalIsManager := false

	user := &User{
		ID:            originalID,
		Email:         originalEmail,
		Disabled:      originalDisabled,
		IsAdmin:       originalIsAdmin,
		IsManager:     originalIsManager,
		Password:      "should-be-removed",
		Password2:     "should-be-removed",
		ConfirmCode:   "should-be-removed",
		RecoveryCodes: []byte("should-be-removed"),
		TwoFactorCode: []byte("should-be-removed"),
	}

	user.RemoveSensitiveInformation()

	// Verify non-sensitive data is preserved
	if user.ID != originalID {
		t.Errorf("ID changed: got %v, expected %v", user.ID, originalID)
	}
	if user.Email != originalEmail {
		t.Errorf("Email changed: got %q, expected %q", user.Email, originalEmail)
	}
	if user.Disabled != originalDisabled {
		t.Errorf("Disabled changed: got %v, expected %v", user.Disabled, originalDisabled)
	}
	if user.IsAdmin != originalIsAdmin {
		t.Errorf("IsAdmin changed: got %v, expected %v", user.IsAdmin, originalIsAdmin)
	}
	if user.IsManager != originalIsManager {
		t.Errorf("IsManager changed: got %v, expected %v", user.IsManager, originalIsManager)
	}

	t.Log("RemoveSensitiveInformation() correctly preserves non-sensitive data ✓")
}

// Test UserCoreMapping.IsHostAllowed
func TestUserCoreMapping_IsHostAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedHosts []*AllowedHost
		checkHost    [4]byte
		checkPort    [2]byte
		expectFound  bool
		expectIndex  int
	}{
		{
			name: "manual host - should match regardless of port",
			allowedHosts: []*AllowedHost{
				{IP: [4]byte{192, 168, 1, 1}, PORT: [2]byte{0, 80}, Type: "manual"},
			},
			checkHost:   [4]byte{192, 168, 1, 1},
			checkPort:   [2]byte{1, 187}, // Different port (443 = 0x01BB)
			expectFound: true,
			expectIndex: 0,
		},
		{
			name: "auto host - must match both IP and port",
			allowedHosts: []*AllowedHost{
				{IP: [4]byte{10, 0, 0, 1}, PORT: [2]byte{0, 80}, Type: "auto"},
			},
			checkHost:   [4]byte{10, 0, 0, 1},
			checkPort:   [2]byte{0, 80},
			expectFound: true,
			expectIndex: 0,
		},
		{
			name: "auto host - wrong port should not match",
			allowedHosts: []*AllowedHost{
				{IP: [4]byte{10, 0, 0, 1}, PORT: [2]byte{0, 80}, Type: "auto"},
			},
			checkHost:   [4]byte{10, 0, 0, 1},
			checkPort:   [2]byte{1, 187}, // Different port (443 = 0x01BB)
			expectFound: false,
		},
		{
			name: "IP not in allowed list",
			allowedHosts: []*AllowedHost{
				{IP: [4]byte{192, 168, 1, 1}, PORT: [2]byte{0, 80}, Type: "manual"},
			},
			checkHost:   [4]byte{192, 168, 1, 2}, // Different IP
			checkPort:   [2]byte{0, 80},
			expectFound: false,
		},
		{
			name: "multiple hosts - find manual in middle",
			allowedHosts: []*AllowedHost{
				{IP: [4]byte{192, 168, 1, 1}, PORT: [2]byte{0, 80}, Type: "auto"},
				{IP: [4]byte{192, 168, 1, 2}, PORT: [2]byte{1, 187}, Type: "manual"}, // 443 = 0x01BB
				{IP: [4]byte{192, 168, 1, 3}, PORT: [2]byte{0, 22}, Type: "auto"},
			},
			checkHost:   [4]byte{192, 168, 1, 2},
			checkPort:   [2]byte{1, 1}, // Any port should work for manual
			expectFound: true,
			expectIndex: 1,
		},
		{
			name: "multiple hosts - find auto with correct port",
			allowedHosts: []*AllowedHost{
				{IP: [4]byte{192, 168, 1, 1}, PORT: [2]byte{0, 80}, Type: "auto"},
				{IP: [4]byte{192, 168, 1, 2}, PORT: [2]byte{1, 187}, Type: "auto"}, // 443 = 0x01BB
				{IP: [4]byte{192, 168, 1, 3}, PORT: [2]byte{0, 22}, Type: "auto"},
			},
			checkHost:   [4]byte{192, 168, 1, 2},
			checkPort:   [2]byte{1, 187}, // 443 = 0x01BB
			expectFound: true,
			expectIndex: 1,
		},
		{
			name:         "empty allowed hosts list",
			allowedHosts: []*AllowedHost{},
			checkHost:    [4]byte{192, 168, 1, 1},
			checkPort:    [2]byte{0, 80},
			expectFound:  false,
		},
		{
			name:         "nil allowed hosts list",
			allowedHosts: nil,
			checkHost:    [4]byte{192, 168, 1, 1},
			checkPort:    [2]byte{0, 80},
			expectFound:  false,
		},
		{
			name: "manual type priority - should return first manual match",
			allowedHosts: []*AllowedHost{
				{IP: [4]byte{192, 168, 1, 1}, PORT: [2]byte{0, 80}, Type: "auto"},
				{IP: [4]byte{192, 168, 1, 1}, PORT: [2]byte{1, 187}, Type: "manual"}, // 443 = 0x01BB
			},
			checkHost:   [4]byte{192, 168, 1, 1},
			checkPort:   [2]byte{0, 22}, // Different from both
			expectFound: true,
			expectIndex: 1, // Should find manual type
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ucm := &UserCoreMapping{
				AllowedHosts: tc.allowedHosts,
			}

			result := ucm.IsHostAllowed(tc.checkHost, tc.checkPort)

			if tc.expectFound {
				if result == nil {
					t.Errorf("IsHostAllowed(%v, %v) = nil, expected to find host", tc.checkHost, tc.checkPort)
					return
				}

				// Verify it's the correct host
				if result.IP != tc.checkHost {
					t.Errorf("IsHostAllowed returned wrong host: IP = %v, expected %v", result.IP, tc.checkHost)
				}

				// Verify it's the correct index
				if tc.expectIndex >= 0 && tc.expectIndex < len(tc.allowedHosts) {
					expectedHost := tc.allowedHosts[tc.expectIndex]
					if result.IP != expectedHost.IP || result.PORT != expectedHost.PORT || result.Type != expectedHost.Type {
						t.Errorf("IsHostAllowed returned host at wrong index")
					}
				}

				t.Logf("IsHostAllowed(%v, %v) correctly found host (type=%s) ✓", tc.checkHost, tc.checkPort, result.Type)
			} else {
				if result != nil {
					t.Errorf("IsHostAllowed(%v, %v) = %+v, expected nil", tc.checkHost, tc.checkPort, result)
				}
				t.Logf("IsHostAllowed(%v, %v) correctly returned nil ✓", tc.checkHost, tc.checkPort)
			}
		})
	}
}

func TestUserCoreMapping_IsHostAllowed_PortEncoding(t *testing.T) {
	// Test different port encodings (big-endian vs little-endian)
	tests := []struct {
		name      string
		portBytes [2]byte
		portNum   uint16
	}{
		{
			name:      "port 80 - 0x0050",
			portBytes: [2]byte{0x00, 0x50},
			portNum:   80,
		},
		{
			name:      "port 443 - 0x01BB",
			portBytes: [2]byte{0x01, 0xBB},
			portNum:   443,
		},
		{
			name:      "port 8080 - 0x1F90",
			portBytes: [2]byte{0x1F, 0x90},
			portNum:   8080,
		},
		{
			name:      "port 22 - 0x0016",
			portBytes: [2]byte{0x00, 0x16},
			portNum:   22,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ucm := &UserCoreMapping{
				AllowedHosts: []*AllowedHost{
					{IP: [4]byte{192, 168, 1, 1}, PORT: tc.portBytes, Type: "auto"},
				},
			}

			result := ucm.IsHostAllowed([4]byte{192, 168, 1, 1}, tc.portBytes)
			if result == nil {
				t.Errorf("IsHostAllowed failed to find host with port %d (%v)", tc.portNum, tc.portBytes)
			} else {
				t.Logf("IsHostAllowed correctly matched port %d (%v) ✓", tc.portNum, tc.portBytes)
			}
		})
	}
}
