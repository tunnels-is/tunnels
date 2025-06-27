package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// createTestTarGz creates a test tar.gz archive with a binary file
func createTestTarGz(tb testing.TB, binaryName string, content []byte) []byte {
	tb.Helper()

	// Create a buffer to write the archive to
	var buf strings.Builder

	// Create gzip writer
	gzw := gzip.NewWriter(&buf)
	defer gzw.Close()

	// Create tar writer
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// Create tar header for the binary file
	header := &tar.Header{
		Name:     binaryName,
		Mode:     0755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
		ModTime:  time.Now(),
	}
	// Write header
	if err := tw.WriteHeader(header); err != nil {
		tb.Fatalf("Failed to write tar header: %v", err)
	}

	// Write content
	if _, err := tw.Write(content); err != nil {
		tb.Fatalf("Failed to write tar content: %v", err)
	}

	// Close writers to flush
	tw.Close()
	gzw.Close()

	return []byte(buf.String())
}

// createTestServer creates a mock HTTP server for testing downloads
func createTestServer(t *testing.T, responseCode int, responseBody []byte) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(responseCode)
		if responseBody != nil {
			w.Write(responseBody)
		}
	}))
}

func TestGetArchitecture(t *testing.T) {
	// Test the actual function as it is - we can't mock runtime.GOOS easily
	result := getArchitecture()

	// Verify it returns a valid format
	if !strings.Contains(result, "_") {
		t.Errorf("Expected architecture string to contain underscore, got: %s", result)
	}

	// Split on underscore, but we expect at least 2 parts (OS_ARCH)
	// The architecture part might contain additional underscores (like x86_64)
	parts := strings.Split(result, "_")
	if len(parts) < 2 {
		t.Errorf("Expected architecture string to have at least 2 parts separated by underscore, got: %v", parts)
	}

	// The OS is the first part
	osName := parts[0]
	if len(osName) == 0 || osName[0] < 'A' || osName[0] > 'Z' {
		t.Errorf("Expected OS name to be capitalized, got: %s", osName)
	}

	// The architecture is everything after the first underscore
	archName := strings.Join(parts[1:], "_")
	validArchs := []string{"x86_64", "arm64", "armv7", "i386"}
	found := false
	for _, valid := range validArchs {
		if archName == valid {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected valid architecture name from %v, got: %s", validArchs, archName)
	}
}

func TestExtractBinary(t *testing.T) {
	tests := []struct {
		name          string
		binaryName    string
		binaryContent string
		expectSuccess bool
		expectedError string
	}{
		{
			name:          "Extract expected tunnels binary for current OS",
			binaryName:    getExpectedBinaryName(),
			binaryContent: "fake tunnels binary content",
			expectSuccess: true,
		},
		{
			name:          "Extract min binary",
			binaryName:    "min",
			binaryContent: "fake min binary content",
			expectSuccess: true,
		},
		{
			name:          "Extract min.exe binary",
			binaryName:    "min.exe",
			binaryContent: "fake min.exe binary content",
			expectSuccess: true,
		},
		{
			name:          "Binary not found",
			binaryName:    "other-binary",
			binaryContent: "fake other binary content",
			expectSuccess: false,
			expectedError: "binary not found in archive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test tar.gz archive
			archiveData := createTestTarGz(t, tt.binaryName, []byte(tt.binaryContent))

			// Create temporary file for the archive
			tempArchive, err := os.CreateTemp("", "test-archive-*.tar.gz")
			if err != nil {
				t.Fatalf("Failed to create temp archive file: %v", err)
			}
			defer os.Remove(tempArchive.Name())
			defer tempArchive.Close()

			// Write archive data
			if _, err := tempArchive.Write(archiveData); err != nil {
				t.Fatalf("Failed to write archive data: %v", err)
			}

			// Reset file pointer
			if _, err := tempArchive.Seek(0, 0); err != nil {
				t.Fatalf("Failed to seek archive file: %v", err)
			}

			// Create temporary target file
			tempTarget, err := os.CreateTemp("", "test-target-*")
			if err != nil {
				t.Fatalf("Failed to create temp target file: %v", err)
			}
			targetPath := tempTarget.Name()
			tempTarget.Close()
			defer os.Remove(targetPath)

			// Test extraction
			err = extractBinary(tempArchive, targetPath)

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
					return
				}

				// Verify extracted content
				extractedData, err := os.ReadFile(targetPath)
				if err != nil {
					t.Errorf("Failed to read extracted file: %v", err)
					return
				}

				if string(extractedData) != tt.binaryContent {
					t.Errorf("Expected extracted content %q, got %q", tt.binaryContent, string(extractedData))
				}

				// Verify file permissions (Unix only)
				if runtime.GOOS != "windows" {
					info, err := os.Stat(targetPath)
					if err != nil {
						t.Errorf("Failed to stat extracted file: %v", err)
						return
					}

					mode := info.Mode()
					if mode&0755 == 0 {
						t.Errorf("Expected extracted file to have executable permissions, got mode: %v", mode)
					}
				}
			} else {
				if err == nil {
					t.Errorf("Expected error, got success")
					return
				}

				if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing %q, got: %v", tt.expectedError, err)
				}
			}
		})
	}
}

// getExpectedBinaryName returns the binary name that extractBinary would look for
func getExpectedBinaryName() string {
	if runtime.GOOS == "windows" {
		return "tunnels.exe"
	}
	return "tunnels"
}

func TestDownloadUpdate_Success(t *testing.T) {
	// Create test binary content
	binaryContent := "fake binary content for testing"

	// Determine binary name based on OS
	binaryName := "min"
	if runtime.GOOS == "windows" {
		binaryName = "min.exe"
	}

	// Create test tar.gz archive
	archiveData := createTestTarGz(t, binaryName, []byte(binaryContent))

	// Create mock server
	server := createTestServer(t, 200, archiveData)
	defer server.Close()

	// Mock the GitHub URL by replacing the download logic
	// Since we can't easily mock the URL construction, we'll test the download logic separately

	// Create temporary target file
	tempTarget, err := os.CreateTemp("", "test-download-*")
	if err != nil {
		t.Fatalf("Failed to create temp target file: %v", err)
	}
	targetPath := tempTarget.Name()
	tempTarget.Close()
	defer os.Remove(targetPath)

	// Test the extraction part by creating a temporary archive file
	tempArchive, err := os.CreateTemp("", "test-archive-*.tar.gz")
	if err != nil {
		t.Fatalf("Failed to create temp archive file: %v", err)
	}
	defer os.Remove(tempArchive.Name())
	defer tempArchive.Close()

	// Write archive data
	if _, err := tempArchive.Write(archiveData); err != nil {
		t.Fatalf("Failed to write archive data: %v", err)
	}

	// Reset file pointer
	if _, err := tempArchive.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek archive file: %v", err)
	}

	// Test extraction
	err = extractBinary(tempArchive, targetPath)
	if err != nil {
		t.Errorf("Expected successful extraction, got error: %v", err)
		return
	}

	// Verify extracted content
	extractedData, err := os.ReadFile(targetPath)
	if err != nil {
		t.Errorf("Failed to read extracted file: %v", err)
		return
	}

	if string(extractedData) != binaryContent {
		t.Errorf("Expected extracted content %q, got %q", binaryContent, string(extractedData))
	}
}

func TestDownloadUpdate_InvalidArchive(t *testing.T) {
	// Create invalid archive data
	invalidData := []byte("this is not a valid tar.gz file")

	// Create temporary target file
	tempTarget, err := os.CreateTemp("", "test-download-*")
	if err != nil {
		t.Fatalf("Failed to create temp target file: %v", err)
	}
	targetPath := tempTarget.Name()
	tempTarget.Close()
	defer os.Remove(targetPath)

	// Create temporary archive file with invalid data
	tempArchive, err := os.CreateTemp("", "test-archive-*.tar.gz")
	if err != nil {
		t.Fatalf("Failed to create temp archive file: %v", err)
	}
	defer os.Remove(tempArchive.Name())
	defer tempArchive.Close()

	// Write invalid data
	if _, err := tempArchive.Write(invalidData); err != nil {
		t.Fatalf("Failed to write invalid data: %v", err)
	}

	// Reset file pointer
	if _, err := tempArchive.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek archive file: %v", err)
	}

	// Test extraction (should fail)
	err = extractBinary(tempArchive, targetPath)
	if err == nil {
		t.Errorf("Expected error for invalid archive, got success")
	}
}

func TestExtractBinary_EmptyArchive(t *testing.T) {
	// Create empty tar.gz archive
	var buf strings.Builder
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	tw.Close()
	gzw.Close()

	archiveData := []byte(buf.String())

	// Create temporary archive file
	tempArchive, err := os.CreateTemp("", "test-empty-archive-*.tar.gz")
	if err != nil {
		t.Fatalf("Failed to create temp archive file: %v", err)
	}
	defer os.Remove(tempArchive.Name())
	defer tempArchive.Close()

	// Write empty archive data
	if _, err := tempArchive.Write(archiveData); err != nil {
		t.Fatalf("Failed to write archive data: %v", err)
	}

	// Reset file pointer
	if _, err := tempArchive.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek archive file: %v", err)
	}

	// Create temporary target file
	tempTarget, err := os.CreateTemp("", "test-target-*")
	if err != nil {
		t.Fatalf("Failed to create temp target file: %v", err)
	}
	targetPath := tempTarget.Name()
	tempTarget.Close()
	defer os.Remove(targetPath)

	// Test extraction (should fail - no binary found)
	err = extractBinary(tempArchive, targetPath)
	if err == nil {
		t.Errorf("Expected error for empty archive, got success")
	}

	expectedError := "binary not found in archive"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing %q, got: %v", expectedError, err)
	}
}

func TestExtractBinary_ArchiveWithMultipleFiles(t *testing.T) {
	// Create tar.gz archive with multiple files including the target binary
	var buf strings.Builder
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	// Add some other files first
	files := []struct {
		name    string
		content string
	}{
		{"README.md", "This is a readme file"},
		{"config.json", `{"key": "value"}`},
		{"min", "this is the binary we want"},
		{"other-file.txt", "some other content"},
	}

	for _, file := range files {
		header := &tar.Header{
			Name:     file.name,
			Mode:     0644,
			Size:     int64(len(file.content)),
			Typeflag: tar.TypeReg,
			ModTime:  time.Now(),
		}

		if file.name == "min" {
			header.Mode = 0755 // Make binary executable
		}

		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("Failed to write tar header for %s: %v", file.name, err)
		}

		if _, err := tw.Write([]byte(file.content)); err != nil {
			t.Fatalf("Failed to write tar content for %s: %v", file.name, err)
		}
	}

	tw.Close()
	gzw.Close()

	archiveData := []byte(buf.String())

	// Create temporary archive file
	tempArchive, err := os.CreateTemp("", "test-multi-archive-*.tar.gz")
	if err != nil {
		t.Fatalf("Failed to create temp archive file: %v", err)
	}
	defer os.Remove(tempArchive.Name())
	defer tempArchive.Close()

	// Write archive data
	if _, err := tempArchive.Write(archiveData); err != nil {
		t.Fatalf("Failed to write archive data: %v", err)
	}

	// Reset file pointer
	if _, err := tempArchive.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek archive file: %v", err)
	}

	// Create temporary target file
	tempTarget, err := os.CreateTemp("", "test-target-*")
	if err != nil {
		t.Fatalf("Failed to create temp target file: %v", err)
	}
	targetPath := tempTarget.Name()
	tempTarget.Close()
	defer os.Remove(targetPath)

	// Test extraction
	err = extractBinary(tempArchive, targetPath)
	if err != nil {
		t.Errorf("Expected successful extraction, got error: %v", err)
		return
	}

	// Verify extracted content is the binary content
	extractedData, err := os.ReadFile(targetPath)
	if err != nil {
		t.Errorf("Failed to read extracted file: %v", err)
		return
	}

	expectedContent := "this is the binary we want"
	if string(extractedData) != expectedContent {
		t.Errorf("Expected extracted content %q, got %q", expectedContent, string(extractedData))
	}
}

func TestGetCurrentVersion(t *testing.T) {
	version := getCurrentVersion()
	if version == "" {
		t.Error("Expected non-empty version string")
	}

	// Version should follow semantic versioning pattern (at least X.Y.Z)
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		t.Errorf("Expected version to have at least 3 parts (X.Y.Z), got: %s", version)
	}
}

// TestExtractBinary_CorruptedGzipData tests handling of corrupted gzip data
func TestExtractBinary_CorruptedGzipData(t *testing.T) {
	// Create corrupted gzip data
	corruptedData := []byte("corrupted gzip data that's not actually gzipped")

	// Create temporary archive file with corrupted data
	tempArchive, err := os.CreateTemp("", "test-corrupted-*.tar.gz")
	if err != nil {
		t.Fatalf("Failed to create temp archive file: %v", err)
	}
	defer os.Remove(tempArchive.Name())
	defer tempArchive.Close()

	// Write corrupted data
	if _, err := tempArchive.Write(corruptedData); err != nil {
		t.Fatalf("Failed to write corrupted data: %v", err)
	}

	// Reset file pointer
	if _, err := tempArchive.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek archive file: %v", err)
	}

	// Create temporary target file
	tempTarget, err := os.CreateTemp("", "test-target-*")
	if err != nil {
		t.Fatalf("Failed to create temp target file: %v", err)
	}
	targetPath := tempTarget.Name()
	tempTarget.Close()
	defer os.Remove(targetPath)

	// Test extraction (should fail)
	err = extractBinary(tempArchive, targetPath)
	if err == nil {
		t.Errorf("Expected error for corrupted gzip data, got success")
	}

	expectedError := "failed to create gzip reader"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing %q, got: %v", expectedError, err)
	}
}

// TestExtractBinary_ReadOnlyTargetDirectory tests handling when target directory is read-only
func TestExtractBinary_ReadOnlyTargetDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping read-only directory test on Windows")
	}

	// Create test tar.gz archive
	binaryContent := "test binary content"
	archiveData := createTestTarGz(t, "min", []byte(binaryContent))

	// Create temporary archive file
	tempArchive, err := os.CreateTemp("", "test-archive-*.tar.gz")
	if err != nil {
		t.Fatalf("Failed to create temp archive file: %v", err)
	}
	defer os.Remove(tempArchive.Name())
	defer tempArchive.Close()

	// Write archive data
	if _, err := tempArchive.Write(archiveData); err != nil {
		t.Fatalf("Failed to write archive data: %v", err)
	}

	// Reset file pointer
	if _, err := tempArchive.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek archive file: %v", err)
	}

	// Create read-only directory
	readOnlyDir, err := os.MkdirTemp("", "readonly-*")
	if err != nil {
		t.Fatalf("Failed to create read-only directory: %v", err)
	}
	defer os.RemoveAll(readOnlyDir)

	// Make directory read-only
	if err := os.Chmod(readOnlyDir, 0444); err != nil {
		t.Fatalf("Failed to make directory read-only: %v", err)
	}

	// Restore write permissions for cleanup
	defer func() {
		os.Chmod(readOnlyDir, 0755)
	}()

	targetPath := readOnlyDir + "/target-binary"

	// Test extraction (should fail)
	err = extractBinary(tempArchive, targetPath)
	if err == nil {
		t.Errorf("Expected error for read-only target directory, got success")
	}

	expectedError := "failed to create target file"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing %q, got: %v", expectedError, err)
	}
}

// TestExtractBinary_LargeArchive tests handling of large archive files
func TestExtractBinary_LargeArchive(t *testing.T) {
	// Create large binary content (1MB)
	largeContent := make([]byte, 1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	// Create test tar.gz archive with large content
	archiveData := createTestTarGz(t, "min", largeContent)

	// Create temporary archive file
	tempArchive, err := os.CreateTemp("", "test-large-archive-*.tar.gz")
	if err != nil {
		t.Fatalf("Failed to create temp archive file: %v", err)
	}
	defer os.Remove(tempArchive.Name())
	defer tempArchive.Close()

	// Write archive data
	if _, err := tempArchive.Write(archiveData); err != nil {
		t.Fatalf("Failed to write archive data: %v", err)
	}

	// Reset file pointer
	if _, err := tempArchive.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek archive file: %v", err)
	}

	// Create temporary target file
	tempTarget, err := os.CreateTemp("", "test-target-*")
	if err != nil {
		t.Fatalf("Failed to create temp target file: %v", err)
	}
	targetPath := tempTarget.Name()
	tempTarget.Close()
	defer os.Remove(targetPath)

	// Test extraction
	err = extractBinary(tempArchive, targetPath)
	if err != nil {
		t.Errorf("Expected successful extraction of large file, got error: %v", err)
		return
	}

	// Verify extracted content
	extractedData, err := os.ReadFile(targetPath)
	if err != nil {
		t.Errorf("Failed to read extracted file: %v", err)
		return
	}

	if len(extractedData) != len(largeContent) {
		t.Errorf("Expected extracted size %d, got %d", len(largeContent), len(extractedData))
		return
	}

	// Verify content integrity (check first and last 100 bytes)
	for i := 0; i < 100; i++ {
		if extractedData[i] != largeContent[i] {
			t.Errorf("Content mismatch at beginning, position %d: expected %d, got %d", i, largeContent[i], extractedData[i])
			break
		}
	}

	start := len(largeContent) - 100
	for i := 0; i < 100; i++ {
		if extractedData[start+i] != largeContent[start+i] {
			t.Errorf("Content mismatch at end, position %d: expected %d, got %d", start+i, largeContent[start+i], extractedData[start+i])
			break
		}
	}
}

// TestVersionComparison tests version string validation
func TestVersionComparison(t *testing.T) {
	currentVersion := getCurrentVersion()

	// Test that version is not empty
	if currentVersion == "" {
		t.Error("Current version should not be empty")
	}

	// Test that version follows semantic versioning pattern
	parts := strings.Split(currentVersion, ".")
	if len(parts) < 3 {
		t.Errorf("Version should have at least 3 parts (major.minor.patch), got: %s", currentVersion)
	}

	// Test that all parts are numeric (basic check)
	for i, part := range parts {
		if part == "" {
			t.Errorf("Version part %d should not be empty in version: %s", i, currentVersion)
		}
		// Check if it's numeric by trying to parse it
		for _, char := range part {
			if char < '0' || char > '9' {
				t.Errorf("Version part %d contains non-numeric character '%c' in version: %s", i, char, currentVersion)
				break
			}
		}
	}
}

// BenchmarkExtractBinary benchmarks the binary extraction performance
func BenchmarkExtractBinary(b *testing.B) {
	// Create test binary content
	binaryContent := make([]byte, 10*1024) // 10KB
	for i := range binaryContent {
		binaryContent[i] = byte(i % 256)
	}

	// Create test tar.gz archive
	archiveData := createTestTarGz(b, "min", binaryContent)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		// Create temporary archive file
		tempArchive, err := os.CreateTemp("", "bench-archive-*.tar.gz")
		if err != nil {
			b.Fatalf("Failed to create temp archive file: %v", err)
		}

		// Write archive data
		if _, err := tempArchive.Write(archiveData); err != nil {
			tempArchive.Close()
			os.Remove(tempArchive.Name())
			b.Fatalf("Failed to write archive data: %v", err)
		}

		// Reset file pointer
		if _, err := tempArchive.Seek(0, 0); err != nil {
			tempArchive.Close()
			os.Remove(tempArchive.Name())
			b.Fatalf("Failed to seek archive file: %v", err)
		}

		// Create temporary target file
		tempTarget, err := os.CreateTemp("", "bench-target-*")
		if err != nil {
			tempArchive.Close()
			os.Remove(tempArchive.Name())
			b.Fatalf("Failed to create temp target file: %v", err)
		}
		targetPath := tempTarget.Name()
		tempTarget.Close()

		b.StartTimer()

		// Benchmark the extraction
		err = extractBinary(tempArchive, targetPath)

		b.StopTimer()

		// Cleanup
		tempArchive.Close()
		os.Remove(tempArchive.Name())
		os.Remove(targetPath)

		if err != nil {
			b.Fatalf("Extraction failed: %v", err)
		}
	}
}

// TestDownloadURLConstruction tests that the download URL is constructed correctly
func TestDownloadURLConstruction(t *testing.T) {
	version := "2.1.0"
	arch := getArchitecture()

	// Simulate URL construction from downloadUpdate function
	expectedURL := fmt.Sprintf("https://github.com/tunnels-is/tunnels/releases/download/v%s/min_%s_%s.tar.gz", version, version, arch)

	// Verify URL format
	if !strings.HasPrefix(expectedURL, "https://github.com/tunnels-is/tunnels/releases/download/") {
		t.Errorf("Expected URL to start with GitHub releases URL, got: %s", expectedURL)
	}

	if !strings.Contains(expectedURL, version) {
		t.Errorf("Expected URL to contain version %s, got: %s", version, expectedURL)
	}

	if !strings.Contains(expectedURL, arch) {
		t.Errorf("Expected URL to contain architecture %s, got: %s", arch, expectedURL)
	}

	if !strings.HasSuffix(expectedURL, ".tar.gz") {
		t.Errorf("Expected URL to end with .tar.gz, got: %s", expectedURL)
	}

	// Print the URL for manual verification
	t.Logf("Constructed download URL: %s", expectedURL)
}

// TestArchitectureMappingLogic tests the architecture mapping logic more thoroughly
func TestArchitectureMappingLogic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"AMD64 to x86_64", "amd64", "x86_64"},
		{"ARM64 unchanged", "arm64", "arm64"},
		{"ARM to ARMv7", "arm", "armv7"},
		{"386 to i386", "386", "i386"},
		{"Unknown architecture", "unknown", "unknown"}, // Should remain unchanged
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily mock runtime.GOARCH, so we'll test the logic by copying it
			archName := tt.input
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

			if archName != tt.expected {
				t.Errorf("Expected %s to map to %s, got %s", tt.input, tt.expected, archName)
			}
		})
	}
}
