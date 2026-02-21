# SYSTEM.md -- Tunnels VPN Technical Deep Dive

This document explains how the Tunnels VPN works at every layer: architecture, connection lifecycle, packet processing, encryption, DNS, NAT, DHCP, firewall, and platform-specific implementation details.

---

## Table of Contents

1. [System Architecture](#1-system-architecture)
2. [Connection Lifecycle](#2-connection-lifecycle)
3. [Post-Quantum Hybrid Key Exchange](#3-post-quantum-hybrid-key-exchange)
4. [Wire Protocol](#4-wire-protocol)
5. [Packet Processing Pipeline](#5-packet-processing-pipeline)
6. [Server Raw Socket Engine](#6-server-raw-socket-engine)
7. [Port Allocation and NAT](#7-port-allocation-and-nat)
8. [Client-Side NAT Translation](#8-client-side-nat-translation)
9. [LAN Overlay Network (VPL)](#9-lan-overlay-network-vpl)
10. [DHCP System](#10-dhcp-system)
11. [Firewall and Connection Tracking](#11-firewall-and-connection-tracking)
12. [DNS Interception and Resolution](#12-dns-interception-and-resolution)
13. [Keepalive and Ping System](#13-keepalive-and-ping-system)
14. [Encryption at Rest](#14-encryption-at-rest)
15. [Platform-Specific Tunnel Interfaces](#15-platform-specific-tunnel-interfaces)
16. [Goroutine Supervision and Concurrency](#16-goroutine-supervision-and-concurrency)

---

## 1. System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            TUNNELS VPN SYSTEM                              │
│                                                                             │
│  ┌─────────────┐      ┌──────────────────┐      ┌────────────────────────┐ │
│  │   CLIENT     │      │   CONTROLLER     │      │       SERVER           │ │
│  │             │      │   (Auth Server)  │      │                        │ │
│  │ ┌─────────┐ │      │                  │      │ ┌────────────────────┐ │ │
│  │ │React UI │ │      │ /v3/session      │      │ │  Raw TCP Socket    │ │ │
│  │ │(embedded)│ │      │ (sign connect    │      │ │  (IPPROTO_TCP)     │ │ │
│  │ └────┬────┘ │      │  requests)       │      │ ├────────────────────┤ │ │
│  │      │      │      │                  │      │ │  Raw UDP Socket    │ │ │
│  │ ┌────┴────┐ │      │ /v3/user/*       │      │ │  (IPPROTO_UDP)     │ │ │
│  │ │Local API│ │      │ /v3/device/*     │      │ ├────────────────────┤ │ │
│  │ │:7777    │ │      │ /v3/group/*      │      │ │  UDP Data Socket   │ │ │
│  │ └────┬────┘ │      │ /v3/server/*     │      │ │  (VPNPort)         │ │ │
│  │      │      │      └──────────────────┘      │ ├────────────────────┤ │ │
│  │ ┌────┴────┐ │                                │ │  HTTPS API         │ │ │
│  │ │ Session │ │   TLS 1.3 + X25519MLKEM768     │ │  (APIPort)         │ │ │
│  │ │ Manager │◄├────────────────────────────────►│ ├────────────────────┤ │ │
│  │ └────┬────┘ │                                │ │  MongoDB / BoltDB  │ │ │
│  │      │      │                                │ └────────────────────┘ │ │
│  │ ┌────┴────┐ │      Encrypted UDP Tunnel      │                        │ │
│  │ │TUN/utun/│◄├════════════════════════════════►│  65536-slot client    │ │
│  │ │ Wintun  │ │    (AEAD encrypted packets)    │  mapping array         │ │
│  │ └─────────┘ │                                │                        │ │
│  └─────────────┘                                └────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Component Roles

**Client** (`client/`): Cross-platform VPN client that creates a virtual network interface (TUN/utun/Wintun), intercepts all IP traffic, encrypts it, and sends it to the server over a UDP tunnel. Runs a local HTTPS API on port 7777 serving an embedded React UI.

**Controller** (same binary as server, AUTH feature): Authentication and authorization server. Signs connection requests so VPN servers can verify client identity without direct database access. Manages users, devices, groups, and subscriptions.

**Server** (`server/`): High-performance VPN server using raw sockets at the IP level. Accepts encrypted tunnels from clients, decrypts packets, forwards them to the internet via raw sockets, and routes responses back. Supports a LAN overlay network for peer-to-peer communication.

### Server Goroutine Architecture

```
main()
 ├── Signal("DATA")    ──► DataSocketListener()       UDP :VPNPort ── client tunnels
 ├── Signal("TCP")     ──► ExternalTCPListener()       Raw SOCK_RAW/TCP ── internet responses
 ├── Signal("UDP")     ──► ExternalUDPListener()       Raw SOCK_RAW/UDP ── internet responses
 ├── Signal("PING")    ──► pingActiveUsers()           10s keepalive loop
 ├── Signal("API")     ──► launchAPIServer()           HTTPS REST API
 ├── Signal("CONFIG")  ──► config reload               30s config reload
 ├── Signal("SUBSCANNER") ──► scanSubs()               12h subscription check
 │
 └── Per connected client:
     ├── Signal("TO:<index>")   ──► toUserChannel()    Encrypt + send to client
     └── Signal("FROM:<index>") ──► fromUserChannel()  Decrypt + route from client
```

### Client Event Loop Architecture

```
LaunchTunnels()
 ├── Background goroutines (auto-restarting via concurrencyMonitor):
 │   ├── LogProcessor           Log queue consumer
 │   ├── APIServer              Local HTTPS API (:7777)
 │   ├── UDPDNSHandler          Local DNS server (:53)
 │   ├── BlockListUpdater       Hourly blocklist refresh
 │   ├── WhiteListUpdater       Hourly whitelist refresh
 │   ├── CleanDNSCache          DNS cache TTL cleanup
 │   ├── LogMapCleaner          Error dedup hash reset (10s)
 │   ├── CleanPortAllocs        Stale port mapping cleanup (10s)
 │   ├── DefaultGateway         Gateway/interface monitoring (2-5s)
 │   ├── AutoConnect            Auto-connect tunnels (30s)
 │   ├── Pinger                 Keepalive pings (10s)
 │   └── Updater                Auto-update check
 │
 └── Main select loop:
     ├── highPriorityChannel    ──► User-initiated actions
     ├── mediumPriorityChannel  ──► System events
     ├── lowPriorityChannel     ──► Background tasks
     ├── interfaceMonitor       ──► Restart TUN reader goroutine
     ├── tunnelMonitor          ──► Restart server reader goroutine
     └── concurrencyMonitor     ──► Restart crashed background goroutines
```

---

## 2. Connection Lifecycle

### Full Handshake Sequence

```
  CLIENT                      CONTROLLER                     VPN SERVER
    │                              │                              │
    │  ① POST /v3/session          │                              │
    │  ControllerConnectRequest    │                              │
    │  {UserID, ServerID,          │                              │
    │   DeviceKey/Token,           │                              │
    │   EncType, Version}          │                              │
    │─────────────────────────────►│                              │
    │                              │                              │
    │                              │  Verify credentials          │
    │                              │  Check group access           │
    │                              │  Sign payload with            │
    │                              │  server private key           │
    │                              │                              │
    │  ② SignedConnectRequest      │                              │
    │  {Payload, Signature}        │                              │
    │◄─────────────────────────────│                              │
    │                              │                              │
    │  ③ Generate X25519 keypair                                  │
    │     Generate ML-KEM-1024                                    │
    │     encap/decap keys                                        │
    │                                                             │
    │  ④ POST /v3/connect (TLS 1.3 + X25519MLKEM768)             │
    │  SignedConnectRequest +                                     │
    │  {X25519PeerPub,                                            │
    │   Mlkem1024Encap}                                           │
    │────────────────────────────────────────────────────────────►│
    │                                                             │
    │                                        ⑤ Verify signature   │
    │                                           Check freshness   │
    │                                           (<240 seconds)    │
    │                                           Count connections │
    │                                                             │
    │                                        ⑥ X25519 ECDH       │
    │                                           ML-KEM-1024       │
    │                                           encapsulate       │
    │                                           HKDF-SHA512       │
    │                                           Create AEADs      │
    │                                                             │
    │                                        ⑦ Allocate client    │
    │                                           slot (0-65535)    │
    │                                           Assign DHCP       │
    │                                           Assign port range │
    │                                           Spawn per-client  │
    │                                           goroutines        │
    │                                                             │
    │  ⑧ ServerConnectResponse                                    │
    │  {X25519Pub, Mlkem1024Cipher,                               │
    │   Signature, Index, InterfaceIP,                            │
    │   StartPort, EndPort, DHCP,                                 │
    │   Networks, Routes, DNS}                                    │
    │◄────────────────────────────────────────────────────────────│
    │                                                             │
    │  ⑨ X25519 ECDH (same shared secret)                        │
    │     ML-KEM-1024 decapsulate                                 │
    │     HKDF-SHA512 (same keys)                                 │
    │     Create same AEADs                                       │
    │     Verify server signature                                 │
    │                                                             │
    │  ⑩ Create TUN interface                                     │
    │     Configure IP/routes                                     │
    │     Open UDP socket to server                               │
    │     Start ReadFromTunnelInterface goroutine                  │
    │     Start ReadFromServeTunnel goroutine                      │
    │                                                             │
    │  ⑪ POST /v3/firewall (if LAN)                               │
    │  {DHCPToken, IP, Hosts}                                     │
    │────────────────────────────────────────────────────────────►│
    │                                                             │
    │══════════ Encrypted UDP Tunnel Active ═══════════════════════│
```

### Client-Side Connection Code Flow

```
PublicConnect(ConnectionRequest)
    │
    ├── CompareAndSwap(IsConnecting, false, true)   ── prevent concurrent connects
    ├── Look up TunnelMETA by tag
    ├── PreConnectCheck (admin permissions)
    │
    ├── getServerByID() ── fetch server IP/port/pubkey from controller
    ├── Add /32 route for controller IP via default gateway
    │
    ├── POST /v3/session to controller
    │   └── Returns SignedConnectRequest
    │
    ├── crypt.NewEncryptionHandler(encType)
    │   ├── InitializeClient()
    │   │   ├── X25519 keypair generation
    │   │   └── ML-KEM-1024 key generation
    │   └── Attach keys to SignedConnectRequest
    │
    ├── TLS 1.3 POST /v3/connect to VPN server
    │   ├── Custom TLS config: X25519MLKEM768 curve
    │   ├── Server cert loaded into custom CA pool
    │   └── Returns ServerConnectResponse
    │
    ├── Verify server signature
    ├── FinalizeClient() ── derive shared secret, create AEADs
    ├── CleanPostSecretGeneration() ── zero key exchange material
    │
    ├── InitializeTunnelFromCRR()
    │   ├── Parse local/server IPs into [4]byte (zero-alloc)
    │   ├── InitNatMaps() ── prepare NAT translation tables
    │   ├── InitVPLMap() ── prepare LAN address tables
    │   └── Initialize port mapping tables
    │
    ├── Add /32 route for server IP via default gateway
    ├── Dial UDP to server:DataPort
    ├── CreateAndConnectToInterface()
    │   ├── Create TUN/utun/Wintun device
    │   └── Configure IP, MTU, routes, default route
    │
    ├── Send initial encrypted ping
    ├── Launch ReadFromServeTunnel goroutine
    ├── Launch ReadFromTunnelInterface goroutine
    │
    └── POST /v3/firewall (if LAN enabled)
```

---

## 3. Post-Quantum Hybrid Key Exchange

The system uses a hybrid key exchange combining classical X25519 with post-quantum ML-KEM-1024, ensuring security even if quantum computers break elliptic curve cryptography.

```
  CLIENT                                          SERVER
    │                                               │
    │  Generate:                                    │
    │    X25519_priv, X25519_pub                    │
    │    MLKEM_decap_key, MLKEM_encap_key           │
    │                                               │
    │  ─── X25519_pub + MLKEM_encap_key ──────────► │
    │                                               │
    │                                  Generate:    │
    │                                    X25519_priv, X25519_pub
    │                                               │
    │                                  X25519 ECDH: │
    │                                    nk = X25519_priv.ECDH(client_X25519_pub)
    │                                    s1 = SHA-256(nk)
    │                                               │
    │                                  ML-KEM-1024: │
    │                                    s2, cipher = MLKEM_encap_key.Encapsulate()
    │                                               │
    │                                  Combined:    │
    │                                    fss = s1 ║ s2 ║ client_pub ║ server_pub
    │                                               │
    │                                  HKDF:        │
    │                                    key1, key2 = HKDF-SHA512(fss)
    │                                               │
    │                                  Create:      │
    │                                    AEAD1(key1) ── client→server direction
    │                                    AEAD2(key2) ── server→client direction
    │                                               │
    │  ◄── X25519_pub + MLKEM_cipher + signature ── │
    │                                               │
    │  Verify server signature                      │
    │                                               │
    │  X25519 ECDH:                                 │
    │    nk = X25519_priv.ECDH(server_X25519_pub)   │
    │    s1 = SHA-256(nk)     ◄── same value        │
    │                                               │
    │  ML-KEM-1024:                                 │
    │    s2 = MLKEM_decap_key.Decapsulate(cipher)   │
    │    s2 is same value     ◄── same value        │
    │                                               │
    │  Combined:                                    │
    │    fss = s1 ║ s2 ║ client_pub ║ server_pub    │
    │    ◄── identical fss on both sides            │
    │                                               │
    │  HKDF-SHA512 → key1, key2                     │
    │  AEAD1(key1), AEAD2(key2)                     │
    │    ◄── identical ciphers on both sides        │
    │                                               │
    │  Zero all ephemeral key material              │
```

### Supported Cipher Suites

| EncType | Algorithm | Key Size | Nonce Size | Tag Size |
|---------|-----------|----------|------------|----------|
| 1 | AES-128-GCM | 16 bytes | 12 bytes | 16 bytes |
| 2 | AES-256-GCM | 32 bytes | 12 bytes | 16 bytes |
| 3 | XChaCha20-Poly1305 | 32 bytes | 24 bytes | 16 bytes |

Two independent AEAD instances are created per connection -- one per direction. This provides directional key separation.

### Security Properties

- **Post-quantum resistance**: ML-KEM-1024 protects against quantum attacks on X25519
- **Forward secrecy**: Ephemeral keys generated per connection
- **Replay protection**: Atomically incrementing nonce counters
- **Channel binding**: Connection index used as AEAD additional authenticated data (AAD)
- **Key hygiene**: All ephemeral material zeroed after AEAD creation

---

## 4. Wire Protocol

### Tunnel Packet Format

```
 0                   1                   2         ...        9  10       ...      N
 ┌───────────────────┬───────────────────┬────────────────────┬────────────────────┐
 │  Index (uint16)   │              Nonce (8 bytes)           │  AEAD Ciphertext   │
 │  [0]      [1]     │ [2] [3] [4] [5] [6] [7] [8] [9]      │  (IP packet + tag) │
 └───────────────────┴───────────────────────────────────────┴────────────────────┘
       2 bytes                    8 bytes                       variable length
                           (uint64 big-endian                (encrypted IPv4 packet
                            counter)                          + 16-byte auth tag)

 AAD (Additional Authenticated Data) = bytes [0:2] (the Index)
```

- **Index**: 2-byte big-endian client slot number (0-65535). Used for O(1) lookup on the server and as AAD for authenticated encryption.
- **Nonce**: 8-byte big-endian uint64 counter, atomically incremented per packet. Padded to full nonce length (12 or 24 bytes) before use.
- **Ciphertext**: The AEAD-sealed IPv4 packet with appended authentication tag.

### Ping Packet Format

Ping packets are transmitted inside the encrypted tunnel (same wire format above). After decryption, pings are identified by being smaller than 20 bytes:

```
 0       1       2       3       4       5       6       7       8       9  10  11
 ┌───────┬───────┬───────┬───────┬───────┬───────┬───────┬───────┬───────┬──────┐
 │ CPU%  │ RAM%  │ Disk% │ 0xFF  │ 0x01  │ 0xFF  │ 0x02  │ 0xFF  │ 0x03 │ 0xFF │
 └───────┴───────┴───────┴───────┴───────┴───────┴───────┴───────┴──────┴──────┘
                          ◄── magic bytes: {255,1,255,2,255,3,255,4} ──►

 11      12      13      14      15      16      17      18
 ┌───────┬───────────────────────────────────────────────────┐
 │ 0x04  │         Ping Counter (uint64 big-endian)          │
 └───────┴───────────────────────────────────────────────────┘
```

---

## 5. Packet Processing Pipeline

### Egress: Application to Internet via VPN

```
┌──────────────┐
│ Application  │    (e.g., curl google.com)
│ sends packet │
└──────┬───────┘
       │ raw IPv4 packet
       ▼
┌──────────────────────────────────────────────────────────┐
│              OS TUN/utun/Wintun Device                   │
│                                                          │
│  Linux:   read() from /dev/net/tun fd                    │
│  macOS:   read() from utun socket (strip 4-byte AF hdr) │
│  Windows: WintunReceivePacket() from ring buffer         │
└──────┬───────────────────────────────────────────────────┘
       │ raw IPv4 packet
       ▼
┌──────────────────────────────────────────────────────────┐
│             ProcessEgressPacket()                         │
│                                                          │
│  1. Version check ── IPv4 only (drop IPv6, etc.)         │
│  2. Protocol filter ── TCP (6) and UDP (17) only         │
│  3. Parse IPv4 header length (IHL * 4)                   │
│  4. Extract destination IP (bytes 16-19)                 │
│  5. Extract source + destination ports                   │
│                                                          │
│  ┌─ Is destination a VPL/LAN IP? ──────────────────────┐ │
│  │                                                      │ │
│  │  YES (LAN):                  NO (Internet):          │ │
│  │  TransLateVPLIP()            CreateNEWPortMapping()  │ │
│  │  src IP → serverVPLIP        Track TCP RST/FIN       │ │
│  │                              TransLateIP()           │ │
│  │                              src IP → serverIfaceIP  │ │
│  │                              src port → mapped port  │ │
│  └──────────────────────────────────────────────────────┘ │
│                                                          │
│  6. Apply NAT destination IP rewrite                     │
│  7. Recalculate IPv4 header checksum                     │
│  8. Recalculate TCP/UDP checksum (with pseudo-header)    │
└──────┬───────────────────────────────────────────────────┘
       │ NAT-rewritten IPv4 packet
       ▼
┌──────────────────────────────────────────────────────────┐
│              SEAL.Seal1(packet, index)                    │
│                                                          │
│  AEAD1 encrypt:                                          │
│    nonce = atomic_increment(counter1)                    │
│    output = [index:2][nonce:8][AEAD1.Seal(packet, AAD)]  │
└──────┬───────────────────────────────────────────────────┘
       │ encrypted tunnel packet
       ▼
┌──────────────────────────────────────────────────────────┐
│         UDP socket write to server:DataPort              │
└──────────────────────────────────────────────────────────┘
```

### Ingress: Internet to Application via VPN

```
┌──────────────────────────────────────────────────────────┐
│        UDP socket read from server:DataPort              │
└──────┬───────────────────────────────────────────────────┘
       │ encrypted tunnel packet
       ▼
┌──────────────────────────────────────────────────────────┐
│            SEAL.Open2(ciphertext, nonce, AAD)             │
│                                                          │
│  Extract: nonce = bytes[2:10]                            │
│           AAD   = bytes[0:2] (index)                     │
│           data  = bytes[10:n]                            │
│  AEAD2 decrypt: plaintext = AEAD2.Open(data, nonce, AAD) │
│                                                          │
│  If decryption fails → terminate (security failure)      │
│  If plaintext < 20 bytes → RegisterPing() and return     │
└──────┬───────────────────────────────────────────────────┘
       │ decrypted IPv4 packet
       ▼
┌──────────────────────────────────────────────────────────┐
│             ProcessIngressPacket()                        │
│                                                          │
│  1. Extract source IP (bytes 12-15)                      │
│  2. Parse protocol and header lengths                    │
│  3. Extract source + destination ports                   │
│                                                          │
│  ┌─ Is source a VPL/LAN IP? ──────────────────────────┐  │
│  │                                                     │  │
│  │  YES (LAN):                  NO (Internet):         │  │
│  │  dst IP → localInterfaceIP   NATIngress lookup      │  │
│  │                              (reverse src IP)       │  │
│  │                              getIngressPortMapping() │  │
│  │                              Track TCP RST/FIN      │  │
│  │                              Restore orig dst port  │  │
│  │                              Restore orig dst IP    │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                          │
│  4. Recalculate IPv4 header checksum                     │
│  5. Recalculate TCP/UDP checksum                         │
└──────┬───────────────────────────────────────────────────┘
       │ restored IPv4 packet
       ▼
┌──────────────────────────────────────────────────────────┐
│              OS TUN/utun/Wintun Device                   │
│                                                          │
│  Linux:   write() to /dev/net/tun fd                     │
│  macOS:   write() to utun socket (prepend 4-byte AF hdr) │
│  Windows: WintunAllocateSendPacket() + SendPacket()      │
└──────┬───────────────────────────────────────────────────┘
       │
       ▼
┌──────────────┐
│ Application  │    (receives response)
└──────────────┘
```

---

## 6. Server Raw Socket Engine

The server uses raw sockets to operate at the IP packet level, bypassing the kernel's TCP/UDP stack entirely.

### Socket Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         VPN SERVER                                      │
│                                                                         │
│  ┌───────────────────┐     ┌────────────────────────────────────────┐   │
│  │  DataSocketListener│     │         clientCoreMappings[65536]      │   │
│  │  (UDP :VPNPort)    │     │                                        │   │
│  │                    │     │  [0] = nil                              │   │
│  │  Recvfrom() loop:  │     │  [1] = nil                              │   │
│  │  id = buf[0:2]     │────►│  [2] = *UserCoreMapping {              │   │
│  │  Push to           │     │          ToUser chan (cap 500,000)      │   │
│  │  [id].FromUser     │     │          FromUser chan (cap 500,000)    │   │
│  └───────────────────┘     │          EH *SocketWrapper              │   │
│                             │          PortRange {2000-2635}          │   │
│                             │          DHCP {10.0.0.5}               │   │
│  ┌───────────────────┐     │          Addr (client UDP sockaddr)     │   │
│  │ fromUserChannel(2)│◄────│        }                                │   │
│  │ (per-client grtn) │     │  [3] = *UserCoreMapping { ... }         │   │
│  │                    │     │  ...                                    │   │
│  │  Decrypt packet    │     │  [65535] = nil                          │   │
│  │  Route decision:   │     └────────────────────────────────────────┘   │
│  │  ├─ LAN → peer     │                                                 │
│  │  │   ToUser chan    │     ┌────────────────────────────────────────┐   │
│  │  └─ Internet →      │     │         portToCoreMapping[65536]       │   │
│  │     raw socket      │     │                                        │   │
│  └──────────┬──────────┘     │  [0..1999] = nil                       │   │
│             │                │  [2000]    = &PortRange{2000,2635,CM2} │   │
│             │ TCPRWC.Write   │  [2001]    = &PortRange{2000,2635,CM2} │   │
│             │ or             │  ...                                    │   │
│             │ UDPRWC.Write   │  [2635]    = &PortRange{2000,2635,CM2} │   │
│             ▼                │  [2636]    = &PortRange{2636,3271,nil} │   │
│  ┌──────────────────┐       │  ...                                    │   │
│  │  Raw Write Socket │       └───────────────────────┬────────────────┘   │
│  │  (IPPROTO_RAW)    │                               │                    │
│  │  SYS_SENDTO with  │                               │                    │
│  │  crafted IP header│       ┌────────────────────────┘                    │
│  └──────────┬────────┘       │                                            │
│             │                │  ExternalTCPListener / ExternalUDPListener  │
│             ▼                │  (raw SOCK_RAW sockets)                    │
│        ┌─────────┐          │                                            │
│        │INTERNET │          │  Recvfrom() loop:                          │
│        └────┬────┘          │  dst_port = parse from IP+transport header │
│             │               │  PM = portToCoreMapping[dst_port]          │
│             │ response      │  PM.Client.ToUser ← packet                 │
│             ▼               │                                            │
│  ┌──────────────────┐       │  ┌───────────────────┐                     │
│  │  Raw Read Socket  │◄──────┘  │ toUserChannel(2)  │                     │
│  │  (SOCK_RAW)       │          │ (per-client grtn)  │                     │
│  │  Bound to VPN IF  │          │                    │                     │
│  └──────────────────┘          │ Read from ToUser   │                     │
│                                │ LAN firewall check │                     │
│                                │ SEAL.Seal2()       │                     │
│                                │ Sendto(client)     │                     │
│                                └────────────────────┘                     │
└───────────────────────────────────────────────────────────────────────────┘
```

### Why Raw Sockets?

The server needs to:
1. Forward arbitrary client IP packets to the internet with the server's source IP
2. Capture all response packets destined for ports in the VPN range
3. Rewrite IP headers at the packet level

Standard Go `net.Conn` cannot do this. Raw sockets with `IPPROTO_RAW` allow sending IP packets with custom headers, and `SOCK_RAW` with specific protocol numbers allow capturing all TCP or UDP packets.

### The RST Problem

When the kernel receives TCP packets for connections it doesn't know about (because the VPN server handles them in userspace), it sends RST packets that kill client connections. The fix:

```bash
iptables -I OUTPUT -p tcp --src {VPN_IP} --tcp-flags ACK,RST RST -j DROP
```

This drops all outgoing RST packets from the VPN interface, applied automatically on startup via `iptables.SetIPTablesRSTDropFilter()`.

---

## 7. Port Allocation and NAT

### Server-Side Port Allocation

The server divides its port range into equal slots based on bandwidth ratios:

```
Configuration:
  ServerBandwidthMbps = 1000
  UserBandwidthMbps   = 10
  StartPort           = 2000
  EndPort             = 65530

Calculation:
  slots = 1000 / 10 = 100 users
  portsPerUser = (65530 - 2000) / 100 = 635 ports each

Allocation:
  Slot  0: ports  2000 -  2635  →  PortRange{Start:2000,  End:2635}
  Slot  1: ports  2636 -  3271  →  PortRange{Start:2636,  End:3271}
  Slot  2: ports  3272 -  3907  →  PortRange{Start:3272,  End:3907}
  ...
  Slot 99: ports 63895 - 64530  →  PortRange{Start:63895, End:64530}
```

Every port in `portToCoreMapping[port]` points to its owning `PortRange`. When a response packet arrives, the server looks up the destination port to find which client owns it.

### How Internet Traffic Flows

The client performs all NAT rewriting before sending. The server simply forwards raw packets:

```
Client app sends to 93.184.216.34:443 (example.com)

CLIENT side:
  1. App packet: src=172.22.22.1:54321, dst=93.184.216.34:443
  2. CreateNEWPortMapping() allocates port 2015 (within assigned range 2000-2635)
  3. TransLateIP() may rewrite dst IP based on NAT network config
  4. Rewrite: src=10.0.0.5:2015, dst=93.184.216.34:443
     (10.0.0.5 = server interface IP, 2015 = mapped port)
  5. Encrypt and send to server

SERVER side:
  6. Decrypt packet
  7. Forward to internet via TCPRWC.Write()
     (raw socket sends packet as-is, src IP = server's VPN interface IP)

RESPONSE:
  8. Internet responds: src=93.184.216.34:443, dst=VPNIP:2015
  9. ExternalTCPListener captures packet (raw socket)
  10. portToCoreMapping[2015] → ClientCoreMapping for this client
  11. Push to client's ToUser channel
  12. Encrypt and send back to client

CLIENT side:
  13. Decrypt packet
  14. getIngressPortMapping(2015) → original mapping
  15. Restore: src=93.184.216.34:443, dst=172.22.22.1:54321
  16. Write to TUN interface → app receives response
```

---

## 8. Client-Side NAT Translation

### NAT Map Initialization

When a tunnel connects, the server response includes `Networks` -- a list of network/NAT CIDR pairs. The client uses these to translate destination IPs:

```
Server Network Config:
  Network: 10.10.0.0/16    ←  "real" server subnet
  NAT:     10.20.0.0/16    ←  client-visible NAT range

Client sends packet to 10.20.5.100 (NAT range):
  TransLateIP():
    1. Check NATEgress cache → miss
    2. Iterate Networks, find NAT 10.20.0.0/16 contains 10.20.5.100
    3. Apply mask formula:
       newIP[i] = networkIP[i] & mask[i] | originalIP[i] & ~mask[i]
       10.10.5.100 (preserves host part, remaps network prefix)
    4. Cache bidirectionally:
       NATEgress[10.20.5.100]  = 10.10.5.100
       NATIngress[10.10.5.100] = 10.20.5.100
    5. Return 10.10.5.100

Response arrives from 10.10.5.100:
  ProcessIngressPacket():
    NATIngress[10.10.5.100] = 10.20.5.100  →  rewrite src to 10.20.5.100
```

### Port Mapping Table Structure

```
Port Mapping (per tunnel):

AvailableTCPPorts[portIndex] = xsync.MapOf:
  key:   [6]byte = {dstIP[0], dstIP[1], dstIP[2], dstIP[3], dstPort[0], dstPort[1]}
  value: *Mapping

ActiveTCPMapping = xsync.MapOf:
  key:   [12]byte = {srcIP[0..3], dstIP[0..3], srcPort[0..1], dstPort[0..1]}
  value: *Mapping

Mapping:
  ┌──────────────────────────────────────────────┐
  │  Proto (TCP/UDP)                             │
  │  SrcPort, DstPort (original)                 │
  │  MappedPort (assigned VPN port)              │
  │  OriginalSourceIP (app's real src IP)        │
  │  DestinationIP (original dst IP)             │
  │  UnixTime (last activity, for cleanup)       │
  │  rstFound (TCP RST detected)                 │
  │  finCount (TCP FIN counter for cleanup)      │
  └──────────────────────────────────────────────┘

Cleanup Timeouts:
  TCP with RST or FIN>1:  10 seconds
  TCP normal:            360 seconds
  UDP DNS:                15 seconds
  UDP other:             150 seconds
```

---

## 9. LAN Overlay Network (VPL)

The VPL (Virtual Private LAN) enables peer-to-peer communication between VPN clients through a 10.0.0.0/16 overlay network.

### LAN Routing Architecture

```
┌──────────────┐                                    ┌──────────────┐
│  Client A    │                                    │  Client B    │
│  10.0.1.5    │                                    │  10.0.2.10   │
│              │          VPN SERVER                 │              │
│  Sends to    │    ┌───────────────────┐           │              │
│  10.0.2.10   │    │                   │           │              │
│  ───────────►├───►│ fromUserChannel() │           │              │
│              │    │                   │           │              │
│              │    │ Decrypt packet    │           │              │
│              │    │ dst = 10.0.2.10   │           │              │
│              │    │                   │           │              │
│              │    │ VPLIPToCore       │           │              │
│              │    │ [10][0][2][10]    │           │              │
│              │    │ = Client B's CM   │           │              │
│              │    │                   │           │              │
│              │    │ Push to B's       │           │              │
│              │    │ ToUser channel    │──────────►├───►│ Receives  │
│              │    │                   │           │    │ packet    │
│              │    │ toUserChannel()   │           │              │
│              │    │ Firewall check    │           │              │
│              │    │ Encrypt + send    │           │              │
│              │    └───────────────────┘           │              │
└──────────────┘                                    └──────────────┘
```

### VPL IP Lookup

The server uses a 4-dimensional array for O(1) IP-to-client lookup:

```
VPLIPToCore[octet1][octet2][octet3][octet4] = *UserCoreMapping

For 10.0.0.0/16:
  VPLIPToCore[10][0][0..255][0..255] = 65536 possible LAN addresses

Lookup for 10.0.2.10:
  VPLIPToCore[10][0][2][10] → Client B's UserCoreMapping → ToUser channel
```

### Client-Side VPL NAT

The client translates between its local interface IP and the VPL address space:

```
Egress (app sends to LAN peer):
  src: 172.22.22.1 (local TUN IP)  →  10.0.1.5 (VPL IP / serverVPLIP)
  dst: stays as-is (10.0.2.10)

Ingress (LAN peer responds):
  src: 10.0.2.10 (stays as-is)
  dst: 10.0.1.5 (VPL IP)  →  172.22.22.1 (local TUN IP)
```

---

## 10. DHCP System

The server pre-allocates a DHCP record for every IP in the LAN network at startup:

```
Network: 10.0.0.0/16  →  65536 DHCPRecord entries

DHCPMapping[0]     = {IP: 10.0.0.0,   Token: "", Activity: zero}  ← skipped (.0)
DHCPMapping[1]     = {IP: 10.0.0.1,   Token: "", Activity: zero}  ← skipped (.1)
DHCPMapping[2]     = {IP: 10.0.0.2,   Token: "", Activity: zero}  ← available
DHCPMapping[3]     = {IP: 10.0.0.3,   Token: "", Activity: zero}  ← available
...
DHCPMapping[65535] = {IP: 10.0.255.255, Token: "", Activity: zero}
```

### Assignment Flow

```
New client connects with DeviceToken="abc123":

Phase 1 ── Reclaim existing lease:
  Scan all 65536 entries looking for Token == "abc123"
  If found → reuse same IP (reconnecting client keeps its address)

Phase 2 ── Assign new lease (if no reclaim):
  Scan entries, skip .0 and .1 addresses
  Find first entry where:
    Token is empty, OR
    Activity is older than DHCPTimeoutHours
  Assign:
    Token = "abc123"
    Activity = now
    Hostname = "10-0-0-5.tunnels.local"  (IP octets joined by dashes)

Register in VPLIPToCore:
  VPLIPToCore[10][0][0][5] = this client's UserCoreMapping
```

DHCP leases are **not** released on disconnect. They persist until `DHCPTimeoutHours` expires, allowing reconnecting clients to reclaim their IP.

---

## 11. Firewall and Connection Tracking

The LAN firewall controls which VPN peers can communicate with each other. It operates in the server's `toUserChannel()` (outbound to client) path.

### Firewall Architecture

```
Packet from Client A (10.0.1.5) to Client B (10.0.2.10)

fromUserChannel(A):                        toUserChannel(B):
  │                                          │
  │ SYN packet detected                      │ Packet from 10.0.1.5
  │ A.AddHost(10.0.2.10,                     │
  │   dstPort, "auto")                       │ Is A a NetAdmin? → bypass
  │                                          │
  │                                          │ Is firewall disabled
  │                                          │ for B? → bypass
  │                                          │
  │ Push to B.ToUser ──────────────────────► │ Check B.AllowedHosts:
  │                                          │   10.0.1.5:srcPort in list?
  │                                          │
  │                                          │   YES → forward to B
  │                                          │   NO  → DROP
```

### Allowed Hosts Types

| Type | Source | Port Matching | Behavior |
|------|--------|---------------|----------|
| `"auto"` | SYN tracking | IP + Port must match | Added on SYN, removed on RST or double-FIN |
| `"manual"` | User/API configured | IP only (any port) | Persistent until removed via API |

### TCP Connection Tracking

```
SYN (in fromUserChannel):
  sender.AddHost(dstIP, dstPort, "auto")     ── allow responses back

FIN (bidirectional tracking):
  fromUserChannel: sender.SetFin(dstIP, dstPort, fromUser=true)   ── FFIN
  toUserChannel:   target.SetFin(srcIP, srcPort, fromUser=false)  ── TFIN
  Both FFIN and TFIN set → connection closed, entry cleaned

RST:
  fromUserChannel: sender.DelHost(dstIP, "auto")    ── immediate cleanup
  toUserChannel:   target.DelHost(srcIP, "auto")     ── immediate cleanup
```

---

## 12. DNS Interception and Resolution

The client runs a local DNS server on `127.0.0.1:53` using the `miekg/dns` library. When the system is configured to use the VPN's DNS, all queries are intercepted.

### DNS Resolution Pipeline

```
Application DNS query
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    DNSQuery Handler                              │
│                                                                  │
│  1. Drop .arpa queries (reverse DNS)                             │
│                                                                  │
│  2. Cache check ────────────────────────► HIT → return cached    │
│                                                                  │
│  3. Whitelist check ──► whitelisted? ──► skip blocklist          │
│                                                                  │
│  4. Blocklist check ──► blocked? ──► set blocked flag            │
│                                                                  │
│  5. Tunnel DNS records ──► match? ──► return server-pushed record│
│     (iterate all connected tunnels,                              │
│      check ServerResponse.DNSRecords                             │
│      with wildcard support)                                      │
│                                                                  │
│  6. Local config DNS records ──► match? ──► return config record │
│                                                                  │
│  7. If blocked and no override ──► return empty (NXDOMAIN-like)  │
│                                                                  │
│  8. If record found with IPs ──► construct A/TXT response        │
│     If record found but no IPs ──► resolve via tunnel DNS        │
│                                                                  │
│  9. Drop *.lan queries                                           │
│                                                                  │
│  10. External resolution:                                        │
│      ├── DNS-over-HTTPS (if enabled)                             │
│      │   POST https://dns-server/dns-query                       │
│      │   Content-Type: application/dns-message                   │
│      │                                                           │
│      └── Standard UDP forwarding                                 │
│          Try DNS1Default:53, fallback DNS2Default:53             │
│                                                                  │
│  11. Cache result with TTL                                       │
└──────────────────────────────────────────────────────────────────┘
```

### DNS Global Block

During tunnel switching/reconnection, `DNSGlobalBlock` is set to `true`, causing all DNS queries to return empty responses. This prevents DNS leaks during the brief window between disconnect and reconnect.

### Blocklist System

- 10 default categories from `github.com/n00bady/bluam` (Ads, Malware, Gambling, etc.)
- Lists downloaded every 24 hours, cached locally for offline use
- Whitelist takes priority over blocklist
- Loaded into `xsync.MapOf[string, bool]` for concurrent access

---

## 13. Keepalive and Ping System

### Bidirectional Ping

```
Every 10 seconds:

SERVER → CLIENT (pingActiveUsers):
  ┌───────────────────────────────────────────────┐
  │  For each active client:                       │
  │  1. Populate PingPongStatsBuffer:              │
  │     [CPU%][RAM%][Disk%][magic][counter:8]      │
  │  2. Write client's own PingInt into counter    │
  │  3. SEAL.Seal2(buffer, Uindex)                 │
  │  4. Sendto(dataSocketFD, encrypted, client)    │
  │                                                │
  │  If send fails → NukeClient(index)             │
  │  If no ping in PingTimeoutMinutes → NukeClient │
  └───────────────────────────────────────────────┘

CLIENT → SERVER (PingConnections):
  ┌───────────────────────────────────────────────┐
  │  For each active tunnel:                       │
  │  1. Increment local ping counter               │
  │  2. Populate buffer with system stats          │
  │  3. SEAL.Seal1(buffer, index)                  │
  │  4. Write to tunnel UDP connection             │
  │                                                │
  │  If no server ping in 45s:                     │
  │    AutoReconnect? → PublicConnect()            │
  │    Otherwise → Disconnect() (if no kill switch)│
  └───────────────────────────────────────────────┘
```

### Counter Desync Detection

The client tracks a local ping counter and compares it against the counter echoed back by the server. If `localCounter > serverCounter + 10`, the connection is considered out of sync and triggers reconnection.

### NukeClient Cleanup

When a client times out or the send fails:

```
NukeClient(index):
  1. Deallocate port range (set PortRange.Client = nil)
  2. Close ToUser and FromUser channels (causes goroutines to exit)
  3. Stop ToSignal and FromSignal (ShouldStop = true)
  4. Set clientCoreMappings[index] = nil
  Note: DHCP lease is NOT released (persists for reconnection)
```

---

## 14. Encryption at Rest

### User Credentials (Client-Side)

```
saveUser():
  key = argon2id(workingDir + execPath)     ← machine-local key
  hash = argon2id(userID, skipSalt=true)    ← deterministic filename
  encrypted = AES-CTR(json(user), key)
  write(userPath + hex(hash), IV || ciphertext)

getUsers():
  key = argon2id(workingDir + execPath)
  for each file in users/:
    IV = file[0:16]
    ciphertext = file[16:]
    plaintext = AES-CTR-decrypt(ciphertext, IV, key)
    unmarshal(plaintext) → User
```

Credentials are tied to the specific binary location. Moving the binary changes the key.

### 2FA Secrets (Server-Side)

```
Encrypt(secret, TwoFactorKey):
  salt = random(16)
  key = PBKDF2(TwoFactorKey, salt, 600000, 32, SHA-256)
  nonce = random(12)
  ciphertext = AES-256-GCM(secret, key, nonce)
  store: [salt:16][nonce:12][ciphertext+tag:16]
```

### User Passwords

Passwords are hashed with bcrypt (cost 13), not encrypted. Verification uses bcrypt.CompareHashAndPassword.

---

## 15. Platform-Specific Tunnel Interfaces

### Linux (TUN via /dev/net/tun)

```
1. Open /dev/net/tun (O_RDWR | O_NONBLOCK)
2. ioctl(TUNSETIFF, {name, IFF_TUN | IFF_NO_PI})
   IFF_NO_PI = no packet info header → raw IP packets
3. ioctl(SIOCSIFADDR)    → set IPv4 address
4. ioctl(SIOCSIFFLAGS)   → bring UP (flag 0x1)
5. ioctl(SIOCSIFMTU)     → set MTU
6. ioctl(SIOCSIFTXQLEN)  → set transmit queue length
7. netlink.RouteAdd()     → add routes (default route, NAT ranges, etc.)

Read/Write: standard file I/O on the fd
```

### macOS (utun via kernel control socket)

```
1. socket(AF_SYSTEM, SOCK_DGRAM, SYSPROTO_CONTROL)
2. ioctl(CTLIOCGINFO, "com.apple.net.utun_control")
3. connect(sockaddrCtl{sc_id, sc_unit=0})  → kernel assigns utunN
4. getsockopt(UTUN_OPT_IFNAME)             → get assigned name
5. ifconfig <name> <ip> <gateway> up
6. ifconfig <name> mtu <value>
7. route add ...

Read:  4-byte AF header prepended → strip before processing
Write: prepend {0,0,0,2} (AF_INET) before packet
```

### Windows (Wintun DLL)

```
1. LoadLibrary("wintun.dll")
2. WintunOpenAdapter(name) or WintunCreateAdapter(name, "tunnels", GUID)
3. netsh interface ipv4 set address ... static <ip> <mask>
4. WintunStartSession(handle, 0x4000000)  → 64 MiB ring buffer
5. netsh interface ipv4 set subinterface ... mtu=<value>
6. netsh interface ipv4 add route ...

Read:  WintunReceivePacket() → packet pointer + size
       WintunReleaseReceivePacket(packet)  → return buffer
Write: WintunAllocateSendPacket(size) → buffer pointer
       Copy packet into buffer
       WintunSendPacket(buffer)
```

---

## 16. Goroutine Supervision and Concurrency

### Lock-Free Architecture

The client uses `atomic.Pointer` for all global state, avoiding mutexes on the hot path:

```
STATE  atomic.Pointer[stateV2]              ← runtime state
CONFIG atomic.Pointer[configV2]             ← configuration
TunnelMetaMap  *xsync.MapOf[string, *TunnelMETA]  ← tunnel configs
TunnelMap      *xsync.MapOf[string, *TUN]          ← active tunnels
DNSBlockList   atomic.Pointer[xsync.MapOf]         ← DNS blocklist
DNSWhiteList   atomic.Pointer[xsync.MapOf]         ← DNS whitelist
DNSCache       *xsync.MapOf[string, any]           ← DNS cache
```

The `TUN` struct pre-allocates fields for packet header parsing (`EP_*` for egress, `IP_*` for ingress) to avoid per-packet heap allocations in the hot path.

### Signal Pattern (server)

```go
signal.NewSignal("TAG", ctx, cancel, sleep, logFunc, method)
```

The `Signal.Start()` method runs `method()` in a loop with automatic panic recovery:

```
loop:
  defer recover()     ← catch panics
  method()            ← run the task
  if ShouldStop → exit
  if ctx.Err() → exit
  sleep(duration)     ← backoff before restart
  log("goroutine restart")
  goto loop
```

### ConcurrencyMonitor Pattern (client)

Background goroutines are wrapped in `goSignal` and self-enqueue for restart:

```
goSignal.execute():
  defer RecoverAndLog()
  method()                          ← run the task
  time.Sleep(1 * time.Second)       ← backoff
  concurrencyMonitor <- self        ← re-enqueue

Main loop:
  select {
  case signal := <-concurrencyMonitor:
    go signal.execute()             ← relaunch
  }
```

This makes every background task (DNS handler, pinger, auto-connect, port cleaner, etc.) automatically restart after completion or failure with a 1-second delay.

### Channel Capacities

```
Server per-client:
  ToUser     chan []byte  (cap 500,000)   ← packets to client
  FromUser   chan Packet  (cap 500,000)   ← packets from client

Client:
  concurrencyMonitor  chan *goSignal  (cap 1,000)
  tunnelMonitor       chan *TUN       (cap 1,000)
  interfaceMonitor    chan *TUN       (cap 1,000)
  highPriorityChannel chan *event     (cap 100)
  mediumPriorityChannel chan *event   (cap 100)
  lowPriorityChannel  chan *event     (cap 100)
  LogQueue            chan string     (cap 1,000)
  APILogQueue         chan string     (cap 1,000)
```

---

## End-to-End Example: HTTPS Request Through the VPN

```
1. Browser sends DNS query for example.com
   → Intercepted by client DNS server on 127.0.0.1:53
   → Not blocked, not cached
   → Forwarded to upstream DNS (1.1.1.1)
   → Response cached, returned to browser

2. Browser sends TCP SYN to 93.184.216.34:443
   → Captured by TUN interface
   → ReadFromTunnelInterface reads packet

3. ProcessEgressPacket:
   → IPv4 ✓, TCP ✓
   → CreateNEWPortMapping: allocates port 2500 (range 2000-2635)
   → TransLateIP: no NAT needed (public IP)
   → Rewrite: src 172.22.22.1:54321 → 10.0.0.5:2500
   → Recalculate checksums

4. SEAL.Seal1(packet, [0,2]):
   → nonce = 1 (first packet)
   → output = [0,2,0,0,0,0,0,0,0,1, <encrypted packet>]

5. UDP send to server:VPNPort

6. DataSocketListener receives:
   → id = BigEndian(0,2) = 2
   → Push to clientCoreMappings[2].FromUser

7. fromUserChannel(2):
   → SEAL.Open1 decrypts
   → dst = 93.184.216.34 (internet)
   → Protocol = 6 (TCP)
   → TCPRWC.Write(packet)
   → SYS_SENDTO via raw socket

8. 93.184.216.34:443 receives SYN, sends SYN-ACK
   → Arrives at server's VPN interface

9. ExternalTCPListener:
   → Parse dst port = 2500
   → portToCoreMapping[2500] = PortRange for client 2
   → Push to client 2's ToUser channel

10. toUserChannel(2):
    → Not LAN traffic, no firewall check
    → SEAL.Seal2(packet, [0,2])
    → Sendto client UDP address

11. Client ReadFromServeTunnel:
    → SEAL.Open2 decrypts
    → ProcessIngressPacket:
      → getIngressPortMapping(2500) → original mapping
      → Restore: dst 10.0.0.5:2500 → 172.22.22.1:54321
      → Recalculate checksums
    → Write to TUN interface

12. Browser receives SYN-ACK, TCP handshake completes
    → TLS handshake, HTTP request/response follow same path
```
