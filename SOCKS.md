# SOCKS5 Proxy Feature

The SOCKS5 proxy feature allows authenticated users to route traffic through the server using the SOCKS5 protocol.

## How It Works

1. User connects to a VPN tunnel
2. User clicks "Enable Proxy" in the tunnel menu
3. Server whitelists the user's IP for 24 hours
4. User configures applications to use the SOCKS5 proxy

## Server Configuration

Add `"SOCKS"` to the Features array in `server/config.json`:

```json
{
  "Features": ["AUTH", "SOCKS", "BBOLT"],
  "SOCKSIP": "127.0.0.1",
  "SOCKSPort": "1080"
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `SOCKSIP` | Interface IP | IP address for SOCKS5 server |
| `SOCKSPort` | `80` | Port for SOCKS5 server |

Note: `SOCKSIP` must differ from `VPNIP` if both features are enabled.

## Local Testing

### 1. Build and Initialize Server

```bash
cd server
go build .
./server --config
```

Copy the admin password from the log output:
```
ADMIN PASSWORD (change this!!) pass=XXXXXX
```

### 2. Enable SOCKS Feature

Edit `server/config.json`:
```json
{
  "Features": ["LAN", "VPN", "AUTH", "DNS", "BBOLT", "SOCKS"],
  "SOCKSPort": "1080"
}
```

Restart the server:
```bash
./server
```

### 3. Start Client

```bash
cd cmd/main
go build .
./main
```

### 4. Test Flow

1. Open `https://127.0.0.1:7777`
2. Login with `admin` and the password from step 1
3. Connect to a tunnel
4. Click "Enable Proxy" in the tunnel dropdown
5. Test the proxy:

```bash
curl -x socks5://127.0.0.1:1080 https://ifconfig.me
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v3/proxy/request` | POST | Request signed proxy token |
| `/v3/proxy/connect` | POST | Enable proxy for client IP |

Both endpoints require AUTH feature to be enabled.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "SOCKS and VPN cannot use same IP" | Set different values for `SOCKSIP` and `VPNIP` |
| Proxy connection refused | Ensure "Enable Proxy" was clicked first |
| Port 80 permission denied | Use a higher port like `1080` |
| Proxy stops working after 24h | Click "Enable Proxy" again to refresh whitelist |
