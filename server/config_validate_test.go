package main

import (
	"strings"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func TestValidateServerConfig(t *testing.T) {
	okKey := strings.Repeat("a", minSecretLen)
	cases := []struct {
		name    string
		cfg     *types.ServerConfig
		wantErr bool
	}{
		{"nil config", nil, true},
		{"empty cookie key", &types.ServerConfig{CookieSigningKey: "", TwoFactorKey: okKey}, true},
		{"too-short cookie key", &types.ServerConfig{CookieSigningKey: strings.Repeat("a", minSecretLen-1), TwoFactorKey: okKey}, true},
		{"empty two-factor key", &types.ServerConfig{CookieSigningKey: okKey, TwoFactorKey: ""}, true},
		{"too-short two-factor key", &types.ServerConfig{CookieSigningKey: okKey, TwoFactorKey: strings.Repeat("a", minSecretLen-1)}, true},
		{"both valid", &types.ServerConfig{CookieSigningKey: okKey, TwoFactorKey: okKey}, false},
		{"short admin API key", &types.ServerConfig{CookieSigningKey: okKey, TwoFactorKey: okKey, AdminAPIKey: "short"}, true},
		{"ok admin API key", &types.ServerConfig{CookieSigningKey: okKey, TwoFactorKey: okKey, AdminAPIKey: strings.Repeat("b", 16)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateServerConfig(tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateServerConfig() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
