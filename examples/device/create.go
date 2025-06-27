package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tunnels-is/tunnels/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CreateDevice creates a new device using the tunnels API with X-API-KEY authentication
// Parameters:
//   - serverURL: The base URL of the tunnels server (e.g., "https://localhost:8443")
//   - apiKey: The admin API key for authentication
//   - deviceTag: The tag/name for the new device
//   - groups: Array of group IDs to assign to the device (can be empty)
//
// Returns:
//   - *types.Device: The created device with populated ID and CreatedAt fields
//   - error: Any error that occurred during the creation process
func CreateDevice(serverURL, apiKey, deviceTag string, groups []primitive.ObjectID) (*types.Device, error) {
	// Create a new device
	newDevice := &types.Device{
		Tag:    deviceTag,
		Groups: groups,
	}

	// Create the request body
	// Note: When using X-API-KEY, DeviceToken and UID can be empty/nil
	// The API handler will skip user authentication if a valid API key is provided
	requestBody := &types.FORM_CREATE_DEVICE{
		DeviceToken: "",                    // Not needed when using API key
		UID:         primitive.NilObjectID, // Not needed when using API key
		Device:      newDevice,
	}

	// Marshal the request body to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling JSON: %w", err)
	}

	// Create HTTP client with disabled certificate validation
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Disable certificate validation
			},
		},
		Timeout: 30 * time.Second,
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", serverURL+"/v3/device/create", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey) // Use API key for authentication

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Check if the request was successful
	if resp.StatusCode != 200 {
		// Try to parse error response
		var errorResp map[string]string
		if err := json.Unmarshal(responseBody, &errorResp); err == nil {
			if errorMsg, exists := errorResp["Error"]; exists {
				return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, errorMsg)
			}
		}
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Parse the successful response
	var createdDevice types.Device
	err = json.Unmarshal(responseBody, &createdDevice)
	if err != nil {
		return nil, fmt.Errorf("error parsing response JSON: %w", err)
	}

	return &createdDevice, nil
}

func main() {
	// Example usage of the CreateDevice function

	// Configuration
	serverURL := "https://localhost:8443" // Replace with your server URL
	apiKey := "your-admin-api-key-here"   // Replace with your actual API key
	deviceTag := "Example Production Server"
	groups := make([]primitive.ObjectID, 0) // Empty groups array

	// Create the device
	fmt.Printf("Creating device with tag: %s\n", deviceTag)
	device, err := CreateDevice(serverURL, apiKey, deviceTag, groups)
	if err != nil {
		fmt.Printf("Error creating device: %v\n", err)
		return
	}

	// Display the created device
	fmt.Printf("\n=== Device Created Successfully ===\n")
	fmt.Printf("Device ID: %s\n", device.ID.Hex())
	fmt.Printf("Device Tag: %s\n", device.Tag)
	fmt.Printf("Created At: %s\n", device.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Groups: %v\n", device.Groups)
}
