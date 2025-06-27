# Auto-Update Mechanism for Tunnels CLI

## Overview

The Tunnels CLI client (`cmd/min`) now includes an automatic update mechanism that can keep IoT devices updated with the latest version without manual intervention.

## How it works

1. **Version Check**: On startup, the client calls `{authServer}/update` to get the latest version
2. **Version Comparison**: Compares the returned version with the current version (`2.0.0`)
3. **Download**: If a newer version is available, downloads the appropriate architecture binary from GitHub releases
4. **Backup**: Moves the current binary to `tunnels.prev` as a backup
5. **Update**: Extracts and replaces the current binary with the new version
6. **Restart**: Forks a new process with the same CLI parameters and exits the old process

## GitHub Release URL Format

The client downloads releases from:
```
https://github.com/tunnels-is/tunnels/releases/download/v{version}/min_{version}_{OS}_{ARCH}.tar.gz
```

Example:
```
https://github.com/tunnels-is/tunnels/releases/download/v2.0.2/min_2.0.2_Linux_arm64.tar.gz
```

## Supported Architectures

- **Linux**: x86_64, arm64, armv7, i386
- **Darwin** (macOS): x86_64, arm64
- **Windows**: x86_64, i386

## Command Line Options

- `--disableAutoUpdate`: Disable automatic updates on startup
- All existing CLI parameters are preserved during restart

## Error Handling

- **Network failures**: Logs error and continues with current version
- **Download failures**: Restores backup binary automatically
- **Timeout protection**: 5-minute timeout for the entire update process
- **Panic recovery**: Catches and logs any panics during update

## API Endpoint Requirements

The auth server must implement a `/update` endpoint that returns:

```json
{
  "version": "2.0.2"
}
```

## Security Considerations

- Downloads use HTTPS from GitHub releases
- Binary integrity should be verified (future enhancement)
- Backup mechanism prevents broken installations
- Update process runs with current user permissions

## Files Created

- `{binary_name}.prev`: Backup of the previous binary version
- Temporary download files are cleaned up automatically

## Logging

All update activities are logged using the existing client logging system:
- INFO: Normal update process steps
- ERROR: Failures and warnings
- DEBUG: Detailed debugging information

## Example Usage

```bash
# Normal startup with auto-update
./tunnels --authHost=api.tunnels.is --deviceID=abc123

# Disable auto-update
./tunnels --authHost=api.tunnels.is --deviceID=abc123 --disableAutoUpdate

# All parameters are preserved during restart
./tunnels --authHost=custom.server.com --secure --deviceID=abc123
```
