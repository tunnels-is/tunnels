# Unused Code Report

**Generated:** 2026-08-01  
**Last cleanup:** 2026-08-01  

Most items from the original audit have been **removed**. This file now tracks what remains intentionally, and what was cleaned up.

## Intentionally kept

### `new-frontend/`
Full alternate client UI. Not wired into release builds yet — **left alone** by request for future use.

### Platform-specific symbols (false positives on Linux)
These look unused under a Linux build, but are required on Darwin/Windows:

| Symbol | Used by |
|---|---|
| `closeAllOpenTCPconnections` | Windows interface teardown |
| `validateRouteArgs` | Darwin + Windows route code |
| `IsDefaultConnection` | Darwin + Windows |
| `verifyAndWriteFile` | Windows wintun deploy + tests |
| `RestoreDNSOnClose` | Darwin implementation (Unix stub) |

### Product tools outside goreleaser
`certcheck`, `credits`, `build-parser`, `cmd/wg-qr`, `cmd/wg-db-setup`, `cmd/user-migrate`, `test/mesh/meshpeer` — kept as operational tools.

---

## Removed in cleanup (2026-08-01)

### Packages / trees
- `iptables/` — orphan package
- `testing/` — scratch mains
- `desktop/` — empty Tauri scaffold
- `crypt/encrypt.go`, `crypt/main.go`, handshake/sign APIs — only `LoadPrivateKey` / `LoadPublicKey` remain
- Frontend shadcn leftovers: `alert`, `badge`, `switch`, `table`

### Client
- `HTTP_GetConfig`, `HTTP_GetTunnels`
- `GetDeviceByID`, `DEEP`, `openURL` (all platforms)
- `createDevNetTun`, `needsReconnect`, `event.Wait` / `event.done`
- `CreateConnectionUUID`, `IsAlphanumeric`, `CopySlice`, `inc`
- `client/tls.go` (cert load helpers)

### Server / wg-server / shared
- User API-key lookup + confirm-wipe helpers (`BBolt_findUserByAPIKey`, `BBolt_WipeUserConfirmCode`, DB wrappers)
- `CopySlice`, `StartWithExternalMonitor`
- `GetCurrentPeerKeys`, `AddPeerWithEndpoint`, `ipcGet`
- `PeerStore.Get` / `GetByPubKey` / `GetAll`
- `handleControl` wrapper (tests call `handleControlParsed`)
- `peerListSnapshot` / `addrAt` (test-local helpers now)
- `Signal.Stop`, unused `Cancel`/`ShouldStop` fields; `NewSignal` API simplified
- certs: `ExtractSerialNumber*`, `ResolveMetaTXT`
- argon: `NewDefault`, `Hash`, `Compare` (tests rewritten for `Key` / folder hash)
- types: `Feature`/`AUTH`, `LogConfig`, `SecretStore`, `WGServerInfo`

### go.mod
- Dropped `github.com/songgao/water` (was only used by deleted `testing/`)

---

## How to re-check

```bash
staticcheck -checks=U1000 ./client/... ./server/... ./wg-server/... ./crypt/... ./certs/... ./types/... ./signal/... ./argon/...
deadcode -test=false ./cmd/main ./cmd/wails ./cmd/service ./server ./wg-server
go test ./...
```
