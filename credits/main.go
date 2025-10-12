package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	// Read go.mod file
	goModPath := filepath.Join("..", "go.mod")
	file, err := os.Open(goModPath)
	if err != nil {
		fmt.Printf("Error opening go.mod: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Parse dependencies
	var dependencies []string
	scanner := bufio.NewScanner(file)
	inRequire := false
	moduleRegex := regexp.MustCompile(`^\s*([a-zA-Z0-9.\-/]+)`)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "require (") {
			inRequire = true
			continue
		}

		if inRequire {
			if trimmed == ")" {
				inRequire = false
				continue
			}

			// Skip comments and empty lines
			if strings.HasPrefix(trimmed, "//") || trimmed == "" {
				continue
			}

			// Extract module path
			matches := moduleRegex.FindStringSubmatch(trimmed)
			if len(matches) > 1 {
				modulePath := matches[1]
				// Skip standard library and indirect dependencies for now
				if !strings.Contains(modulePath, ".") {
					continue
				}
				dependencies = append(dependencies, modulePath)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading go.mod: %v\n", err)
		os.Exit(1)
	}

	// Create CREDITS.md file
	creditsPath := filepath.Join("..", "CREDITS.md")
	creditsFile, err := os.Create(creditsPath)
	if err != nil {
		fmt.Printf("Error creating CREDITS.md: %v\n", err)
		os.Exit(1)
	}
	defer creditsFile.Close()

	writer := bufio.NewWriter(creditsFile)
	defer writer.Flush()

	fmt.Fprintf(writer, "# Third-Party Software Credits\n\n")
	fmt.Fprintf(writer, "This project uses the following open-source software:\n\n")

	// HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Fetch license for each dependency
	for i, dep := range dependencies {
		fmt.Printf("Processing %d/%d: %s\n", i+1, len(dependencies), dep)

		// Construct GitHub URL (most Go modules are on GitHub)
		repoURL := fmt.Sprintf("https://%s", dep)

		// Try to fetch LICENSE file
		licenseText := fetchLicense(client, dep)

		// Write to CREDITS.md
		fmt.Fprintf(writer, "## %s\n\n", dep)
		fmt.Fprintf(writer, "**Link:** %s\n\n", repoURL)
		fmt.Fprintf(writer, "**License:**\n\n")
		fmt.Fprintf(writer, "```\n%s\n```\n\n", licenseText)

		// Add spacing between entries
		fmt.Fprintf(writer, "---\n\n")
	}

	fmt.Printf("\nSuccessfully generated CREDITS.md with %d dependencies\n", len(dependencies))
}

// fetchLicense attempts to fetch the license from various common locations
func fetchLicense(client *http.Client, modulePath string) string {
	// Try different license file names
	licenseFiles := []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING", "COPYING.md"}

	// Convert module path to GitHub raw URL
	// Example: github.com/user/repo -> https://raw.githubusercontent.com/user/repo/master/LICENSE
	parts := strings.Split(modulePath, "/")

	var rawBaseURL string
	if len(parts) >= 3 && parts[0] == "github.com" {
		// GitHub module
		rawBaseURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s", parts[1], parts[2])
	} else if strings.HasPrefix(modulePath, "go.etcd.io") {
		// etcd modules
		repo := strings.TrimPrefix(modulePath, "go.etcd.io/")
		rawBaseURL = fmt.Sprintf("https://raw.githubusercontent.com/etcd-io/%s", repo)
	} else if strings.HasPrefix(modulePath, "go.mongodb.org") {
		// MongoDB modules
		rawBaseURL = "https://raw.githubusercontent.com/mongodb/mongo-go-driver"
	} else if strings.HasPrefix(modulePath, "golang.org/x/") {
		// Go extended packages
		pkg := strings.TrimPrefix(modulePath, "golang.org/x/")
		rawBaseURL = fmt.Sprintf("https://raw.githubusercontent.com/golang/%s", pkg)
	} else if strings.HasPrefix(modulePath, "kernel.org") {
		// kernel.org modules
		return "License information available at: " + modulePath
	} else {
		// Try generic approach
		rawBaseURL = fmt.Sprintf("https://raw.githubusercontent.com/%s", strings.TrimPrefix(modulePath, strings.Split(modulePath, "/")[0]+"/"))
	}

	// Try common branches
	branches := []string{"main", "master", "HEAD"}

	for _, branch := range branches {
		for _, licenseFile := range licenseFiles {
			url := fmt.Sprintf("%s/%s/%s", rawBaseURL, branch, licenseFile)

			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err == nil && len(body) > 0 {
					return string(body)
				}
			}
		}
	}

	return fmt.Sprintf("License file not found. Please visit https://%s for license information.", modulePath)
}
