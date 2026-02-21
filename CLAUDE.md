# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
# Client binary
cd cmd/main && go build .

# Server binary (linux only)
cd server && go build .

# Frontend
cd frontend && pnpm install && pnpm run build

# Frontend dev server (must first accept TLS cert at https://127.0.0.1:7777)
cd frontend && pnpm run dev

# Linting
golangci-lint run --timeout=10m --config .golangci.yml

# All tests
make test

# Server tests only (verbose)
make test-server

# Tests with coverage
make test-coverage

# Run tests directly
go test ./server/...
go test ./client/...

# Single test
go test -v -run TestName ./server/...

# Pre-commit checks (mod tidy + tests + lint)
make pre-commit
```

## Architecture

### Three-Component System
- **Server** (`server/`) - VPN server with REST API, runs on Linux. Uses MongoDB or BoltDB for storage.
- **Client** (`client/`) - VPN client with embedded web UI. Cross-platform (Linux/macOS/Windows).
- **Frontend** (`frontend/`) - React 18 + Vite + TailwindCSS 4 + Radix UI. Built and embedded into the client binary via `//go:embed dist`.

### Entry Points
- `cmd/main/main.go` - CLI client entry point (embeds frontend dist)
- `cmd/wails/main.go` - Desktop app entry point (Wails framework, experimental)
- `cmd/service/` - Windows service wrapper
- `server/main.go` - Server entry point

### Connection Flow
1. Client calls `PublicConnect()` in `client/session.go`
2. Client contacts controller at `/v3/session` with signed request
3. TLS 1.3 handshake with server, hybrid encryption (X25519 + ML-KEM1024)
4. Tunnel interface created, UDP data channel established
5. Packet forwarding goroutines started (`ReadFromServeTunnel`/`ReadFromTunnelInterface`)

### Server Feature Flags
Server config enables/disables features: `VPN`, `LAN`, `AUTH`, `DNS`, `BBOLT`. These are defined in `types/types.go` as the `Feature` enum.

### Platform-Specific Code
Build tags separate platform code in `client/`:
- `IFINIT_Darwin.go`, `IFINIT_unix.go`, `IFINIT_windows.go` - Interface initialization
- `packet_darwin.go`, `packet_unix.go`, `packet_windows.go` - Packet handling
- `helpers_darwin.go`, `helpers_unix.go`, `helpers_windows.go` - OS helpers

### Key Packages
- `types/` - Shared data structures (`ServerConfig`, `ControllerConnectRequest`, `Device`, `Server`, `Network`, `DNSRecord`, etc.)
- `crypt/` - Hybrid post-quantum encryption (X25519 + ML-KEM1024, AES128/256, ChaCha20)
- `signal/` - Named goroutine scheduler pattern (`NewSignal(tag, ctx, cancel, sleep, logFunc, method)`)
- `server/handlers.go` - API endpoint handlers
- `server/dbwrapper.go` - MongoDB layer; `server/bboltwrapper.go` - BoltDB alternative

### Frontend Structure
- `frontend/src/App.jsx` - HashRouter, main routing
- `frontend/src/state.jsx` - Global state with manual re-render triggers (`STATE.renderPage()`, `STATE.globalRerender()`)
- `frontend/src/store.js` - Session storage cache (prefixed `data_`)
- `frontend/src/ws.js` - WebSocket for real-time logs
- `frontend/src/App/` - Page components (Login, Users, Groups, Devices, Settings, etc.)
- `frontend/src/components/ui/` - Radix UI wrappers
- Path alias: `@` maps to `frontend/src/`

### Concurrency Patterns
- `atomic.Pointer` for thread-safe config/state access (no locks)
- `signal.Signal` for recurring goroutine tasks with named tags and intervals
- Client uses priority channels (high/medium/low) in a select loop
- `puzpuzpuz/xsync` for concurrent maps

### Permissions Required
- Linux: `setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' main`
- macOS: `sudo`
- Windows: Administrator
