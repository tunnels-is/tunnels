# VPN Throughput Investigation: 5MB/s → 40MB/s

## Problem
VPN tunnel throughput was capped at ~5MB/s (40Mbps) while the raw network path supports 520Mbps TCP and 400Mbps UDP with near-zero loss. A 10x degradation.

## Root Cause: GRO (Generic Receive Offload) on the Server NIC

The server uses `SOCK_RAW/IPPROTO_TCP` to read return traffic from the internet. With GRO enabled on the NIC, the kernel coalesces multiple TCP segments into single large buffers (~3KB) before delivering them to the raw socket. The server then wraps each coalesced buffer in a single UDP datagram and sends it to the client.

These oversized UDP datagrams (3KB + 28 bytes UDP/IP overhead = ~3028 bytes) exceed the 1500-byte path MTU, forcing the kernel to IP-fragment them into 2-3 pieces. IP fragmentation of UDP is devastating for throughput because:

1. If **any** fragment is lost, the **entire** datagram is lost (fragment loss amplification)
2. Reassembly consumes kernel memory and CPU, with timeout-based cleanup
3. Fragments may be treated differently by intermediate routers and firewalls

This caused the inner TCP (wget) to see packet loss, throttle its congestion window, and plateau at ~5MB/s.

## Fix

```bash
sudo ethtool -K eth0 gro off
```

One command. With GRO disabled, the raw socket receives individual TCP segments (≤1420 bytes each). Wrapped in UDP: ≤1448 bytes — well under the 1500 MTU. No fragmentation, no amplified loss, full throughput.

## How We Found It

### Phase 1: Code Optimizations (no improvement)
We initially suspected code-level bottlenecks and made several optimizations:
- **UDP socket buffers**: Set 8MB `SO_RCVBUF`/`SO_SNDBUF` on all sockets (server and client)
- **Allocation reduction**: Replaced per-packet `make([]byte)` nonce buffers with stack-allocated `[24]byte` arrays in `crypt/main.go`
- **Buffer pooling**: Added `sync.Pool` for packet buffers in `server/helpers.go`, replacing per-packet heap allocations in `CopySlice`
- **Channel sizing**: Reduced server channel buffers from 500,000 to 10,000
- **Client sysctls**: Added automatic tuning of `rmem_max`, `wmem_max`, BBR congestion control, `fq` qdisc in `client/helpers_unix.go`

These are all valid improvements that reduce GC pressure and improve buffer utilization, but they didn't move the throughput needle because the bottleneck was elsewhere.

### Phase 2: Minimal Tunnel Test (same 5MB/s)
Built a stripped-down tunnel in `perftest/` — no encryption, no auth, no channels, just TUN → UDP → raw socket. Got the same ~5MB/s, proving the bottleneck was **not** in the application code complexity.

### Phase 3: Network Baseline (400Mbps UDP)
Ran iperf3 tests:
- TCP: 520 Mbps — network path is excellent
- UDP at 400Mbps: 0.024% loss — UDP path has no throttling

This proved the network was not the issue.

### Phase 4: Instrumentation (found oversized packets)
Added per-second stats to the perftest. The data revealed:
- Average packet size from server → client: **~2.9KB** (should be ≤1420 bytes)
- This confirmed GRO was coalescing raw socket packets

### Phase 5: Disable GRO (40MB/s)
Installed ethtool, disabled GRO → throughput jumped to 40MB/s.

## Files Modified

### Code Changes (valid optimizations, keep them)
- `crypt/main.go` — Stack-allocated nonces in Seal1/Seal2/Open1/Open2
- `client/session.go` — 8MB UDP socket buffers after DialUDP
- `client/helpers_unix.go` — Automatic sysctl tuning on startup (rmem_max, wmem_max, BBR, fq)
- `server/socket.go` — 8MB socket buffers on all raw sockets, reduced channel sizes to 10,000
- `server/helpers.go` — sync.Pool for packet buffers, ReturnPacketBuf function

### Server Configuration
- `ethtool -K eth0 gro off` — **The critical fix.** Should be persisted via systemd service or udev rule.

### Test Artifacts
- `perftest/` — Minimal tunnel client/server used for isolation testing (can be removed)

## Persisting the GRO Fix

Add to the server's startup (e.g., systemd service or `/etc/networkd-dispatcher/routable.d/`):

```bash
ethtool -K eth0 gro off
```

Or via udev rule in `/etc/udev/rules.d/50-disable-gro.rules`:

```
ACTION=="add", SUBSYSTEM=="net", NAME=="eth0", RUN+="/usr/bin/ethtool -K $name gro off"
```

## Lessons Learned

1. **GRO + raw sockets = silent packet inflation.** Raw sockets receive GRO-coalesced buffers that can be multiples of MTU. Any application forwarding raw socket data over a size-constrained transport (like UDP) must account for this.

2. **Isolation testing is essential.** Building the minimal `perftest/` tunnel eliminated dozens of variables in one step and proved the issue was at the network/OS level, not in application code.

3. **iperf3 baselines are invaluable.** Testing raw TCP and UDP throughput immediately ruled out ISP throttling and network path issues.

4. **Instrumentation reveals the truth.** Adding packet size stats to the perftest immediately exposed the ~3KB average packet size that pointed directly to GRO coalescing.
