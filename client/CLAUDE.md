# Tunnels.is VPN Client - Context

## Client Overview
The client component is a cross-platform VPN client written in Go that connects to Tunnels.is VPN servers. It features an embedded web UI, local API server, DNS resolution, and comprehensive tunnel management. The client runs as a local service and provides a web interface for configuration and monitoring.

## Key Files & Components

### Core Entry Points
- **cmd/main/main.go**: Main executable entry point with embedded frontend and DLL
- **main.go** (`client/main.go:55`): Core client initialization and service management
- **structs_globals.go**: Global variables, data structures, and embedded resources

### Network & Tunneling
- **packet.go**: Core packet processing and routing logic
- **packet_*.go**: Platform-specific packet handling (Darwin, Unix, Windows)
- **IFAndTunnel.go**: Tunnel interface management
- **IFINIT_*.go**: Platform-specific network interface initialization
- **NEW_interface.go**: Modern interface management implementation

### Connection Management
- **new.go**: Connection establishment and server communication
- **api.go**: Client API for server communication and authentication
- **http_layer.go**: Local HTTP server for web UI and API

### Platform Support
- **helpers.go**: Cross-platform helper functions
- **helpers_*.go**: Platform-specific implementations (Darwin, Unix, Windows)
- **EXPE_*.go**: Platform-specific execution and permissions

### DNS & Network Services
- **DNSResolver.go**: Custom DNS resolution with caching and blocking
- **blocklist.go**: DNS-based blocking functionality
- **nat.go**: NAT and port forwarding management

### Configuration & Persistence
- **user.go**: User configuration and preferences
- **update.go**: Automatic update functionality
- **logging.go**: Logging system and file management

### Monitoring & Stats
- **portmappingv2.go**: Port mapping and monitoring
- **new_code.go**: Additional monitoring and statistics

## Configuration System

### Automatic Configuration
- Client auto-generates configuration if none exists
- Supports multiple tunnel types: default, strict, iot
- Configurable via command-line flags and config files

### Runtime Configuration (`structs_globals.go:17-95`)
- **Production Mode**: Controls debug features and logging
- **Default Tunnel Name**: "tunnels" - primary tunnel identifier
- **DNS Configuration**: Custom DNS servers and resolution
- **API Settings**: Local web server configuration (port 7777 default)

## Network Architecture

### Tunnel Management
- **TUN/TAP Interface**: Platform-specific tunnel creation
- **Raw Socket Programming**: Direct packet manipulation
- **Multi-platform Support**: Windows (WinTUN), macOS, Linux

### DNS Resolution (`DNSResolver.go`)
- **Custom DNS Client**: Bypass system DNS for privacy
- **DNS Caching**: Performance optimization with TTL management
- **DNS Blocking**: Domain-based blocking capabilities
- **Upstream Servers**: Configurable DNS server selection

### Connection Types
- **VPN Tunnels**: Encrypted connections to VPN servers
- **Local API**: Embedded web server for UI (HTTPS on 7777)
- **DNS Server**: Local DNS resolver for custom resolution

## Web UI Integration

### Embedded Frontend
- **React Application**: Full web-based configuration interface
- **Embedded Resources**: Frontend built into binary (`cmd/main/main.go:14-18`)
- **Local HTTPS**: TLS-secured local web interface
- **Real-time Updates**: WebSocket-style communication for live stats

### API Endpoints
- Configuration management
- Connection status and statistics
- DNS query monitoring
- Log viewing and management

## Platform-Specific Features

### Windows (`IFINIT_windows.go`, `packet_windows.go`)
- **WinTUN Driver**: High-performance Windows tunnel driver
- **Windows Services**: Service installation and management
- **Registry Integration**: Windows-specific settings
- **Elevated Privileges**: Admin rights management

### macOS (`IFINIT_Darwin.go`, `packet_darwin.go`)
- **TUN Interface**: Native macOS tunnel interface
- **Route Management**: System routing table manipulation
- **Network Extensions**: macOS network framework integration
- **Sudo Requirements**: Elevated privileges for network operations

### Linux (`IFINIT_unix.go`, `packet_unix.go`)
- **TUN Interface**: Linux tunnel interface management
- **iptables Integration**: Firewall rule management
- **Capability Management**: Fine-grained permissions via setcap
- **Network Namespaces**: Advanced networking features

## Core Data Structures

### Connection Request (`structs_globals.go:35-50`)
- Server connection details
- Authentication tokens
- Device and user identification
- Encryption parameters

### DNS Statistics (`structs_globals.go:24-33`)
- Query counting and timing
- Resolution success/failure tracking
- Blocking statistics
- Cache performance metrics

### Global State (`structs_globals.go:67-95`)
- Application uptime tracking
- Traffic statistics (ingress/egress packets)
- HTTP server instance
- DNS server management

## Security Features

### Encryption & Authentication
- **Device Tokens**: Secure device authentication
- **Signed Requests**: Cryptographic request signing
- **TLS Communication**: Encrypted client-server communication
- **Local HTTPS**: Secure web interface

### Network Security
- **DNS Security**: Prevent DNS leaks with custom resolution
- **Traffic Encryption**: VPN tunnel encryption
- **Local Firewall**: Platform-specific firewall integration

## Development & Building

### Build Process
```bash
# Main client
cd cmd/main
go build .

# Development with embedded frontend
cd frontend
pnpm install
vite build
# Frontend is automatically embedded in binary
```

### Configuration Options
- **Base Path**: `--basePath` - Custom config/log directory
- **Tunnel Type**: `--tunnelType` - default/strict/iot tunnel configuration
- **Debug Mode**: `--debug` - Enable debug logging
- **Require Config**: `--requireConfig` - Force config file requirement

### Cross-Platform Considerations
- **Windows**: Requires admin privileges, includes WinTUN DLL
- **macOS**: Requires sudo, uses system TUN interface
- **Linux**: Uses capabilities for minimal privileges (preferred)

## Update System (`update.go`)
- **Automatic Updates**: Configurable auto-update functionality
- **Version Checking**: Remote version verification
- **Binary Replacement**: Safe update mechanism
- **Rollback Capabilities**: Update failure recovery

## Monitoring & Logging

### Statistics Tracking
- **Packet Counters**: Ingress/egress packet counting
- **Bandwidth Monitoring**: Real-time data rate tracking
- **Connection Status**: Tunnel state and health monitoring
- **DNS Metrics**: Query performance and blocking stats

### Logging System (`logging.go`)
- **File Logging**: Persistent log files
- **Console Logging**: Real-time console output
- **Log Rotation**: Automatic log file management
- **Debug Levels**: Configurable verbosity

This client provides a comprehensive VPN solution with an intuitive web interface, supporting multiple platforms and advanced networking features while maintaining strong security and privacy protections.