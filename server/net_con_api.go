package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/tunnels-is/tunnels/types"
)

func handleTCPConnection(conn net.Conn) {
	defer conn.Close()
	defer BasicRecover()

	lengthBytes := make([]byte, 2)
	_, err := io.ReadFull(conn, lengthBytes)
	if err != nil {
		logger.Error("Failed to read message length", slog.Any("err", err))
		return
	}

	messageLength := binary.BigEndian.Uint16(lengthBytes)
	if messageLength == 0 {
		logger.Error("Invalid message length: 0")
		return
	}

	messageData := make([]byte, messageLength)
	_, err = io.ReadFull(conn, messageData)
	if err != nil {
		logger.Error("Failed to read message data", slog.Any("err", err))
		return
	}
	fmt.Println(messageData)
	fmt.Println(string(messageData))

	var message types.NetConMessage
	err = json.Unmarshal(messageData, &message)
	if err != nil {
		logger.Error("Failed to unmarshal netConMessage", slog.Any("err", err))
		return
	}

	sendNetConResponse(conn, routeNetConMessage(&message))
}

func routeNetConMessage(message *types.NetConMessage) any {
	switch message.Method {
	case "/v3/user/login":
		return handleUserLogin(message.Data)
	case "/v3/user/create":
		return handleUserCreate(message.Data)
	case "/v3/user/update":
		return handleUserUpdate(message.Data)
	case "/v3/user/logout":
		return handleUserLogout(message.Data)
	case "/v3/user/list":
		return handleUserList(message.Data)
	case "/v3/user/reset/code":
		return handleUserRequestPasswordCode(message.Data)
	case "/v3/user/reset/password":
		return handleUserResetPassword(message.Data)
	case "/v3/user/2fa/confirm":
		return handleUserTwoFactorConfirm(message.Data)
	case "/v3/user/toggle/substatus":
		return handleUserToggleSubStatus(message.Data)
	case "/v3/key/activate":
		return handleActivateLicenseKey(message.Data)

	case "/v3/device/create":
		return handleDeviceCreate(message.Data)
	case "/v3/device/update":
		return handleDeviceUpdate(message.Data)
	case "/v3/device/delete":
		return handleDeviceDelete(message.Data)
	case "/v3/device/list":
		return handleDeviceList(message.Data)
	case "/v3/device":
		return handleDeviceGet(message.Data)

	case "/v3/group/create":
		return handleGroupCreate(message.Data)
	case "/v3/group/update":
		return handleGroupUpdate(message.Data)
	case "/v3/group/delete":
		return handleGroupDelete(message.Data)
	case "/v3/group/list":
		return handleGroupList(message.Data)
	case "/v3/group":
		return handleGroupGet(message.Data)
	case "/v3/group/entities":
		return handleGroupGetEntities(message.Data)
	case "/v3/group/add":
		return handleGroupAdd(message.Data)
	case "/v3/group/remove":
		return handleGroupRemove(message.Data)

	case "/v3/server/create":
		return handleServerCreate(message.Data)
	case "/v3/server/update":
		return handleServerUpdate(message.Data)
	case "/v3/server":
		return handleServerGet(message.Data)
	case "/v3/servers":
		return handleServersForUser(message.Data)

	case "/v3/session":
		return handleSessionCreate(message.Data)
	case "/v3/connect":
		return handleAcceptUserConnections(message.Data)

	case "/v3/firewall":
		return handleFirewall(message.Data)
	case "/v3/devices":
		return handleListDevices(message.Data)
	case "/health":
		return handleHealth(message.Data)

	default:
		return makeErr(400, fmt.Sprintf("Unknown method: %s", message.Method), slog.String("method", message.Method))
	}
}

// User management handlers
func handleUserLogin(data any) any {
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

func handleUserCreate(data any) any {
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

func handleUserUpdate(data any) any {
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

func handleUserLogout(data any) any {
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

func handleUserList(data any) any {
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

func handleUserRequestPasswordCode(data any) any {
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

func handleUserResetPassword(data any) any {
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

func handleUserTwoFactorConfirm(data any) any {
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

func handleUserToggleSubStatus(data any) any {
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

func handleActivateLicenseKey(data any) any {
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
func handleDeviceCreate(data any) any {
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

func handleDeviceUpdate(data any) any {
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

func handleDeviceDelete(data any) any {
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

func handleDeviceList(data any) any {
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

func handleDeviceGet(data any) any {
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
func handleGroupCreate(data any) any {
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

func handleGroupUpdate(data any) any {
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

func handleGroupDelete(data any) any {
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

func handleGroupList(data any) any {
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

func handleGroupGet(data any) any {
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

func handleGroupGetEntities(data any) any {
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

func handleGroupAdd(data any) any {
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

func handleGroupRemove(data any) any {
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
func handleServerCreate(data any) any {
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

func handleServerUpdate(data any) any {
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

func handleServerGet(data any) any {
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

func handleServersForUser(data any) any {
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
func handleSessionCreate(data any) any {
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

func handleAcceptUserConnections(data any) any {
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

// LAN and health management handlers
func handleFirewall(data any) any {
	form, err := castToStruct[types.FirewallRequest](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_Firewall(form)
	if errData != nil {
		return errData
	}
	return okData
}

func handleListDevices(data any) any {
	form, err := castToStruct[FORM_LIST_DEVICE](data)
	if err != nil {
		return makeErr(400, "Invalid request format", slog.Any("err", err))
	}
	errData, okData := APIv2_ListDevices(false, form) // No API key for TCP connections
	if errData != nil {
		return errData
	}
	return okData
}

func handleHealth(data any) any {
	errData, okData := APIv2_Health()
	if errData != nil {
		return errData
	}
	return okData
}

func castToStruct[T any](data []byte) (out *T, err error) {
	fmt.Println("DATA:", data)
	out = new(T)
	err = json.Unmarshal(data, out)
	if err != nil {
		return nil, err
	}

	return
}

func sendNetConResponse(conn net.Conn, response any) {
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

func StartTCPServer() {
	Config := Config.Load()
	addr := fmt.Sprintf("%s:%s",
		Config.APIIP,
		Config.APIPort,
	)

	listener, err := net.Listen("tcp4", addr)
	if err != nil {
		logger.Error("TCPAPI", slog.Any("err", err.Error()))
		return
	}
	defer listener.Close()

	logger.Info("TCP API server listening", slog.String("address", addr))

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("Failed to udp datagram", slog.Any("err", err))
			continue
		}

		go handleTCPConnection(conn)
	}
}
