package main

import (
	"os"

	"github.com/tunnels-is/tunnels/types"
)

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
		case "AdminApiKey":
			return config.AdminApiKey
		case "TwoFactorKey":
			return config.TwoFactorEncKey
		case "EmailKey":
			return config.EmailKey
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
