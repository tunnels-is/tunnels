package main

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

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

			if result.ID != tc.user.ID.Hex() {
				t.Errorf("ToMinifiedUser().ID = %q, expected %q", result.ID, tc.user.ID.Hex())
			}

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

	if minified.Email != "test@example.com" {
		t.Errorf("Email should be included in minified user")
	}

	if minified.ID != user.ID.Hex() {
		t.Errorf("ID should be included in minified user")
	}

	t.Log("ToMinifiedUser() correctly excludes sensitive data ✓")
}

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
			expectedKeyRedacted: "SINGLEPARTHARDKEY",
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
			expectedKeyRedacted: "",
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
