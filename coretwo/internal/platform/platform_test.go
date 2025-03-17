package platform

import (
	"context"
	"os"
	"runtime"
	"testing"
)

func TestPlatformInitialization(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Skipping test: requires root privileges")
	}

	ctx := context.Background()
	err := Initialize(ctx)
	if err != nil {
		t.Errorf("Platform initialization failed: %v", err)
	}
}

func TestAdminCheck(t *testing.T) {
	err := CheckAdmin()
	if err != nil {
		// On non-root/admin systems, we expect an error
		if runtime.GOOS == "windows" {
			if os.Geteuid() != 0 {
				t.Logf("Admin check failed as expected (non-admin user): %v", err)
				return
			}
		} else {
			if os.Geteuid() != 0 {
				t.Logf("Admin check failed as expected (non-root user): %v", err)
				return
			}
		}
		t.Errorf("Admin check failed unexpectedly: %v", err)
	}
}

func TestNetworkInitialization(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Skipping test: requires root privileges")
	}

	err := InitializeNetwork(nil)
	if err != nil {
		t.Errorf("Network initialization failed: %v", err)
	}
}

func TestPlatformSpecificFunctions(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Skipping test: requires root privileges")
	}

	ctx := context.Background()

	// Test Windows functions
	if runtime.GOOS == "windows" {
		err := initializeWindows(ctx)
		if err != nil {
			t.Errorf("Windows initialization failed: %v", err)
		}

		err = initializeNetworkWindows(nil)
		if err != nil {
			t.Errorf("Windows network initialization failed: %v", err)
		}

		err = checkAdminWindows()
		if err != nil {
			t.Errorf("Windows admin check failed: %v", err)
		}
	}

	// Test Darwin functions
	if runtime.GOOS == "darwin" {
		err := initializeDarwin(ctx)
		if err != nil {
			t.Errorf("Darwin initialization failed: %v", err)
		}

		err = initializeNetworkDarwin(nil)
		if err != nil {
			t.Errorf("Darwin network initialization failed: %v", err)
		}

		err = checkAdminDarwin()
		if err != nil {
			t.Errorf("Darwin admin check failed: %v", err)
		}
	}

	// Test Linux functions
	if runtime.GOOS == "linux" {
		err := initializeLinux(ctx)
		if err != nil {
			t.Errorf("Linux initialization failed: %v", err)
		}

		err = initializeNetworkLinux(nil)
		if err != nil {
			t.Errorf("Linux network initialization failed: %v", err)
		}

		err = checkAdminLinux()
		if err != nil {
			t.Errorf("Linux admin check failed: %v", err)
		}
	}
}

func TestUnsupportedPlatform(t *testing.T) {
	// Test that we get appropriate errors on unsupported platforms
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		ctx := context.Background()
		err := Initialize(ctx)
		if err == nil {
			t.Error("Expected error for unsupported platform, got nil")
		}

		err = InitializeNetwork(nil)
		if err == nil {
			t.Error("Expected error for unsupported platform network initialization, got nil")
		}

		err = CheckAdmin()
		if err == nil {
			t.Error("Expected error for unsupported platform admin check, got nil")
		}
	}
}
