package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"reflect"

	"github.com/tunnels-is/tunnels/types"
)

type netConMessage struct {
	Version int    `json:"version"`
	Method  string `json:"method"`
	Data    any    `json:"data"`
}

// handleUDPConnection processes incoming UDP connections and routes to APIv2 methods
func handleUDPConnection(conn net.Conn) {
	defer conn.Close()
	defer BasicRecover()

	// Read 2 bytes representing the length of the incoming message
	lengthBytes := make([]byte, 2)
	_, err := io.ReadFull(conn, lengthBytes)
	if err != nil {
		logger.Error("Failed to read message length", slog.Any("err", err))
		return
	}

	// Convert 2 bytes to uint16 (big-endian)
	messageLength := binary.BigEndian.Uint16(lengthBytes)
	if messageLength == 0 {
		logger.Error("Invalid message length: 0")
		return
	}

	// Read the rest of the data based on the length
	messageData := make([]byte, messageLength)
	_, err = io.ReadFull(conn, messageData)
	if err != nil {
		logger.Error("Failed to read message data", slog.Any("err", err))
		return
	}

	// Unmarshal the netConMessage
	var message netConMessage
	err = json.Unmarshal(messageData, &message)
	if err != nil {
		logger.Error("Failed to unmarshal netConMessage", slog.Any("err", err))
		return
	}

	// Route to appropriate APIv2 method based on the Method field
	response := routeNetConMessage(&message)

	// Send response back to client
	sendNetConResponse(conn, response)
}

// routeNetConMessage routes the message to the appropriate APIv2 function
func routeNetConMessage(message *netConMessage) any {
	switch message.Method {
	// User management methods
	case "user.login":
		return handleUserLogin(message.Data)
	case "user.create":
		return handleUserCreate(message.Data)
	case "user.update":
		return handleUserUpdate(message.Data)
	case "user.logout":
		return handleUserLogout(message.Data)
	case "user.list":
		return handleUserList(message.Data)
	case "user.requestPasswordCode":
		return handleUserRequestPasswordCode(message.Data)
	case "user.resetPassword":
		return handleUserResetPassword(message.Data)
	case "user.twoFactorConfirm":
		return handleUserTwoFactorConfirm(message.Data)
	case "user.toggleSubStatus":
		return handleUserToggleSubStatus(message.Data)
	case "user.activateLicenseKey":
		return handleActivateLicenseKey(message.Data)

	// Device management methods
	case "device.create":
		return handleDeviceCreate(message.Data)
	case "device.update":
		return handleDeviceUpdate(message.Data)
	case "device.delete":
		return handleDeviceDelete(message.Data)
	case "device.list":
		return handleDeviceList(message.Data)
	case "device.get":
		return handleDeviceGet(message.Data)

	// Group management methods
	case "group.create":
		return handleGroupCreate(message.Data)
	case "group.update":
		return handleGroupUpdate(message.Data)
	case "group.delete":
		return handleGroupDelete(message.Data)
	case "group.list":
		return handleGroupList(message.Data)
	case "group.get":
		return handleGroupGet(message.Data)
	case "group.getEntities":
		return handleGroupGetEntities(message.Data)
	case "group.add":
		return handleGroupAdd(message.Data)
	case "group.remove":
		return handleGroupRemove(message.Data)

	// Server management methods
	case "server.create":
		return handleServerCreate(message.Data)
	case "server.update":
		return handleServerUpdate(message.Data)
	case "server.get":
		return handleServerGet(message.Data)
	case "server.forUser":
		return handleServersForUser(message.Data)

	// Connection methods
	case "session.create":
		return handleSessionCreate(message.Data)
	case "connection.accept":
		return handleAcceptUserConnections(message.Data)

	default:
		return makeErr(400, fmt.Sprintf("Unknown method: %s", message.Method), slog.String("method", message.Method))
	}
}

// User management handlers
func handleUserLogin(data interface{}) interface{} {
	form, err := castToStruct[LOGIN_FORM](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_UserLogin(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleUserCreate(data interface{}) interface{} {
	form, err := castToStruct[REGISTER_FORM](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_UserCreate(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleUserUpdate(data interface{}) interface{} {
	form, err := castToStruct[USER_UPDATE_FORM](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_UserUpdate(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleUserLogout(data interface{}) interface{} {
	form, err := castToStruct[LOGOUT_FORM](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_UserLogout(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleUserList(data interface{}) interface{} {
	form, err := castToStruct[FORM_LIST_USERS](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_UserList(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleUserRequestPasswordCode(data interface{}) interface{} {
	form, err := castToStruct[PASSWORD_RESET_FORM](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_UserRequestPasswordCode(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleUserResetPassword(data interface{}) interface{} {
	form, err := castToStruct[PASSWORD_RESET_FORM](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_UserResetPassword(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleUserTwoFactorConfirm(data interface{}) interface{} {
	form, err := castToStruct[TWO_FACTOR_FORM](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_UserTwoFactorConfirm(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleUserToggleSubStatus(data interface{}) interface{} {
	form, err := castToStruct[USER_UPDATE_SUB_FORM](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_UserToggleSubStatus(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleActivateLicenseKey(data interface{}) interface{} {
	form, err := castToStruct[KEY_ACTIVATE_FORM](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_ActivateLicenseKey(form)
	if errData != nil {
		return errData
	}
	return okData
}

// Device management handlers
func handleDeviceCreate(data interface{}) interface{} {
	form, err := castToStruct[FORM_CREATE_DEVICE](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_DeviceCreate(form, false) // No API key for UDP connections
	if errData != nil {
		return errData
	}
	return okData
}

func handleDeviceUpdate(data interface{}) interface{} {
	form, err := castToStruct[FORM_UPDATE_DEVICE](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_DeviceUpdate(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleDeviceDelete(data interface{}) interface{} {
	form, err := castToStruct[FORM_DELETE_DEVICE](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_DeviceDelete(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleDeviceList(data interface{}) interface{} {
	form, err := castToStruct[FORM_LIST_DEVICE](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_DeviceList(form, false) // No API key for UDP connections
	if errData != nil {
		return errData
	}
	return okData
}

func handleDeviceGet(data interface{}) interface{} {
	form, err := castToStruct[types.FORM_GET_DEVICE](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_DeviceGet(form)
	if errData != nil {
		return errData
	}
	return okData
}

// Group management handlers
func handleGroupCreate(data interface{}) interface{} {
	form, err := castToStruct[FORM_CREATE_GROUP](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_GroupCreate(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleGroupUpdate(data interface{}) interface{} {
	form, err := castToStruct[FORM_UPDATE_GROUP](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_GroupUpdate(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleGroupDelete(data interface{}) interface{} {
	form, err := castToStruct[FORM_DELETE_GROUP](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_GroupDelete(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleGroupList(data interface{}) interface{} {
	form, err := castToStruct[FORM_LIST_GROUP](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_GroupList(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleGroupGet(data interface{}) interface{} {
	form, err := castToStruct[FORM_GET_GROUP](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_GroupGet(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleGroupGetEntities(data interface{}) interface{} {
	form, err := castToStruct[FORM_GET_GROUP_ENTITIES](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_GroupGetEntities(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleGroupAdd(data interface{}) interface{} {
	form, err := castToStruct[FORM_GROUP_ADD](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_GroupAdd(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleGroupRemove(data interface{}) interface{} {
	form, err := castToStruct[FORM_GROUP_REMOVE](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_GroupRemove(form)
	if errData != nil {
		return errData
	}
	return okData
}

// Server management handlers
func handleServerCreate(data interface{}) interface{} {
	form, err := castToStruct[FORM_CREATE_SERVER](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_ServerCreate(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleServerUpdate(data interface{}) interface{} {
	form, err := castToStruct[FORM_UPDATE_SERVER](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_ServerUpdate(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleServerGet(data interface{}) interface{} {
	form, err := castToStruct[types.FORM_GET_SERVER](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_ServerGet(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleServersForUser(data interface{}) interface{} {
	form, err := castToStruct[FORM_GET_SERVERS](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_ServersForUser(form)
	if errData != nil {
		return errData
	}
	return okData
}

// Connection handlers
func handleSessionCreate(data interface{}) interface{} {
	form, err := castToStruct[types.ControllerConnectRequest](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_SessionCreate(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleAcceptUserConnections(data interface{}) interface{} {
	form, err := castToStruct[types.SignedConnectRequest](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_AcceptUserConnections(form)
	if errData != nil {
		return errData
	}
	return okData
}

func castToStruct[T any](data any) (*T, error) {
	result, ok := data.(*T)
	if !ok {
		return nil, fmt.Errorf("unable to cast data to type")
	}

	return result, nil
}

func sendNetConResponse(conn net.Conn, response interface{}) {
	responseData, err := json.Marshal(response)
	if err != nil {
		logger.Error("Failed to marshal response", slog.Any("err", err))
		return
	}

	var statusByte byte = 1
	if errResp, ok := response.(*ErrorResponse); ok && errResp != nil {
		statusByte = 2
	}

	length := uint16(len(responseData))
	buffer := make([]byte, 0, 2+1+len(responseData))

	lengthBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(lengthBytes, length)
	buffer = append(buffer, lengthBytes...)
	buffer = append(buffer, statusByte)
	buffer = append(buffer, responseData...)

	_, err = conn.Write(buffer)
	if err != nil {
		logger.Error("Failed to write response", slog.Any("err", err))
		return
	}
}

func StartUDPServer(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start UDP server: %w", err)
	}
	defer listener.Close()

	logger.Info("UDP API server listening", slog.String("address", address))

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("Failed to accept connection", slog.Any("err", err))
			continue
		}

		// Handle connection in a goroutine
		go handleUDPConnection(conn)
	}
}
