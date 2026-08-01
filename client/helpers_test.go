package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetDomainAndSubDomain(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedDomain    string
		expectedSubdomain string
	}{
		{
			name:              "simple domain",
			input:             "example.com",
			expectedDomain:    "example.com",
			expectedSubdomain: "",
		},
		{
			name:              "domain with subdomain",
			input:             "www.example.com",
			expectedDomain:    "www.example.com",
			expectedSubdomain: "",
		},
		{
			name:              "domain with multiple subdomains",
			input:             "api.v2.example.com",
			expectedDomain:    "v2.example.com",
			expectedSubdomain: "api",
		},
		{
			name:              "domain with many subdomains",
			input:             "a.b.c.d.example.com",
			expectedDomain:    "d.example.com",
			expectedSubdomain: "a.b.c",
		},
		{
			name:              "single word (invalid)",
			input:             "localhost",
			expectedDomain:    "",
			expectedSubdomain: "",
		},
		{
			name:              "empty string",
			input:             "",
			expectedDomain:    "",
			expectedSubdomain: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domain, subdomain := GetDomainAndSubDomain(tc.input)
			if domain != tc.expectedDomain {
				t.Errorf("Domain = %q, expected %q", domain, tc.expectedDomain)
			}
			if subdomain != tc.expectedSubdomain {
				t.Errorf("Subdomain = %q, expected %q", subdomain, tc.expectedSubdomain)
			}
			t.Logf("Input: %q -> Domain: %q, Subdomain: %q", tc.input, domain, subdomain)
		})
	}
}

func TestCheckIfPlainDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid domain",
			input:    "example.com",
			expected: true,
		},
		{
			name:     "domain with subdomain",
			input:    "www.example.com",
			expected: true,
		},
		{
			name:     "single word without dot",
			input:    "localhost",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "domain with multiple dots",
			input:    "api.v2.example.com",
			expected: true,
		},
		{
			name:     "IP address",
			input:    "192.168.1.1",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CheckIfPlainDomain(tc.input)
			if result != tc.expected {
				t.Errorf("CheckIfPlainDomain(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsDefaultConnection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "exact match lowercase",
			input:    DefaultTunnelName,
			expected: true,
		},
		{
			name:     "exact match uppercase",
			input:    strings.ToUpper(DefaultTunnelName),
			expected: true,
		},
		{
			name:     "exact match mixed case",
			input:    "TuNnElS",
			expected: true,
		},
		{
			name:     "different name",
			input:    "custom",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "with spaces",
			input:    " " + DefaultTunnelName + " ",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsDefaultConnection(tc.input)
			if result != tc.expected {
				t.Errorf("IsDefaultConnection(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestCreateFolder_NewDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "newdir")

	err := createFolder(target)
	if err != nil {
		t.Fatalf("createFolder failed: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("path is not a directory")
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("permissions = %o, expected 700", info.Mode().Perm())
	}
}

func TestCreateFolder_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "existing")

	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := createFolder(target)
	if err != nil {
		t.Errorf("createFolder should succeed when directory exists, got: %v", err)
	}
}

func TestCreateFolder_ParentMissing(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "no", "such", "parent")

	err := createFolder(target)
	if err == nil {
		t.Fatal("createFolder should fail when parent directory does not exist")
	}
}

func TestInitBaseFoldersAndPaths_CreatesAllSubdirs(t *testing.T) {
	dir := t.TempDir()

	s := &stateV2{BasePath: dir}
	STATE.Store(s)

	InitBaseFoldersAndPaths()

	s = STATE.Load()

	expected := []struct {
		name string
		path string
	}{
		{"BasePath", s.BasePath},
		{"AccountsPath", s.AccountsPath},
		{"UserPath", s.UserPath},
		{"LogPath", s.LogPath},
		{"BlockListPath", s.BlockListPath},
		{"WhiteListPath", s.WhiteListPath},
	}

	for _, e := range expected {
		info, err := os.Stat(e.path)
		if err != nil {
			t.Errorf("%s (%s) does not exist: %v", e.name, e.path, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s (%s) is not a directory", e.name, e.path)
		}
	}
	// Tunnels/devices paths are empty until an account is activated.
	if s.TunnelsPath != "" || s.DevicesPath != "" || s.ActiveAccountHash != "" {
		t.Errorf("expected empty tunnel/device workspace before account activate, got tunnels=%q devices=%q active=%q",
			s.TunnelsPath, s.DevicesPath, s.ActiveAccountHash)
	}
}

func TestInitBaseFoldersAndPaths_TrailingSeparator(t *testing.T) {
	dir := t.TempDir()
	withTrailing := dir + string(os.PathSeparator)

	s := &stateV2{BasePath: withTrailing}
	STATE.Store(s)

	InitBaseFoldersAndPaths()

	s = STATE.Load()

	if strings.HasSuffix(s.BasePath, string(os.PathSeparator)+string(os.PathSeparator)) {
		t.Errorf("BasePath has double separator: %q", s.BasePath)
	}
}

func TestInitBaseFoldersAndPaths_SetsConfigFileName(t *testing.T) {
	dir := t.TempDir()

	s := &stateV2{BasePath: dir}
	STATE.Store(s)

	InitBaseFoldersAndPaths()

	s = STATE.Load()

	if s.ConfigFileName == "" {
		t.Fatal("ConfigFileName should be set")
	}
	if !strings.HasSuffix(s.ConfigFileName, ".conf") {
		t.Errorf("ConfigFileName should end with .conf, got: %q", s.ConfigFileName)
	}
}

func TestVerifyAndWriteFile_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("hello world")

	written, err := verifyAndWriteFile(path, content)
	if err != nil {
		t.Fatalf("verifyAndWriteFile failed: %v", err)
	}
	if !written {
		t.Error("expected file to be written")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("file content = %q, expected %q", got, content)
	}
}

func TestVerifyAndWriteFile_SkipsMatchingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("hello world")

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	written, err := verifyAndWriteFile(path, content)
	if err != nil {
		t.Fatalf("verifyAndWriteFile failed: %v", err)
	}
	if written {
		t.Error("file should not be rewritten when hash matches")
	}
}

func TestVerifyAndWriteFile_ReplaceTamperedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	expected := []byte("correct content")
	tampered := []byte("tampered content")

	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	written, err := verifyAndWriteFile(path, expected)
	if err != nil {
		t.Fatalf("verifyAndWriteFile failed: %v", err)
	}
	if !written {
		t.Error("expected file to be rewritten when hash differs")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(got) != string(expected) {
		t.Errorf("file content = %q, expected %q", got, expected)
	}
}

func TestVerifyAndWriteFile_ReplaceTruncatedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	expected := []byte("full content here")

	if err := os.WriteFile(path, expected[:5], 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	written, err := verifyAndWriteFile(path, expected)
	if err != nil {
		t.Fatalf("verifyAndWriteFile failed: %v", err)
	}
	if !written {
		t.Error("expected file to be rewritten for truncated content")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(got) != string(expected) {
		t.Errorf("file content = %q, expected %q", got, expected)
	}
}
