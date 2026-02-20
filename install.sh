#!/bin/sh
set -eu

REPO="tunnels-is/tunnels"
INSTALL_DIR="/opt/tunnels"
SERVICE_NAME="tunnels"
BINARY_NAME="tunnels"

# --- helpers ---

info()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
err()   { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || err "'$1' is required but not found"
}

# --- preflight ---

[ "$(id -u)" -eq 0 ] || err "This script must be run as root (try: curl ... | sudo sh)"

need curl
need tar
need grep
need uname

# --- detect platform ---

OS=$(uname -s)
ARCH=$(uname -m)

case "$OS" in
  Linux)  OS_LABEL="Linux"  ;;
  *)      err "Unsupported OS: $OS (only Linux is supported for server installs)" ;;
esac

case "$ARCH" in
  x86_64)       ARCH_LABEL="amd64" ;;
  aarch64|arm64) ARCH_LABEL="arm64" ;;
  armv7l|armv6l) ARCH_LABEL="arm"   ;;
  *)             err "Unsupported architecture: $ARCH" ;;
esac

info "Detected platform: ${OS_LABEL}/${ARCH_LABEL}"

# --- resolve latest version ---

info "Fetching latest release from ${REPO}..."

LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | head -1 \
  | sed 's/.*"tag_name": *"//;s/".*//')

[ -n "$LATEST" ] || err "Could not determine latest release"

VERSION="${LATEST#v}"
info "Latest version: ${LATEST}"

# --- build download URL ---

ASSET="server_${VERSION}_${OS_LABEL}_${ARCH_LABEL}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${ASSET}"

# --- download and install ---

info "Downloading ${ASSET}..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fSL --progress-bar -o "${TMP}/${ASSET}" "$URL" \
  || err "Download failed. Check that the release asset exists: ${URL}"

info "Installing to ${INSTALL_DIR}..."

mkdir -p "$INSTALL_DIR"
tar -xzf "${TMP}/${ASSET}" -C "$INSTALL_DIR" "$BINARY_NAME"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

# --- systemd service ---

if command -v systemctl >/dev/null 2>&1; then
  UNIT="/etc/systemd/system/${SERVICE_NAME}.service"

  if [ ! -f "$UNIT" ]; then
    info "Creating systemd service..."

    cat > "$UNIT" <<EOF
[Unit]
Description=Tunnels VPN Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME}
WorkingDirectory=${INSTALL_DIR}
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME"
    info "Service created and enabled (${SERVICE_NAME})"
  else
    info "Systemd unit already exists, restarting..."
    systemctl restart "$SERVICE_NAME"
  fi
fi

# --- done ---

printf '\n'
info "Tunnels ${LATEST} installed to ${INSTALL_DIR}/${BINARY_NAME}"
info ""
info "  Start:   systemctl start ${SERVICE_NAME}"
info "  Status:  systemctl status ${SERVICE_NAME}"
info "  Logs:    journalctl -u ${SERVICE_NAME} -f"
printf '\n'
