# CoreTwo VPN Client

CoreTwo is a modern, cross-platform VPN client implementation in Go. It provides a clean, modular architecture for managing VPN tunnels across Windows, macOS, and Linux.

## Features

- Cross-platform support (Windows, macOS, Linux)
- Modular architecture
- REST API for external control
- DNS resolution with caching
- Network interface management
- Platform-specific optimizations
- Graceful shutdown handling
- Structured logging with multiple output formats
- End-to-end encryption for secure communication
- Comprehensive metrics and monitoring system

## Architecture

The module is organized into several key components:

### Core Components

- `cmd/tunnels/`: Main application entry point
- `internal/`: Internal packages
  - `api/`: REST API implementation
  - `config/`: Configuration management
  - `platform/`: Platform-specific code
- `pkg/`: Public packages
  - `tunnel/`: VPN tunnel implementation
  - `interface/`: Network interface management
  - `dns/`: DNS resolution
  - `logger/`: Structured logging
  - `crypto/`: Encryption and security
  - `metrics/`: Performance monitoring

### Platform Support

- **Windows**: Uses WinTun driver for network interface
- **macOS**: Uses native network extensions
- **Linux**: Uses TUN/TAP interface

## Building

### Prerequisites

- Go 1.21 or later
- Platform-specific requirements:
  - Windows: WinTun driver
  - macOS: Network extension entitlements
  - Linux: CAP_NET_ADMIN capability

### Build Commands

```bash
# Build for current platform
go build -o tunnels ./cmd/tunnels

# Build for specific platform
GOOS=windows GOARCH=amd64 go build -o tunnels.exe ./cmd/tunnels
GOOS=darwin GOARCH=amd64 go build -o tunnels ./cmd/tunnels
GOOS=linux GOARCH=amd64 go build -o tunnels ./cmd/tunnels
```

## Usage

### Command Line

```bash
# Start with default configuration
./tunnels

# Start with custom base path
./tunnels -basePath /path/to/config

# Start with custom config file
./tunnels -config /path/to/config.json

# Start with custom log level
./tunnels -log-level debug

# Start with JSON log output
./tunnels -json-logs

# Start with metrics enabled
./tunnels -metrics

# Start with encryption enabled
./tunnels -encryption-key your-secure-key
```

### Logging

The application supports structured logging with the following features:

- Multiple log levels (debug, info, warn, error)
- JSON output format option
- Contextual fields for better debugging
- File and line information
- Thread-safe logging

Example log output:
```
# Text format
2024-03-14T12:34:56Z [INFO] Starting tunnel service
2024-03-14T12:34:56Z [INFO] Connecting tunnel {"tunnel_id": "tunnel-1", "server": "127.0.0.1:8080"}

# JSON format
{"time":"2024-03-14T12:34:56Z","level":"INFO","message":"Starting tunnel service"}
{"time":"2024-03-14T12:34:56Z","level":"INFO","message":"Connecting tunnel","fields":{"tunnel_id":"tunnel-1","server":"127.0.0.1:8080"}}
```

### Encryption

The application supports end-to-end encryption for secure communication:

- AES-GCM encryption for all tunnel traffic
- Secure key management
- Optional per-tunnel configuration
- Performance monitoring of encryption operations

To enable encryption for a tunnel, configure it in the tunnel configuration:

```json
{
  "tunnels": [
    {
      "id": "tunnel-1",
      "serverIP": "127.0.0.1",
      "serverPort": 8080,
      "protocol": "tcp",
      "encryption": {
        "enabled": true,
        "key": "your-secure-key"
      }
    }
  ]
}
```

### Metrics and Monitoring

The application includes a comprehensive metrics and monitoring system:

- Tunnel performance metrics (bytes in/out, packets in/out)
- Latency monitoring
- Encryption performance (success/failure rates)
- Service-level metrics (uptime, number of tunnels)
- Custom histograms for analyzing performance patterns

Metrics are collected at regular intervals and can be accessed via logs or the API.

Example metrics:
```
# Tunnel metrics
tunnel_bytes_in: 1024000
tunnel_bytes_out: 2048000
tunnel_packets_in: 1000
tunnel_packets_out: 2000
tunnel_latency: 50ms
tunnel_encryption_errors: 0
tunnel_decryption_errors: 0

# Service metrics
tunnels_total: 5
tunnels_active: 3
service_uptime: 3600s
```

### API Endpoints

- `GET /api/v1/tunnels`: List all tunnels
- `POST /api/v1/tunnels`: Create a new tunnel
- `GET /api/v1/tunnels/{id}`: Get tunnel information
- `DELETE /api/v1/tunnels/{id}`: Delete a tunnel
- `GET /api/v1/status`: Get service status
- `GET /api/v1/metrics`: Get performance metrics

## Configuration

The configuration file (`config.json`) supports the following options:

```json
{
  "basePath": "/path/to/base",
  "logPath": "/path/to/logs",
  "configPath": "/path/to/config.json",
  "apiIP": "127.0.0.1",
  "apiPort": "8080",
  "minimal": false,
  "consoleLogOnly": false,
  "metrics": {
    "enabled": true,
    "interval": "5s"
  },
  "encryption": {
    "enabled": true,
    "defaultKey": "your-secure-key"
  }
}
```

## Development

### Adding New Features

1. Create new package in appropriate directory
2. Implement interfaces and types
3. Add tests
4. Update documentation

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./pkg/tunnel
go test ./pkg/crypto
go test ./pkg/metrics
```

### Implementing Custom Encryption

You can implement custom encryption providers by implementing the encryption interface:

```go
type Encryptor interface {
    Encrypt(data []byte) ([]byte, error)
    Decrypt(data []byte) ([]byte, error)
}
```

## Security Considerations

- Always use strong, unique encryption keys
- Regularly rotate encryption keys
- Monitor encryption errors for potential security issues
- Keep the application updated with the latest security patches

## License

This project is licensed under the MIT License - see the LICENSE file for details. 