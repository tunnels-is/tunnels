# Tunnels.is VPN Project - Root Context

## Project Overview
Tunnels.is is a comprehensive VPN solution with both server and client components, featuring a web-based management interface. The project is built in Go with a React frontend and provides enterprise-grade VPN functionality with user management, device management, group-based access control, and real-time monitoring.

## Architecture
The project follows a client-server architecture:
- **Server**: Go-based VPN server with REST API for management
- **Client**: Go-based VPN client with embedded web UI
- **Frontend**: React-based web interface for configuration and management

## Key Components

### Core Directories
- `server/` - VPN server implementation with API endpoints
- `client/` - VPN client implementation with local web server
- `frontend/` - React web UI for both client and server management
- `cmd/main/` - Main client executable entry point
- `types/` - Shared data structures and types
- `certs/`, `crypt/`, `iptables/`, `setcap/` - Supporting utilities

### Features (Configurable)
- **VPN**: Core VPN tunneling functionality
- **LAN**: Local network access and device management
- **AUTH**: User authentication and authorization
- **DNS**: Custom DNS resolution and records
- **BBOLT**: Local database storage option

## Technologies Used
- **Backend**: Go 1.24, MongoDB, BoltDB
- **Frontend**: React 18, Vite, TailwindCSS, Radix UI
- **Networking**: WireGuard-style cryptography, raw sockets
- **Security**: bcrypt, 2FA/TOTP, TLS 1.2+, signed requests

## Build Process
- Client: `cd cmd/main && go build .`
- Server: `cd server && go build .`
- Frontend: `cd frontend && pnpm install && vite build`
- Linting: `golangci-lint run --timeout=10m --config .golangci.yml`

## Configuration
- Server config: `server/config.json` with feature toggles
- Client auto-generates config if not present
- Environment variables or config file for secrets
- Supports multiple tunnel types: default, strict, iot

## Network Architecture
- VPN uses custom protocol over UDP
- Raw socket programming for packet handling
- DHCP-style IP assignment for LAN clients
- Firewall rules automatically configured via iptables
- Support for custom DNS resolution and blocking

## Development Notes
- Cross-platform support (Linux, macOS, Windows)
- Requires network admin permissions to run
- Embedded frontend in client binary
- Hot reloading in development mode
- Comprehensive API documentation available

## Important Files
- `README.md` - Setup and usage instructions
- `API_DOCUMENTATION.md` - Complete API reference
- `go.mod` - Go module dependencies
- `.golangci.yml` - Linting configuration
- `.goreleaser.yaml` - Release automation

## Client vs Server
- **Client**: Connects to servers, runs local web UI, manages user tunnels
- **Server**: Accepts client connections, manages users/devices/groups, provides API

This project represents a production-ready VPN solution with enterprise features, suitable for both individual and organizational use.