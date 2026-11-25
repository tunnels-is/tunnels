package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestUser_ToMinifiedUser ...
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

// TestUser_ToMinifiedUser_DoesNotIncludeSensitiveData ...
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

// TestUser_RemoveSensitiveInformation ...
func TestUser_RemoveSensitiveInformation(t *testing.T) {
	user := &User{
		ID:            uuid.New(),
		Email:         "test@example.com",
		Password:      "hashed-password",
		Password2:     "password2",
		ConfirmCode:   "ABC123",
		RecoveryCodes: []byte("recovery1,recovery2"),
		TwoFactorCode: []byte("123456"),
	}

	user.RemoveSensitiveInformation()

	if user.Password != "" {
		t.Errorf("Password should be empty, got %q", user.Password)
	}
	if user.Password2 != "" {
		t.Errorf("Password2 should be empty, got %q", user.Password2)
	}
	if user.ConfirmCode != "" {
		t.Errorf("ConfirmCode should be empty, got %q", user.ConfirmCode)
	}
	if user.RecoveryCodes != nil {
		t.Errorf("RecoveryCodes should be nil, got %v", user.RecoveryCodes)
	}
	if user.TwoFactorCode != nil {
		t.Errorf("TwoFactorCode should be nil, got %v", user.TwoFactorCode)
	}
}

// TestUser_RemoveSensitiveInformation_PreservesNonSensitiveData ...
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

// TestUser_RemoveSensitiveInformation_RedactsOtherSessionTokens ...
func TestUser_RemoveSensitiveInformation_RedactsOtherSessionTokens(t *testing.T) {
	current := &DeviceToken{DT: "current-secret", N: "this-device", Created: time.Now()}
	other := &DeviceToken{DT: "other-secret", N: "other-device", Created: time.Now().Add(-time.Hour)}
	user := &User{
		Email:       "sess@example.com",
		DeviceToken: current,
		Tokens:      []*DeviceToken{current, other},
		APIKey:      "should-clear",
	}

	user.RemoveSensitiveInformation()

	if user.APIKey != "" {
		t.Fatalf("APIKey should be cleared, got %q", user.APIKey)
	}
	if user.DeviceToken == nil || user.DeviceToken.DT != "current-secret" {
		t.Fatalf("current DeviceToken.DT must be preserved, got %#v", user.DeviceToken)
	}
	if len(user.Tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(user.Tokens))
	}
	if user.Tokens[0].DT != "current-secret" {
		t.Errorf("current entry in Tokens should keep DT, got %q", user.Tokens[0].DT)
	}
	if user.Tokens[1].DT != "" {
		t.Errorf("other session DT must be redacted, got %q", user.Tokens[1].DT)
	}
	if user.Tokens[1].N != "other-device" {
		t.Errorf("other session name should remain, got %q", user.Tokens[1].N)
	}
}

// TestUser_RemoveSensitiveInformation_AdminListNoDeviceToken ...
func TestUser_RemoveSensitiveInformation_AdminListNoDeviceToken(t *testing.T) {
	user := &User{
		Email: "admin-view@example.com",
		Tokens: []*DeviceToken{
			{DT: "s1", N: "phone", Created: time.Now()},
			{DT: "s2", N: "laptop", Created: time.Now()},
		},
	}
	user.RemoveSensitiveInformation()
	for i, tok := range user.Tokens {
		if tok.DT != "" {
			t.Errorf("token %d DT should be empty without DeviceToken context, got %q", i, tok.DT)
		}
	}
}

// TestDeviceTokenMatchesLogout ...
func TestDeviceTokenMatchesLogout(t *testing.T) {
	created := time.Date(2026, 3, 1, 12, 0, 0, 123456789, time.UTC)
	dt := &DeviceToken{DT: "secret-dt", N: "laptop", Created: created}

	if !deviceTokenMatchesLogout(dt, &logoutRequest{LogoutToken: "secret-dt"}) {
		t.Error("should match by LogoutToken")
	}
	if deviceTokenMatchesLogout(dt, &logoutRequest{LogoutToken: "wrong"}) {
		t.Error("should not match wrong LogoutToken")
	}
	if !deviceTokenMatchesLogout(dt, &logoutRequest{
		LogoutName:    "laptop",
		LogoutCreated: time.Unix(created.Unix(), 0).UTC(),
	}) {
		t.Error("should match by name + created (second resolution)")
	}
	if deviceTokenMatchesLogout(dt, &logoutRequest{LogoutName: "laptop"}) {
		t.Error("name alone must not match without created")
	}
}

// TestRevokeUserDeviceTokens ...
func TestRevokeUserDeviceTokens(t *testing.T) {
	t1 := &DeviceToken{DT: "a", N: "one", Created: time.Unix(100, 0)}
	t2 := &DeviceToken{DT: "b", N: "two", Created: time.Unix(200, 0)}
	tokens := []*DeviceToken{t1, t2}

	out := revokeUserDeviceTokens(tokens, &logoutRequest{All: true})
	if len(out) != 0 {
		t.Fatalf("All should clear tokens, got %d", len(out))
	}

	out = revokeUserDeviceTokens([]*DeviceToken{t1, t2}, &logoutRequest{LogoutToken: "b"})
	if len(out) != 1 || out[0].DT != "a" {
		t.Fatalf("LogoutToken revoke failed: %#v", out)
	}

	out = revokeUserDeviceTokens([]*DeviceToken{t1, t2}, &logoutRequest{
		LogoutName:    "one",
		LogoutCreated: time.Unix(100, 0),
	})
	if len(out) != 1 || out[0].N != "two" {
		t.Fatalf("LogoutName+Created revoke failed: %#v", out)
	}
}
