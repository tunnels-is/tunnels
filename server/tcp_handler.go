package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
)

// StartTCPHandler starts a TCP server that handles binary protocol messages
func StartTCPHandler(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start TCP listener: %w", err)
	}
	defer listener.Close()

	logger.Info("TCP Handler started", slog.String("address", address))

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("failed to accept TCP connection", slog.Any("err", err))
			continue
		}

		go handleTCPConnection(conn)
	}
}

// handleTCPConnection handles a single TCP connection
func handleTCPConnection(conn net.Conn) {
	defer conn.Close()
	
	logger.Info("new TCP connection", slog.String("remote", conn.RemoteAddr().String()))

	for {
		// Read 2 bytes for message length
		lengthBytes := make([]byte, 2)
		_, err := io.ReadFull(conn, lengthBytes)
		if err != nil {
			if err == io.EOF {
				logger.Info("TCP connection closed", slog.String("remote", conn.RemoteAddr().String()))
				return
			}
			logger.Error("failed to read message length", slog.Any("err", err))
			return
		}

		// Convert bytes to message length (big endian)
		messageLength := binary.BigEndian.Uint16(lengthBytes)
		
		if messageLength == 0 {
			logger.Warn("received zero-length message")
			continue
		}

		// Read the rest of the message
		messageData := make([]byte, messageLength)
		_, err = io.ReadFull(conn, messageData)
		if err != nil {
			logger.Error("failed to read message data", slog.Any("err", err))
			return
		}

		// Process the message
		response := processTCPMessage(messageData)
		
		// Send response back
		if err := sendTCPResponse(conn, response); err != nil {
			logger.Error("failed to send TCP response", slog.Any("err", err))
			return
		}
	}
}

// processTCPMessage processes a single message according to the binary protocol
func processTCPMessage(data []byte) []byte {
	if len(data) < 30 {
		logger.Error("message too short", slog.Int("length", len(data)))
		return createErrorResponse("message too short")
	}

	// Extract 30-byte header string
	headerBytes := data[:30]
	// Remove null bytes and trim whitespace
	headerString := strings.TrimSpace(strings.TrimRight(string(headerBytes), "\x00"))

	// Extract JSON data (remaining bytes)
	jsonData := data[30:]

	logger.Info("processing TCP message", 
		slog.String("header", headerString),
		slog.Int("json_length", len(jsonData)))

	// Route the message to appropriate API handler
	return routeTCPMessage(headerString, jsonData)
}

// routeTCPMessage routes the message to the appropriate API function based on the header
func routeTCPMessage(header string, jsonData []byte) []byte {
	// Create a mock HTTP request and response recorder
	recorder := httptest.NewRecorder()
	
	// Create request with JSON body
	req, err := http.NewRequest("POST", "/", bytes.NewReader(jsonData))
	if err != nil {
		logger.Error("failed to create HTTP request", slog.Any("err", err))
		return createErrorResponse("failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	// Route based on header string
	switch strings.TrimSpace(header) {
	// Health check
	case "health":
		healthCheckHandler(recorder, req)

	// LAN API routes
	case "v3/firewall":
		if LANEnabled {
			API_Firewall(recorder, req)
		} else {
			return createErrorResponse("LAN API not enabled")
		}
	case "v3/devices":
		if LANEnabled {
			API_ListDevices(recorder, req)
		} else {
			return createErrorResponse("LAN API not enabled")
		}

	// VPN API routes
	case "v3/connect":
		if VPNEnabled {
			API_AcceptUserConnections(recorder, req)
		} else {
			return createErrorResponse("VPN API not enabled")
		}

	// Auth API routes
	case "v3/user/create":
		if AUTHEnabled {
			API_UserCreate(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/user/update":
		if AUTHEnabled {
			API_UserUpdate(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/user/login":
		if AUTHEnabled {
			API_UserLogin(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/user/logout":
		if AUTHEnabled {
			API_UserLogout(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/user/reset/code":
		if AUTHEnabled {
			API_UserRequestPasswordCode(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/user/reset/password":
		if AUTHEnabled {
			API_UserResetPassword(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/user/2fa/confirm":
		if AUTHEnabled {
			API_UserTwoFactorConfirm(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/user/list":
		if AUTHEnabled {
			API_UserList(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}

	// Device API routes
	case "v3/device/list":
		if AUTHEnabled {
			API_DeviceList(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/device/create":
		if AUTHEnabled {
			API_DeviceCreate(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/device/delete":
		if AUTHEnabled {
			API_DeviceDelete(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/device/update":
		if AUTHEnabled {
			API_DeviceUpdate(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/device":
		if AUTHEnabled {
			API_DeviceGet(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}

	// Group API routes
	case "v3/group/create":
		if AUTHEnabled {
			API_GroupCreate(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/group/delete":
		if AUTHEnabled {
			API_GroupDelete(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/group/update":
		if AUTHEnabled {
			API_GroupUpdate(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/group/add":
		if AUTHEnabled {
			API_GroupAdd(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/group/remove":
		if AUTHEnabled {
			API_GroupRemove(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/group/list":
		if AUTHEnabled {
			API_GroupList(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/group":
		if AUTHEnabled {
			API_GroupGet(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/group/entities":
		if AUTHEnabled {
			API_GroupGetEntities(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}

	// Server API routes
	case "v3/server":
		if AUTHEnabled {
			API_ServerGet(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/server/create":
		if AUTHEnabled {
			API_ServerCreate(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/server/update":
		if AUTHEnabled {
			API_ServerUpdate(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/servers":
		if AUTHEnabled {
			API_ServersForUser(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}
	case "v3/session":
		if AUTHEnabled {
			API_SessionCreate(recorder, req)
		} else {
			return createErrorResponse("Auth API not enabled")
		}

	// Payment API routes (only if PayKey is configured)
	case "v3/key/activate":
		if AUTHEnabled && loadSecret("PayKey") != "" {
			API_ActivateLicenseKey(recorder, req)
		} else {
			return createErrorResponse("Payment API not enabled")
		}
	case "v3/user/toggle/substatus":
		if AUTHEnabled && loadSecret("PayKey") != "" {
			API_UserToggleSubStatus(recorder, req)
		} else {
			return createErrorResponse("Payment API not enabled")
		}

	default:
		logger.Warn("unknown route", slog.String("header", header))
		return createErrorResponse(fmt.Sprintf("unknown route: %s", header))
	}

	// Get the response from the recorder
	result := recorder.Result()
	defer result.Body.Close()

	responseBody, err := io.ReadAll(result.Body)
	if err != nil {
		logger.Error("failed to read response body", slog.Any("err", err))
		return createErrorResponse("failed to read response")
	}

	// Create a structured response with status code and body
	response := map[string]interface{}{
		"status": result.StatusCode,
		"body":   string(responseBody),
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		logger.Error("failed to marshal response", slog.Any("err", err))
		return createErrorResponse("failed to marshal response")
	}

	return responseJSON
}

// sendTCPResponse sends a response back to the TCP client
func sendTCPResponse(conn net.Conn, response []byte) error {
	// Send response length (2 bytes, big endian)
	lengthBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(lengthBytes, uint16(len(response)))
	
	if _, err := conn.Write(lengthBytes); err != nil {
		return fmt.Errorf("failed to write response length: %w", err)
	}

	// Send response data
	if _, err := conn.Write(response); err != nil {
		return fmt.Errorf("failed to write response data: %w", err)
	}

	return nil
}

// createErrorResponse creates a JSON error response
func createErrorResponse(message string) []byte {
	errorResponse := map[string]interface{}{
		"status": 400,
		"body":   fmt.Sprintf(`{"error": "%s"}`, message),
	}

	responseJSON, err := json.Marshal(errorResponse)
	if err != nil {
		// Fallback error response
		return []byte(fmt.Sprintf(`{"status": 500, "body": "{\"error\": \"internal error\"}"}`))
	}

	return responseJSON
}

// LaunchTCPHandler starts the TCP handler on the specified address
// This function should be called as a goroutine from the main application
func LaunchTCPHandler(address string) {
	err := StartTCPHandler(address)
	if err != nil {
		logger.Error("TCP Handler failed to start", slog.Any("err", err), slog.String("address", address))
	}
}
