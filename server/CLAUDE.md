# Tunnels.is VPN Server - Context

## Server Overview
The server component is a comprehensive VPN server written in Go that provides VPN connectivity, user management, device management, and API services. It supports multiple features that can be enabled/disabled via configuration.

## Key Files & Components

### Core Files
- **main.go** (`server/main.go:55`): Main entry point, initializes server components and feature flags
- **handlers.go**: HTTP API request handlers for all v3 endpoints
- **new_api.go**: Newer API implementations and server setup
- **types.go**: Server-specific type definitions and core data structures
- **config.json**: Server configuration file with feature toggles and settings

### Database & Storage
- **dbwrapper.go**: MongoDB database operations and user/device/group management
- **bboltwrapper.go**: BoltDB local database operations (alternative to MongoDB)
- **secret_store.go**: Handles secret storage (config file vs environment variables)

### Network & Socket Management
- **socket.go**: Raw socket programming, client connection management, packet handling
- **dhcp.go**: DHCP-style IP assignment for LAN clients
- **ping.go**: Client ping/keepalive functionality
- **firewall.go**: iptables firewall rule management

### Security & Crypto
- **encrypt.go**: Encryption utilities and key management
- **keys.go**: Cryptographic key operations
- **helpers.go**: Security helpers and utility functions

### Communication & API
- **lan_api.go**: LAN-specific API endpoints for device management
- **email.go**: Email notifications and password reset functionality
- **ratelimiter.go**: API rate limiting implementation
- **ports.go**: Port allocation and management

### Subscription & Payments
- **subs.go**: Subscription management and payment integration

## Configuration (`config.json`)

### Feature Flags
```json
{
  "Features": ["LAN", "VPN", "AUTH", "DNS", "BBOLT"]
}
```

### Key Settings
- **Network**: VPN IP/Port, API IP/Port configuration (`server/config.json:10-13`)
- **LAN Settings**: Network range (10.0.0.0/16), DHCP configuration
- **Security**: Admin API key, database URLs, 2FA keys
- **Certificates**: TLS certificate paths for HTTPS API
- **Bandwidth**: Rate limiting and user bandwidth controls

## API Architecture

### Core API Endpoints (`new_api.go:24-75`)
- **Health**: `/health` - Server health check
- **VPN**: `/v3/connect` - VPN connection establishment
- **Users**: `/v3/user/*` - User management (create, login, logout, 2FA)
- **Devices**: `/v3/device/*` - Device registration and management
- **Groups**: `/v3/group/*` - Group-based access control
- **Servers**: `/v3/server/*` - Server configuration management
- **LAN**: `/v3/devices`, `/v3/firewall` - LAN device and firewall management

### Authentication Methods
1. **Device Token + User ID**: Standard user authentication
2. **Admin API Key**: Administrative operations via X-API-KEY header
3. **Signed Requests**: Cryptographically signed connection requests

## Core Data Structures (`types.go`)

### User Core Mapping (`types.go:29-58`)
- Manages active client connections
- Tracks allowed hosts, firewall rules, DHCP assignments
- Handles ping monitoring and connection lifecycle
- Contains channels for packet routing

### Packet Routing (`types.go:60-63`)
- Raw packet structures with socket addresses
- Efficient packet forwarding between clients and internet

### Firewall Management (`types.go:78-84`)
- Per-client allowed host tracking
- Dynamic firewall rule management
- Connection state tracking (FIN flags)

## Network Architecture

### VPN Connectivity (`socket.go`)
- Raw UDP socket programming for VPN data
- Custom protocol implementation
- Client connection indexing and mapping
- Packet filtering and routing

### LAN Features (`dhcp.go`, `lan_api.go`)
- DHCP-style IP assignment (10.0.0.0/16 default)
- Device discovery and monitoring
- Per-device firewall controls
- Real-time device metrics (CPU, RAM, Disk)

### Port Management (`ports.go`)
- Dynamic port range allocation per client
- Port-to-client mapping for routing
- Configurable port ranges (2000-65530 default)

## Security Features

### Authentication & Authorization
- bcrypt password hashing (cost 13)
- TOTP-based 2FA with recovery codes
- Device token-based session management
- Group-based access control

### Network Security
- Automatic iptables rule management
- Per-client firewall rules
- Connection signature verification
- TLS 1.2+ for all API communications

### Data Protection
- Encrypted VPN tunnels
- Secure key exchange
- Request signing and verification

## Database Support

### MongoDB (`dbwrapper.go`)
- Primary database option
- Full user/device/group management
- Configurable via DBurl setting

### BoltDB (`bboltwrapper.go`)
- Local file-based database option
- Suitable for smaller deployments
- Enabled via BBOLT feature flag

## Development & Operations

### Building
```bash
cd server
go build .
./server --config ./config.json
```

### Feature Management
Features are controlled via config.json and loaded at startup (`main.go:68-73`):
- LANEnabled, VPNEnabled, AUTHEnabled, DNSEnabled, BBOLTEnabled

### Logging & Monitoring
- Structured logging with slog
- Configurable log levels
- Client connection monitoring
- Real-time statistics tracking

### Deployment Considerations
- Requires iptables for firewall management
- Needs network admin permissions
- Supports Docker deployment
- Configurable for production environments

This server provides enterprise-grade VPN functionality with comprehensive management capabilities, suitable for both small deployments and large-scale organizational use.