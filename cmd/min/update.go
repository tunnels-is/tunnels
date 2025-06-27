package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/client"
)

// VersionResponse represents the response from the update endpoint
type VersionResponse struct {
	Version string `json:"version"`
}

// checkForUpdates calls the auth server to get the current version
func checkForUpdates(authServer string, secure bool) (string, error) {
	url := authServer + "/update"
	if !strings.HasPrefix(url, "http") {
		if secure {
			url = "https://" + url
		} else {
			url = "http://" + url
		}
	}

	responseBytes, code, err := client.SendRequestToURL(
		nil,
		"GET",
		url,
		nil,
		10000,
		secure,
	)

	if err != nil {
		return "", fmt.Errorf("failed to check for updates: %w", err)
	}

	if code != 200 {
		return "", fmt.Errorf("server returned status code %d", code)
	}

	var versionResp VersionResponse
	if err := json.Unmarshal(responseBytes, &versionResp); err != nil {
		return "", fmt.Errorf("failed to parse version response: %w", err)
	}

	return versionResp.Version, nil
}

// getCurrentVersion returns the current version constant
func getCurrentVersion() string {
	return "2.0.0" // This should match the version constant in client/new.go
}

// getArchitecture returns the current OS and architecture for the download URL
func getArchitecture() string {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// Map Go architecture names to GitHub release naming convention
	switch archName {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		// Keep as is
	case "arm":
		archName = "armv7"
	case "386":
		archName = "i386"
	}

	// Capitalize first letter of OS name
	switch osName {
	case "linux":
		osName = "Linux"
	case "darwin":
		osName = "Darwin"
	case "windows":
		osName = "Windows"
	}

	return fmt.Sprintf("%s_%s", osName, archName)
}

// downloadUpdate downloads the latest version from GitHub
func downloadUpdate(version string, targetPath string) error {
	arch := getArchitecture()
	url := fmt.Sprintf("https://github.com/tunnels-is/tunnels/releases/download/v%s/min_%s_%s.tar.gz", version, version, arch)

	responseBytes, code, err := client.SendRequestToURL(
		nil,
		"GET",
		url,
		nil,
		60000, // 60 second timeout for download
		true,  // Use secure connection for GitHub
	)

	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	if code != 200 {
		return fmt.Errorf("download failed with status code %d", code)
	}

	// Create temporary file for the downloaded archive
	tempFile, err := os.CreateTemp("", "tunnels-update-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write downloaded data to temp file
	if _, err := tempFile.Write(responseBytes); err != nil {
		return fmt.Errorf("failed to write download to temp file: %w", err)
	}

	// Reset file pointer for reading
	if _, err := tempFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek temp file: %w", err)
	}

	// Extract the binary from the tar.gz archive
	if err := extractBinary(tempFile, targetPath); err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	return nil
}

// extractBinary extracts the tunnels binary from the tar.gz archive
func extractBinary(archive *os.File, targetPath string) error {
	gzr, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Look for the tunnels binary (might be named differently on Windows)
		binaryName := "tunnels"
		if runtime.GOOS == "windows" {
			binaryName = "tunnels.exe"
		}

		if header.Typeflag == tar.TypeReg && (filepath.Base(header.Name) == binaryName || filepath.Base(header.Name) == "min" || filepath.Base(header.Name) == "min.exe") {
			// Create the target file
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("failed to create target file: %w", err)
			}
			defer outFile.Close()

			// Copy the binary content
			if _, err := io.Copy(outFile, tr); err != nil {
				return fmt.Errorf("failed to copy binary content: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("binary not found in archive")
}

// getCurrentBinaryPath returns the path of the currently running binary
func getCurrentBinaryPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve any symlinks
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return executable, nil // Fall back to original path if symlink resolution fails
	}

	return resolved, nil
}

// restartWithSameArgs restarts the application with the same command line arguments
func restartWithSameArgs(newBinaryPath string) error {
	// Get current command line arguments (excluding the program name)
	args := os.Args[1:]

	// Start the new process
	cmd := exec.Command(newBinaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// On Windows, we need to set the proper attributes
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		}
	} else {
		// On Unix systems, create a new process group
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start new process: %w", err)
	}

	// The new process is now running, we can exit this one
	os.Exit(0)
	return nil // This line will never be reached
}

func restartProcess() error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command(os.Args[0], os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = os.Environ()
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		}
		err := cmd.Run()
		if err == nil {
			os.Exit(0)
		}
		return err
	}

	// Use the original binary location. This works with symlinks such that if
	// the file it points to has been changed we will use the updated symlink.
	argv0, err := exec.LookPath(os.Args[0])
	if err != nil {
		return err
	}

	// Invokes the execve system call.
	// Re-uses the same pid. This preserves the pid over multiple server-respawns.
	return syscall.Exec(argv0, os.Args, os.Environ())
}

// performUpdate handles the complete update process
func performUpdate(cli *client.CLIInfo) error {
	// Set a timeout for the entire update process
	updateTimeout := 5 * time.Minute
	deadline := time.Now().Add(updateTimeout)

	// Step 1: Check for updates
	client.INFO("Checking for updates from:", cli.AuthServer)
	if time.Now().After(deadline) {
		return fmt.Errorf("update process timed out")
	}

	latestVersion, err := checkForUpdates(cli.AuthServer, cli.Secure)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	currentVersion := getCurrentVersion()
	client.INFO("Current version:", currentVersion)
	client.INFO("Latest version:", latestVersion)

	// Step 2: Compare versions
	if latestVersion == currentVersion {
		client.INFO("Already running the latest version")
		return nil
	}

	client.INFO("New version available, starting update process...")

	// Step 3: Get current binary path
	if time.Now().After(deadline) {
		return fmt.Errorf("update process timed out")
	}

	currentBinaryPath, err := getCurrentBinaryPath()
	if err != nil {
		return fmt.Errorf("failed to get current binary path: %w", err)
	}

	// Step 4: Move current binary to backup
	backupPath := currentBinaryPath + ".prev"
	client.INFO("Backing up current binary to:", backupPath)

	// Remove old backup if it exists
	if _, err := os.Stat(backupPath); err == nil {
		if err := os.Remove(backupPath); err != nil {
			client.ERROR("Warning: failed to remove old backup:", err)
		}
	}

	// Move current binary to backup location
	if err := os.Rename(currentBinaryPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Step 5: Download and install new version
	client.INFO("Downloading new version...")
	if time.Now().After(deadline) {
		// Restore backup on timeout
		client.ERROR("Update timed out, restoring backup...")
		if restoreErr := os.Rename(backupPath, currentBinaryPath); restoreErr != nil {
			client.ERROR("Critical: failed to restore backup:", restoreErr)
		}
		return fmt.Errorf("update process timed out during download")
	}

	if err := downloadUpdate(latestVersion, currentBinaryPath); err != nil {
		// Restore backup on failure
		client.ERROR("Update failed, restoring backup...")
		if restoreErr := os.Rename(backupPath, currentBinaryPath); restoreErr != nil {
			client.ERROR("Critical: failed to restore backup:", restoreErr)
		}
		return fmt.Errorf("failed to download update: %w", err)
	}

	client.INFO("Update downloaded successfully")

	// Step 6: Make the new binary executable (Unix systems)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(currentBinaryPath, 0755); err != nil {
			client.ERROR("Warning: failed to set executable permissions:", err)
		}
	}

	// Step 7: Restart with new binary
	client.INFO("Restarting with new version...")

	// Give a moment for logs to flush
	time.Sleep(1 * time.Second)

	return restartWithSameArgs(currentBinaryPath)
}

// AutoUpdate performs the auto-update check and process
func AutoUpdate() {
	// Add defer for error recovery
	defer func() {
		if r := recover(); r != nil {
			client.ERROR("Auto-update panic recovered:", r)
		}
	}()

	cli := client.CLIConfig.Load()
	if !cli.Enabled {
		client.DEBUG("CLI mode not enabled, skipping auto-update")
		return
	}

	if cli.AuthServer == "" {
		client.ERROR("No auth server configured, skipping auto-update")
		return
	}

	client.INFO("Starting auto-update process...")

	if err := performUpdate(cli); err != nil {
		client.ERROR("Auto-update failed:", err)
		// Don't exit the program if update fails, just continue with current version
		client.INFO("Continuing with current version...")
	}
}
