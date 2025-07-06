# Tunnels API Documentation

# WARNING: this was generate using machine learning, it might not be 100% accurate.

This document provides comprehensive documentation for the Tunnels API endpoints, including request/response formats and example usage.

## Base URL and Authentication

- **Base URL**: `https://{server-ip}:{api-port}`
- **Default Port**: Configured in server settings
- **TLS**: All endpoints require HTTPS
- **Authentication**: Most endpoints require `DeviceToken` and `UID` in the request body, or `X-API-KEY` header for admin operations

## Common Response Format

### Success Response
```json
{
  "Data": {...}  // Response data
}
```

### Error Response
```json
{
  "Error": "Error message description"
}
```

## Health Check Endpoints

### GET /health
Health check endpoint to verify server status.

**Response:**
```
OK
```

**Example:**
```bash
curl -X GET https://your-server.com:8443/health
```

---

## User Management Endpoints

### POST /v3/user/create
Create a new user account.

**Request Body:**
```json
{
  "Email": "user@example.com",
  "Password": "your-secure-password",
  "AdditionalInformation": "Optional additional info"
}
```

**Response:**
```json
{
  "Data": {
    "_id": "507f1f77bcf86cd799439011",
    "Email": "user@example.com",
    "IsAdmin": false,
    "IsManager": false,
    "Trial": true,
    "SubExpiration": "2024-06-28T12:00:00Z",
    "DeviceToken": {
      "DT": "device-token-string",
      "N": "registration",
      "Created": "2024-06-27T12:00:00Z"
    }
  }
}
```

**Example:**
```bash
curl -X POST https://your-server.com:8443/v3/user/create \
  -H "Content-Type: application/json" \
  -d '{
    "Email": "user@example.com",
    "Password": "your-secure-password"
  }'
```

### POST /v3/user/login
Authenticate user and get device token.

**Request Body:**
```json
{
  "Email": "user@example.com",
  "Password": "your-password",
  "DeviceName": "My Device",
  "Version": "1.0.0",
  "Digits": "123456",  // 2FA code (if enabled)
  "Recovery": "recovery-code"  // Recovery code (if using 2FA recovery)
}
```

**Response:**
```json
{
  "Data": {
    "_id": "507f1f77bcf86cd799439011",
    "Email": "user@example.com",
    "DeviceToken": {
      "DT": "new-device-token",
      "N": "My Device",
      "Created": "2024-06-27T12:00:00Z"
    },
    "IsAdmin": false,
    "Groups": ["group-id-1", "group-id-2"],
    "APIKey": "user-api-key"
  }
}
```

**Example:**
```bash
curl -X POST https://your-server.com:8443/v3/user/login \
  -H "Content-Type: application/json" \
  -d '{
    "Email": "user@example.com",
    "Password": "your-password",
    "DeviceName": "My Laptop"
  }'
```

### POST /v3/user/logout
Logout user by invalidating device tokens.

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "current-device-token",
  "LogoutToken": "token-to-logout",  // Optional: specific token to logout
  "All": false  // Set to true to logout from all devices
}
```

**Response:** `200 OK`

**Example:**
```bash
curl -X POST https://your-server.com:8443/v3/user/logout \
  -H "Content-Type: application/json" \
  -d '{
    "UID": "507f1f77bcf86cd799439011",
    "DeviceToken": "current-device-token",
    "All": true
  }'
```

### POST /v3/user/update
Update user information.

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "APIKey": "new-api-key",
  "AdditionalInformation": "Updated info"
}
```

**Response:** `200 OK`

**Example:**
```bash
curl -X POST https://your-server.com:8443/v3/user/update \
  -H "Content-Type: application/json" \
  -d '{
    "UID": "507f1f77bcf86cd799439011",
    "DeviceToken": "your-device-token",
    "AdditionalInformation": "Updated information"
  }'
```

### POST /v3/user/list
List users (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "Limit": 50,
  "Offset": 0
}
```

**Response:**
```json
{
  "Data": [
    {
      "_id": "507f1f77bcf86cd799439011",
      "Email": "user1@example.com",
      "IsAdmin": false,
      "IsManager": false,
      "Groups": ["group-id-1"],
      "Trial": false,
      "Disabled": false
    }
  ]
}
```

### POST /v3/user/reset/code
Request password reset code.

**Request Body:**
```json
{
  "Email": "user@example.com"
}
```

**Response:** `200 OK`

**Example:**
```bash
curl -X POST https://your-server.com:8443/v3/user/reset/code \
  -H "Content-Type: application/json" \
  -d '{
    "Email": "user@example.com"
  }'
```

### POST /v3/user/reset/password
Reset password using reset code.

**Request Body:**
```json
{
  "Email": "user@example.com",
  "Password": "new-password",
  "ResetCode": "reset-code-from-email",
  "UseTwoFactor": false
}
```

**Response:** `200 OK`

### POST /v3/user/2fa/confirm
Confirm two-factor authentication setup.

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "Code": "totp-secret-key",
  "Digits": "123456",
  "Password": "your-password",
  "Recovery": "recovery-code"  // Optional: for using recovery code
}
```

**Response:**
```json
{
  "Data": "RECOVERY-CODE-1 RECOVERY-CODE-2"
}
```

---

## Device Management Endpoints

### POST /v3/device/create
Create a new device (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "Device": {
    "Tag": "My Device",
    "Groups": ["group-id-1", "group-id-2"]
  }
}
```

**Response:**
```json
{
  "Data": {
    "_id": "507f1f77bcf86cd799439012",
    "Tag": "My Device",
    "Groups": ["group-id-1", "group-id-2"],
    "CreatedAt": "2024-06-27T12:00:00Z"
  }
}
```

**Example:**
```bash
curl -X POST https://your-server.com:8443/v3/device/create \
  -H "Content-Type: application/json" \
  -d '{
    "UID": "507f1f77bcf86cd799439011",
    "DeviceToken": "your-device-token",
    "Device": {
      "Tag": "Production Server",
      "Groups": []
    }
  }'
```

### POST /v3/device/list
List devices (Admin/Manager only, or with API key).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "Limit": 50,
  "Offset": 0
}
```

**Alternative with API Key:**
```bash
curl -X POST https://your-server.com:8443/v3/device/list \
  -H "X-API-KEY: your-admin-api-key" \
  -H "Content-Type: application/json" \
  -d '{}'
```

**Response:**
```json
{
  "Data": [
    {
      "_id": "507f1f77bcf86cd799439012",
      "Tag": "Device 1",
      "Groups": ["group-id-1"],
      "CreatedAt": "2024-06-27T12:00:00Z"
    }
  ]
}
```

### POST /v3/device
Get specific device information.

**Request Body:**
```json
{
  "DeviceID": "507f1f77bcf86cd799439012"
}
```

**Response:**
```json
{
  "Data": {
    "_id": "507f1f77bcf86cd799439012",
    "Tag": "My Device",
    "Groups": ["group-id-1"],
    "CreatedAt": "2024-06-27T12:00:00Z"
  }
}
```

### POST /v3/device/update
Update device information (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "Device": {
    "_id": "507f1f77bcf86cd799439012",
    "Tag": "Updated Device Name",
    "Groups": ["new-group-id"]
  }
}
```

**Response:** `200 OK`

### POST /v3/device/delete
Delete a device (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "DID": "507f1f77bcf86cd799439012"
}
```

**Response:** `200 OK`

---

## Group Management Endpoints

### POST /v3/group/create
Create a new group (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "Group": {
    "Tag": "Developers",
    "Description": "Development team access group"
  }
}
```

**Response:**
```json
{
  "Data": {
    "_id": "507f1f77bcf86cd799439013",
    "Tag": "Developers",
    "Description": "Development team access group",
    "CreatedAt": "2024-06-27T12:00:00Z"
  }
}
```

### POST /v3/group/list
List groups (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token"
}
```

**Response:**
```json
{
  "Data": [
    {
      "_id": "507f1f77bcf86cd799439013",
      "Tag": "Developers",
      "Description": "Development team access group",
      "CreatedAt": "2024-06-27T12:00:00Z"
    }
  ]
}
```

### POST /v3/group
Get specific group information.

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "GID": "507f1f77bcf86cd799439013"
}
```

**Response:**
```json
{
  "Data": {
    "_id": "507f1f77bcf86cd799439013",
    "Tag": "Developers",
    "Description": "Development team access group",
    "CreatedAt": "2024-06-27T12:00:00Z"
  }
}
```

### POST /v3/group/entities
Get entities (users/devices/servers) in a group.

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "GID": "507f1f77bcf86cd799439013",
  "Type": "user",  // "user", "device", or "server"
  "Limit": 50,
  "Offset": 0
}
```

**Response (for Type="user"):**
```json
{
  "Data": [
    {
      "_id": "507f1f77bcf86cd799439011",
      "Email": "user@example.com",
      "Disabled": false,
      "IsAdmin": false,
      "IsManager": false
    }
  ]
}
```

### POST /v3/group/add
Add entity to group (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "GroupID": "507f1f77bcf86cd799439013",
  "Type": "user",  // "user", "device", or "server"
  "TypeID": "507f1f77bcf86cd799439011",  // ID of the entity to add
  "TypeTag": "user@example.com"  // Optional: email for user lookup
}
```

**Response:**
```json
{
  "Data": {
    "_id": "507f1f77bcf86cd799439011",
    "Email": "user@example.com",
    "Disabled": false,
    "IsAdmin": false,
    "IsManager": false
  }
}
```

### POST /v3/group/remove
Remove entity from group (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "GroupID": "507f1f77bcf86cd799439013",
  "Type": "user",
  "TypeID": "507f1f77bcf86cd799439011"
}
```

**Response:** `200 OK`

### POST /v3/group/update
Update group information (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "Group": {
    "_id": "507f1f77bcf86cd799439013",
    "Tag": "Updated Group Name",
    "Description": "Updated description"
  }
}
```

**Response:** `200 OK`

### POST /v3/group/delete
Delete a group (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "GID": "507f1f77bcf86cd799439013"
}
```

**Response:** `200 OK`

---

## Server Management Endpoints

### POST /v3/server/create
Create a new server (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "Server": {
    "Tag": "US East Server",
    "Country": "US",
    "IP": "192.168.1.100",
    "Port": "8443",
    "DataPort": "8444",
    "PubKey": "server-public-key"
  }
}
```

**Response:**
```json
{
  "Data": {
    "_id": "507f1f77bcf86cd799439014",
    "Tag": "US East Server",
    "Country": "US",
    "IP": "192.168.1.100",
    "Port": "8443",
    "DataPort": "8444",
    "PubKey": "server-public-key",
    "Groups": []
  }
}
```

### POST /v3/servers
List servers available to user.

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "StartIndex": 0
}
```

**Response:**
```json
{
  "Data": [
    {
      "_id": "507f1f77bcf86cd799439014",
      "Tag": "US East Server",
      "Country": "US",
      "IP": "192.168.1.100",
      "Port": "8443",
      "DataPort": "8444",
      "Groups": ["group-id-1"]
    }
  ]
}
```

### POST /v3/server
Get specific server information.

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "DeviceKey": "device-id-hex-string",  // Optional: for device-based auth
  "ServerID": "507f1f77bcf86cd799439014"
}
```

**Response:**
```json
{
  "Data": {
    "_id": "507f1f77bcf86cd799439014",
    "Tag": "US East Server",
    "Country": "US",
    "IP": "192.168.1.100",
    "Port": "8443",
    "DataPort": "8444",
    "PubKey": "server-public-key",
    "Groups": ["group-id-1"]
  }
}
```

### POST /v3/server/update
Update server information (Admin/Manager only).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "Server": {
    "_id": "507f1f77bcf86cd799439014",
    "Tag": "Updated Server Name",
    "Country": "US",
    "IP": "192.168.1.101",
    "Port": "8443",
    "DataPort": "8444"
  }
}
```

**Response:** `200 OK`

---

## Connection Endpoints

### POST /v3/connect
Establish VPN connection (requires VPN feature enabled).

**Request Body:**
```json
{
  "Signature": "base64-encoded-signature",
  "Payload": "base64-encoded-signed-payload"
}
```

**Response:**
```json
{
  "Data": {
    "Index": 1,
    "ServerHandshake": "base64-server-handshake",
    "ServerHandshakeSignature": "base64-signature",
    "InterfaceIP": "10.0.0.1",
    "DataPort": "8444",
    "StartPort": 50000,
    "EndPort": 50100,
    "InternetAccess": true,
    "LocalNetworkAccess": false,
    "AvailableMbps": 1000,
    "AvailableUserMbps": 100
  }
}
```

### POST /v3/session
Create connection session.

**Request Body:**
```json
{
  "UserID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "DeviceKey": "device-id-hex-string",  // Optional
  "ServerID": "507f1f77bcf86cd799439014",
  "Version": 1,
  "Created": "2024-06-27T12:00:00Z",
  "Hostname": "my-client",
  "RequestingPorts": true,
  "DHCPToken": "dhcp-token-string"
}
```

**Response:**
```json
{
  "Data": {
    "Signature": "base64-encoded-signature",
    "Payload": "base64-encoded-payload",
    "UserHandshake": "base64-user-handshake"
  }
}
```

---

## LAN Network Endpoints (requires LAN feature)

### POST /v3/devices
List connected devices on LAN.

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token"
}
```

**Alternative with API Key:**
```bash
curl -X POST https://your-server.com:8443/v3/devices \
  -H "X-API-KEY: your-admin-api-key" \
  -H "Content-Type: application/json" \
  -d '{}'
```

**Response:**
```json
{
  "Data": {
    "Devices": [
      {
        "DHCP": {
          "IP": [192, 168, 1, 100],
          "Hostname": "client-device.local",
          "Token": "dhcp-token"
        },
        "AllowedIPs": ["192-168-1-100"],
        "CPU": 45,
        "RAM": 67,
        "Disk": 78,
        "IngressQueue": 0,
        "EgressQueue": 0,
        "Created": "2024-06-27T12:00:00Z",
        "StartPort": 50000,
        "EndPort": 50100
      }
    ],
    "DHCPAssigned": 5,
    "DHCPFree": 250
  }
}
```

### POST /v3/firewall
Configure firewall rules.

**Request Body:**
```json
{
  "DHCPToken": "client-dhcp-token",
  "IP": "192.168.1.100",
  "Hosts": ["192.168.1.1", "8.8.8.8"],
  "DisableFirewall": false
}
```

**Response:** `200 OK`

---

## Premium/Subscription Endpoints

### POST /v3/key/activate
Activate license key (requires PayKey configuration).

**Request Body:**
```json
{
  "UID": "507f1f77bcf86cd799439011",
  "DeviceToken": "your-device-token",
  "Key": "license-key-string"
}
```

**Response:** `200 OK`

### POST /v3/user/toggle/substatus
Toggle user subscription status (Admin only, requires PayKey configuration).

**Request Body:**
```json
{
  "Email": "user@example.com",
  "DeviceToken": "admin-device-token",
  "Disable": false
}
```

**Response:** `200 OK`

---

## Error Codes

| HTTP Code | Description |
|-----------|-------------|
| 200 | Success |
| 204 | No Content (empty result) |
| 400 | Bad Request (invalid request body/parameters) |
| 401 | Unauthorized (invalid credentials/token) |
| 404 | Not Found |
| 500 | Internal Server Error |

## Common Error Scenarios

1. **Invalid Device Token**: Returns 401 with "unauthorized" message
2. **Missing Admin/Manager Permissions**: Returns 401 with permission denied message
3. **Invalid Request Body**: Returns 400 with "Invalid request body" message
4. **Database Errors**: Returns 500 with generic error message
5. **Two-Factor Authentication Required**: Returns 401 with specific 2FA error message

## Authentication Flow

1. **User Registration**: POST to `/v3/user/create`
2. **User Login**: POST to `/v3/user/login` to get device token
3. **API Calls**: Include `UID` and `DeviceToken` in request body for subsequent calls
4. **Admin Operations**: Use `X-API-KEY` header for admin API key authentication

## Rate Limiting

- Password reset requests: Limited to once every 30 seconds per user
- General API calls: No specific rate limiting mentioned in code

## Security Features

- All endpoints use HTTPS/TLS 1.2+
- Password hashing with bcrypt (cost 13)
- Two-factor authentication support with TOTP
- Device token-based session management
- Request signature verification for connections
- Admin API key authentication for privileged operations
