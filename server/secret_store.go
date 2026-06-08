package main

func loadStringSliceKey(key string) []string {
	config := Config.Load()
	switch key {
	case "CertPems":
		return config.CertPems
	case "KeyPems":
		return config.KeyPems
	}

	return []string{}
}

func loadSecret(key string) (v string) {
	config := Config.Load()
	switch key {
	case "PayKey":
		return config.PayKey
	case "CertPem":
		return config.CertPem
	case "KeyPem":
		return config.KeyPem
	case "AdminAPIKey":
		return config.AdminAPIKey
	case "TwoFactorKey":
		return config.TwoFactorKey
	case "CookieSigningKey":
		return config.CookieSigningKey
	case "DBurl":
		return config.DBurl
	default:
		WARN("env key not found: ", key)
		return ""
	}
}
