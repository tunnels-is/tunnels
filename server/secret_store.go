package main

import (
	"os"
	"strings"

	"github.com/tunnels-is/tunnels/types"
)

func loadStringSliceKey(key string) []string {
	config := Config.Load()
	switch config.SecretStore {
	case types.ConfigStore:
		switch key {
		case "CertPems":
			return config.CertPems
		case "KeyPems":
			return config.KeyPems
		}
		return config.KeyPems
	case types.EnvStore:
		return strings.Split(os.Getenv(key), ",")
	}

	return []string{}
}

func loadSecret(key string) (v string) {
	config := Config.Load()
	switch config.SecretStore {
	case types.ConfigStore:
		config := Config.Load()
		switch key {
		case "KeyPem":
			return config.KeyPem
		case "CertPem":
			return config.CertPem
		case "SignPem":
			return config.SignPem
		case "AdminAPIKey":
			return config.AdminAPIKey
		case "TwoFactorKey":
			return config.TwoFactorKey
		case "DBurl":
			return config.DBurl
		default:
			return ""
		}
	case types.EnvStore:
		return os.Getenv(key)
	}

	return ""
}
