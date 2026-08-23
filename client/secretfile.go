package client

import (
	"fmt"
	"os"
)

// writeSecretFile writes data with owner-only access. On Unix that is 0600;
// on Windows it is a protected DACL for the current user, SYSTEM, and
// Administrators. Existing files are re-restricted because WriteFile does
// not change the mode of a file that already exists.
func writeSecretFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	if err := secureSecretFile(path); err != nil {
		return fmt.Errorf("set owner-only permissions on %s: %w", path, err)
	}
	return nil
}

// warnIfInsecureSecretFile logs a SECURITY warning when a secret file is
// group/other-readable (Unix) or granted to a world-style SID (Windows).
// It tries to tighten permissions first and only warns if the file is
// still insecure. It does not block load.
func warnIfInsecureSecretFile(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	err = validateUserKeyFile(path, info)
	if err == nil {
		return
	}
	if fixErr := secureSecretFile(path); fixErr == nil {
		info, statErr := os.Stat(path)
		if statErr == nil && validateUserKeyFile(path, info) == nil {
			DEBUG("tightened secret file permissions:", path)
			return
		}
	}
	SECURITY("insecure secret file permissions (continuing):", path, "—", err)
}
