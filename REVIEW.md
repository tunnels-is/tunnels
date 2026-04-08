# Tunnels Platform — Security Review

**Date:** 2026-04-06
**Scope:** `client/`, `wg-server/`, `server/` (controller)
**Branch:** wg-test

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Component Summary](#component-summary)
3. [Data Flow](#data-flow)
4. [Security Strengths](#security-strengths)
5. [Security Findings](#security-findings)
6. [Component-Specific Notes](#component-specific-notes)
7. [Recommendations](#recommendations)

---

## Architecture Overview

Tunnels is a WireGuard-based VPN platform with three components:

```
┌─────────────────┐         HTTPS (TLS 1.3 + X25519MLKEM768)         ┌──────────────────┐
│                 │ ◄──────────────────────────────────────────────── │                  │
│    Controller   │          /client/* (device token auth)            │   VPN Client     │
│    (server/)    │ ────────────────────────────────────────────────► │   (client/)      │
│                 │          WG config, server list, user mgmt        │                  │
│  BoltDB + REST  │                                                   │  WireGuard +     │
│  Admin UI (SPA) │         HTTPS (TLS 1.3)                           │  Local DNS proxy │
│  Subscriptions  │ ◄──────────────────────────────────────────────── │  Local REST API  │
│                 │          /wg/* (X-WG-KEY auth)                    └─────────┬────────┘
│                 │ ────────────────────────────────────────────────►           │
│                 │          Peer list, server config                  WireGuard tunnel
│                 │                                                            │
└────────┬───────┘                                                             │
         │                                                                     │
         │  HTTPS (TLS 1.3)                                                    │
         │  /wg/server-config/fetch, /wg/peers                                 │
         │                                                                     │
┌────────▼───────────┐                                                         │
│                    │         WireGuard UDP (Noise IK)                         │
│   WG Server        │ ◄──────────────────────────────────────────────────────-─┘
│   (wg-server/)     │
│                    │         iptables NAT / MASQUERADE
│  LazyBind peer     │ ◄────────────────────────────────────────────► Internet
│  provisioning      │
│  Noise IK decrypt  │
└────────────────────┘
```

**Key architectural property:** WireGuard private keys are ephemeral on the wg-server — regenerated on every boot, never persisted to disk. The controller is the single source of truth for authorization.

---

## Component Summary

### Controller (`server/`)

Central management server. Single Go binary with embedded BoltDB and SPA admin panel. Handles:

- User registration/login (bcrypt, optional TOTP 2FA)
- Device and server CRUD
- Group-based access control
- WireGuard IP assignment from pre-seeded /22 subnet pool
- WG server config distribution
- Optional LemonSqueezy subscription management
- Admin API key for privileged operations

### WG Server (`wg-server/`)

Userspace WireGuard VPN server with a novel **lazy binding** architecture:

1. Boots with zero peers, fetches config from controller
2. Generates ephemeral Curve25519 keys (fresh every boot)
3. When an unknown client sends a WireGuard handshake, the LazyBind layer intercepts it
4. Partially decrypts the Noise IK handshake to extract the client's static public key
5. Queries the controller to authorize the peer
6. Provisions the peer in WireGuard and replays the buffered handshake
7. Client connects without ever noticing the interception

This eliminates startup sync delays and makes the controller the real-time authorization authority.

### Client (`client/`)

Cross-platform WireGuard VPN client (Linux, macOS, Windows). Features:

- WireGuard tunnel with custom packet processing (NAT translation, port blocking)
- Local DNS proxy on port 53 with blocklist/whitelist (Pi-hole-like)
- DNS-over-HTTPS support
- Local REST API (port 7777) with embedded web UI
- Auto-update from GitHub releases with SHA-256 verification
- Encrypted credential storage (AES-256-GCM + Argon2id)
- Auto-reconnect with ping monitoring and kill-switch

---

## Data Flow

### Connection Lifecycle

```
1. Client generates/loads Curve25519 private key
2. Client derives public key
3. Client -> Controller: GET /client/wg/config?serverID=X&pubKey=Y
   (Auth: X-Device-Token + X-UID headers, over TLS 1.3)
4. Controller assigns WG IP, returns: server pubkey, port, IPs, NAT rules
5. Client creates OS TUN device (platform-specific)
6. Client wraps TUN in processingTUN (NAT, port blocking, bandwidth tracking)
7. Client creates wireguard-go device, configures via IPC
8. Client brings interface up, configures routes

On the server side:
9. WG Server LazyBind intercepts the handshake initiation
10. tryDecryptInitiator() extracts client static pubkey via Noise IK
11. WG Server -> Controller: GET /wg/peers (fetches authorized peer list)
12. If authorized: AddPeer() + requeue handshake packet
13. WireGuard handshake completes, encrypted tunnel established

Traffic flow:
14. App -> OS TUN -> processingTUN.Read() (egress NAT) -> WireGuard encrypt -> UDP -> WG Server
15. WG Server -> iptables MASQUERADE -> Internet
16. Return: Internet -> WG Server -> WireGuard decrypt -> processingTUN.Write() (ingress NAT) -> OS TUN -> App
```

### Authentication Map

| Path | Mechanism | Implementation |
|------|-----------|----------------|
| Client -> Controller `/client/*` | Device token + UID headers | `subtle.ConstantTimeCompare` |
| Admin UI -> Controller `/ui/*` | AES-256-GCM encrypted cookie (IP-bound) | Cookie: HttpOnly, Secure, SameSite=Strict |
| Admin API -> Controller | `X-API-KEY` header | `subtle.ConstantTimeCompare` |
| WG Server -> Controller `/wg/*` | `X-WG-KEY` header | `subtle.ConstantTimeCompare` |
| Client -> WG Server | WireGuard Noise IK handshake | Curve25519 + ChaCha20-Poly1305 |
| Browser -> Client API | Session token cookie | `subtle.ConstantTimeCompare`, HttpOnly, SameSite=Strict |

---

## Security Strengths

### Cryptographic Posture

| Area | Implementation | Assessment |
|------|----------------|------------|
| Transport | TLS 1.3 minimum, X25519MLKEM768 curve | Post-quantum hybrid — strong |
| Passwords | bcrypt cost 13 | Above standard (10-12 typical) |
| 2FA secrets at rest | AES-256-GCM + PBKDF2 (600K iterations) | Solid |
| Client credentials at rest | AES-256-GCM + Argon2id (20 MiB, 3 iterations) | Industry standard KDF |
| WireGuard keys | Curve25519 with proper clamping, `crypto/rand` | Correct per RFC 7748 |
| WG Server key lifetime | Ephemeral per boot, never persisted | Excellent forward secrecy |
| Session tokens | 32 bytes from `crypto/rand` | Sufficient entropy |
| Key material hygiene | `zeroBytes()` on all sensitive buffers after use | Best-effort in Go (GC limitation) |
| Token comparison | `subtle.ConstantTimeCompare` throughout | Timing-safe |
| Cookie encryption | AES-256-GCM with IP binding | Replay/theft resistant |

### Network Security

- **WG Server firewall**: Stateful iptables (RELATED,ESTABLISHED for return traffic), explicit IPv6 DROP when not configured
- **DNS leak prevention**: Global DNS block during connection switching
- **Kill-switch**: Client disconnects if ping fails for 45 seconds (when kill-switch enabled)
- **Host route pinning**: Controller and VPN server IPs get host routes through the real gateway, preventing routing loops
- **Non-blocking packet drop**: `processingTUN` drops oversized batches rather than blocking

### Access Control

- Device tokens rotated on every login
- Token cap of 20 per user (sorted by recency)
- Generic login error messages prevent user enumeration
- `User.RemoveSensitiveInformation()` strips passwords, 2FA codes, recovery codes before API responses
- Admin UI session cookie is IP-bound — stolen cookies won't work from a different IP
- WG server config API keys validated with constant-time comparison
- Controller-assigned WG IPs validated against subnet boundaries

---

## Security Findings

### HIGH — Inverted TLS Certificate Verification Logic

**Location:** `client/api.go:70-73`

```go
TLSClientConfig: &tls.Config{
    MinVersion:         tls.VersionTLS13,
    CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768},
    InsecureSkipVerify: !skipVerify,  // ← inverted
}
```

The parameter `skipVerify` means "skip verification" but the negation makes `InsecureSkipVerify = !skipVerify`. When the caller passes `skipVerify=false` (meaning "do NOT skip"), `InsecureSkipVerify` becomes `true` — **disabling certificate verification**.

**Mitigating factor:** `ForwardToController()` in the same file forces `ValidateCertificate = true` for `api.tunnels.is`, which makes `skipVerify = true`, so `InsecureSkipVerify = false`. The default controller connection is correctly verified due to this double-negation. Custom/third-party controllers with `ValidateCertificate` defaulting to `false` will have their certificates **not verified**.

**Impact:** MITM attacks possible against non-default controller servers.

**Recommendation:** Rename the parameter to `validateCert` and use `InsecureSkipVerify: !validateCert` for clarity, or fix the inversion.

---

### HIGH — User Credential Encryption Key Derived from Public Information

**Location:** `client/user.go` (calls into argon2 package)

The AES-256-GCM encryption key for stored user credentials is derived via Argon2id from:
- The working directory name
- The executable filename
- A **zero salt** (`skipSalt: true`)

This means:
1. The key is entirely deterministic from public filesystem information
2. An attacker with filesystem read access can reconstruct the key and decrypt all stored credentials
3. The zero salt weakens Argon2id (though the input is already low-entropy)

**Impact:** Stored credentials provide no meaningful protection against local attackers.

**Recommendation:** Use OS keychain integration (macOS Keychain, Windows DPAPI, Linux secret-service) or at minimum derive from a machine-specific hardware identifier. If the intent is only to bind credentials to the installation, document this limitation clearly.

---

### HIGH — No Rate Limiting on Controller Login/Registration

**Location:** `server/handlers.go` — `API_UserLogin`, `API_UserCreate`, `API_AdminUILogin`

There is no request rate limiting on any endpoint. The 50ms `time.Sleep` in login handlers is a timing-attack mitigation, not a rate limiter. At bcrypt cost 13, the server can process ~3-4 login attempts per second per CPU core.

**Impact:** Online brute-force attacks against user passwords. Open registration allows unlimited account creation.

**Recommendation:** Add per-IP rate limiting middleware for authentication endpoints (e.g., 5 attempts/minute for login, 2/minute for registration). Consider a progressive backoff or account lockout after N failed attempts.

---

### MEDIUM — Potential NAT Map Data Race

**Location:** `client/wgtun.go`, `client/nat.go`, `client/packet.go`

The `NATEgress` and `NATIngress` maps are plain Go maps protected by separate mutexes:
- `egressMu` protects the egress path (Read)
- `ingressMu` protects the ingress path (Write)

However, `TransLateIP()` (called from the egress path under `egressMu`) writes to **both** `NATEgress` and `NATIngress`. The ingress path reads `NATIngress` under `ingressMu`. This means concurrent egress writes to `NATIngress` and ingress reads from `NATIngress` are not synchronized — a classic data race.

**Impact:** Potential panic from concurrent map read/write, or corrupted NAT state causing misrouted packets.

**Recommendation:** Either protect `NATIngress` with a shared mutex (used by both paths), use `sync.Map`, or restructure so each map is only accessed under its own lock.

---

### MEDIUM — WG Server `rateWindow` Map Grows Unbounded

**Location:** `wg-server/lazybind.go`

The `rateWindow` map tracks per-IP handshake rates but is never cleaned up. Unlike `seenIPs` (which has a 200K hard cap and lazy sweep), `rateWindow` has no eviction or size limit.

Under a distributed handshake flood with many unique source IPs, this map grows indefinitely.

**Impact:** Gradual memory exhaustion under sustained distributed attack.

**Recommendation:** Add periodic cleanup (evict entries older than the rate window) or a size cap with LRU eviction, similar to the `seenIPs` pattern.

---

### MEDIUM — CORS Policy Allows All Origins

**Location:** `server/new_api.go:172`

```go
w.Header().Set("Access-Control-Allow-Origin", "*")
```

This allows any website to make cross-origin requests to the controller API.

**Mitigating factors:**
- Cookie-based admin sessions use `SameSite=Strict`, preventing cross-origin cookie attachment in modern browsers
- Client auth uses custom headers (`X-Device-Token`), which require a CORS preflight — but the `*` origin policy permits it
- The `Access-Control-Allow-Credentials` header is not set, so cookies won't be sent cross-origin

**Impact:** Low in practice due to mitigations, but violates defense-in-depth. A future change adding `Allow-Credentials` would be immediately dangerous.

**Recommendation:** Restrict to specific origins (the admin SPA origin, `localhost:7777` for the client). If the wildcard is needed for the desktop client, consider splitting CORS policies per route group.

---

### MEDIUM — Unsigned Update Checksums

**Location:** `client/update.go`

Auto-update downloads a SHA-256 checksum file from GitHub and verifies the binary archive against it. However, the checksum file itself is not cryptographically signed. Both the binary and checksum file come from the same source (GitHub release assets) over HTTPS.

**Impact:** A GitHub account compromise would allow serving a malicious binary with a valid checksum. The SHA-256 provides integrity (no tampering in transit) but not authenticity (no proof of who created it).

**Recommendation:** Sign the checksum file with a GPG/minisign key and verify the signature client-side, or use Go's built-in code signing if distributing via package managers.

---

### MEDIUM — Session Token Logged to Console

**Location:** `client/http_layer.go:85`

```go
INFO("Session Token: ", sessionToken)
```

The 32-byte session token for the local API is logged at INFO level on every startup. If logs are captured, forwarded, or displayed to other users on a shared system, the token is exposed.

**Impact:** Local API session hijacking if logs are accessible.

**Recommendation:** Log at DEBUG level only, or redact the token (e.g., show only the first 8 characters).

---

### MEDIUM — Open Registration with No Disable Option

**Location:** `server/handlers.go:124` — `API_UserCreate`

User registration at `POST /client/user/create` requires no authentication. Any client can create accounts. New users get a 1-day trial subscription.

For a public VPN service this is expected. For private/enterprise deployments, there is no configuration option to disable open registration.

**Impact:** Unauthorized users can create accounts on private deployments.

**Recommendation:** Add a `DisableRegistration` config flag that returns 403 on the registration endpoint when enabled.

---

### LOW — DNS Blocklist Downloads Not Size-Limited

**Location:** `client/blocklist.go:148`

`http.Get(url)` downloads blocklists without a response body size limit. A compromised or malicious blocklist URL could serve an extremely large response.

**Recommendation:** Wrap the response body with `io.LimitReader` (e.g., 50 MB cap).

---

### LOW — DNS-over-HTTPS Response Unpacking Not Error-Checked

**Location:** `client/DNSResolver.go:539`

```go
newx.Unpack(bb) // error return value ignored
```

**Recommendation:** Check the error return and log/drop malformed responses.

---

### LOW — Command Execution Surface on macOS/Windows

**Location:** `client/IFINIT_Darwin.go`, `client/IFINIT_windows.go`

Network configuration on macOS and Windows uses `exec.Command` with `route`, `ifconfig`, and `netsh`. Arguments come from server responses (IP addresses, interface names). While constrained to IP/CIDR format, there is no explicit validation before passing to commands.

**Mitigating factors:** `exec.Command` does not invoke a shell (no shell metacharacter injection), arguments are positional. IP addresses are parsed via `net.ParseIP()` in most paths.

**Recommendation:** Validate all server-provided values match expected IP/CIDR regex patterns before passing to `exec.Command`.

---

### LOW — IPv4 Address Allocation Never Reuses Gaps

**Location:** `wg-server/store.go` — `nextIP()`

IPv4 allocation always takes `max(used_IPs) + 1`. Deleted peers leave gaps that are never reused within a server session. In a `/24` subnet with high peer churn, addresses could be exhausted prematurely.

**Mitigating factor:** The default subnet is `/16` (65K addresses), and wg-server reboots reset the in-memory store.

**Recommendation:** Scan for gaps (like the IPv6 allocator already does) or document the limitation.

---

### LOW — PeerStore `GetByPubKey` is O(n)

**Location:** `wg-server/store.go`

Linear scan on every unknown-peer handshake. Acceptable for moderate peer counts but scales poorly.

**Recommendation:** Add a secondary `pubkey -> deviceID` index if peer counts are expected to grow significantly.

---

### INFO — Config Backup Write is Not Atomic

**Location:** `client/config.go:45-63`

The config save pattern is rename-old-to-backup, then write-new. A crash between rename and write completion loses data.

**Recommendation:** Write-to-temp-file then rename (atomic on POSIX), or write new file first and only then remove the old.

---

### INFO — WG Server Logging Format Bug

**Location:** `wg-server/logging.go`

The `buildOut(x ...any)` function wraps variadic args in an extra `[]any`, causing log messages to appear as `[foo bar]` with brackets. Cosmetic only.

---

## Component-Specific Notes

### Controller (`server/`)

**Well-implemented patterns:**
- BoltDB secondary indexes with automatic backfill on startup (self-healing)
- `RemoveSensitiveInformation()` consistently called before sending User objects
- bcrypt cost 13 (above typical defaults)
- IP-bound encrypted session cookies
- Comprehensive test coverage for the data layer (1469 lines of DB tests)

**Architecture notes:**
- Single-node only (BoltDB is embedded, no clustering)
- Config reloaded every 30 seconds from disk (supports hot-reload of TLS certs)
- Network seeding creates 16,384 /22 subnets from 10.0.0.0/8

### WG Server (`wg-server/`)

**Well-implemented patterns:**
- LazyBind is architecturally novel and well-executed
- Per-IP rate limiting + debounce + hard memory cap for DoS protection
- IPC injection prevention via `sanitizeIPC()`
- Noise IK partial decryption correctly implements the WireGuard protocol spec
- All intermediate Noise key material zeroed via `defer`

**Architecture notes:**
- Fully stateless (in-memory only, no persistence)
- Ephemeral keys on every boot — no long-lived key material on disk
- Cross-server masquerade exclusion supports multi-server mesh

### Client (`client/`)

**Well-implemented patterns:**
- Priority-based event dispatch with goroutine supervision
- Lock-free state access via `atomic.Pointer`
- Custom packet processing engine with correct RFC checksum recalculation
- DNS proxy with layered resolution (cache → whitelist → blocklist → custom records → upstream)
- Platform-specific TUN implementations (netlink on Linux, utun on macOS, Wintun on Windows)
- SHA-256 verification of extracted DLLs on Windows

**Architecture notes:**
- Supervised goroutines with automatic restart on failure
- Bandwidth tracking per-tunnel with 60-second rolling history
- DNS global block during connection switching prevents leak window
- Ping protocol with server health stats (CPU/MEM/DISK) embedded in custom magic-byte packets

---

## Recommendations Summary

| Priority | Finding | Recommendation |
|----------|---------|----------------|
| **HIGH** | Inverted TLS verify logic | Fix parameter naming or inversion in `client/api.go` |
| **HIGH** | Credential key from public info | Use OS keychain or machine-specific secret |
| **HIGH** | No login rate limiting | Add per-IP rate limiter on auth endpoints |
| **MEDIUM** | NAT map data race | Synchronize `NATIngress` access across both paths |
| **MEDIUM** | `rateWindow` unbounded growth | Add cleanup/eviction to match `seenIPs` pattern |
| **MEDIUM** | Wildcard CORS | Restrict origins per route group |
| **MEDIUM** | Unsigned update checksums | Add GPG/minisign signature verification |
| **MEDIUM** | Session token in logs | Reduce to DEBUG or redact |
| **MEDIUM** | No registration disable flag | Add `DisableRegistration` config option |
| **LOW** | Blocklist download unbounded | Add `io.LimitReader` |
| **LOW** | DNS-over-HTTPS error unchecked | Check `Unpack()` return |
| **LOW** | Command args not pre-validated | Regex-validate server-provided IPs before exec |
| **LOW** | IPv4 gap reuse | Scan for gaps or document limitation |
