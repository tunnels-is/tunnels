package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
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
				ID:       uuid.New(),
				Email:    "test@example.com",
				Disabled: false,
				IsAdmin:  true,
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
				ID:       uuid.New(),
				Email:    "disabled@example.com",
				Disabled: true,
				IsAdmin:  false,
			},
			expected: MinifiedUser{
				Email:     "disabled@example.com",
				Disabled:  true,
				IsAdmin:   false,
				IsManager: false,
			},
		},
		{
			// NOTE: the User struct has no manager field — MinifiedUser.IsManager
			// is always false (vestigial; the admin UI still shows a manager
			// toggle that the backend ignores).
			name: "manager user",
			user: &User{
				ID:       uuid.New(),
				Email:    "manager@example.com",
				Disabled: false,
				IsAdmin:  false,
			},
			expected: MinifiedUser{
				Email:     "manager@example.com",
				Disabled:  false,
				IsAdmin:   false,
				IsManager: false,
			},
		},
		{
			name: "admin and manager",
			user: &User{
				ID:       uuid.New(),
				Email:    "superuser@example.com",
				Disabled: false,
				IsAdmin:  true,
			},
			expected: MinifiedUser{
				Email:     "superuser@example.com",
				Disabled:  false,
				IsAdmin:   true,
				IsManager: false,
			},
		},
		{
			name: "user with empty email",
			user: &User{
				ID:       uuid.New(),
				Email:    "",
				Disabled: false,
				IsAdmin:  false,
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

			if result.ID != tc.user.ID.String() {
				t.Errorf("ToMinifiedUser().ID = %q, expected %q", result.ID, tc.user.ID.String())
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
		ID:            uuid.New(),
		Email:         "test@example.com",
		Password:      "hashed-password-should-not-be-in-minified",
		ConfirmCode:   "secret-confirm-code",
		RecoveryCodes: []byte("recovery-codes"),
		TwoFactorCode: []byte("2fa-code"),
		APIKey:        "api-key-secret",
		Disabled:      false,
		IsAdmin:       true,
	}

	minified := user.ToMinifiedUser()

	if minified.Email != "test@example.com" {
		t.Errorf("Email should be included in minified user")
	}

	if minified.ID != user.ID.String() {
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
				ID:            uuid.New(),
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
				ID:            uuid.New(),
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
				ID:            uuid.New(),
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
				ID:            uuid.New(),
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
				ID:            uuid.New(),
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
				ID:            uuid.New(),
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
	originalID := uuid.New()
	originalEmail := "preserve@example.com"
	originalDisabled := true
	originalIsAdmin := true

	user := &User{
		ID:            originalID,
		Email:         originalEmail,
		Disabled:      originalDisabled,
		IsAdmin:       originalIsAdmin,
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

	t.Log("RemoveSensitiveInformation() correctly preserves non-sensitive data ✓")
}
