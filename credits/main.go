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

	goModPath := filepath.Join("..", "go.mod")
	file, err := os.Open(goModPath)
	if err != nil {
		fmt.Printf("Error opening go.mod: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

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

			if strings.HasPrefix(trimmed, "//") || trimmed == "" {
				continue
			}

			matches := moduleRegex.FindStringSubmatch(trimmed)
			if len(matches) > 1 {
				modulePath := matches[1]

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

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for i, dep := range dependencies {
		fmt.Printf("Processing %d/%d: %s\n", i+1, len(dependencies), dep)

		repoURL := fmt.Sprintf("https://%s", dep)

		licenseText := fetchLicense(client, dep)

		fmt.Fprintf(writer, "## %s\n\n", dep)
		fmt.Fprintf(writer, "**Link:** %s\n\n", repoURL)
		fmt.Fprintf(writer, "**License:**\n\n")
		fmt.Fprintf(writer, "```\n%s\n```\n\n", licenseText)

		fmt.Fprintf(writer, "---\n\n")
	}

	fmt.Printf("\nSuccessfully generated CREDITS.md with %d dependencies\n", len(dependencies))
}

func fetchLicense(client *http.Client, modulePath string) string {

	licenseFiles := []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING", "COPYING.md"}

	parts := strings.Split(modulePath, "/")

	var rawBaseURL string
	if len(parts) >= 3 && parts[0] == "github.com" {

		rawBaseURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s", parts[1], parts[2])
	} else if strings.HasPrefix(modulePath, "go.etcd.io") {

		repo := strings.TrimPrefix(modulePath, "go.etcd.io/")
		rawBaseURL = fmt.Sprintf("https://raw.githubusercontent.com/etcd-io/%s", repo)
	} else if strings.HasPrefix(modulePath, "go.mongodb.org") {

		rawBaseURL = "https://raw.githubusercontent.com/mongodb/mongo-go-driver"
	} else if strings.HasPrefix(modulePath, "golang.org/x/") {

		pkg := strings.TrimPrefix(modulePath, "golang.org/x/")
		rawBaseURL = fmt.Sprintf("https://raw.githubusercontent.com/golang/%s", pkg)
	} else if strings.HasPrefix(modulePath, "kernel.org") {

		return "License information available at: " + modulePath
	} else {

		rawBaseURL = fmt.Sprintf("https://raw.githubusercontent.com/%s", strings.TrimPrefix(modulePath, strings.Split(modulePath, "/")[0]+"/"))
	}

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
