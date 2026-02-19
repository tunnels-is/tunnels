#!/bin/bash
set -e

# Build the Wails desktop client.
#
# Outputs go to ./bin/ in the project root.
#
# Usage:
#   ./build-wails.sh                    # build all possible targets
#   ./build-wails.sh windows            # Windows (cross-compiles from any OS)
#   ./build-wails.sh linux              # Linux (must build on Linux)
#   ./build-wails.sh darwin             # macOS (must build on macOS)
#
# Prerequisites:
#   - Go 1.22+, pnpm
#   - Linux:   sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev
#   - macOS:   xcode-select --install
#   - Windows: WebView2 runtime (bundled with Win 11, install on Win 10)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

TARGET="${1:-all}"
VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X github.com/tunnels-is/tunnels/version.Version=${VERSION}"
TAGS="desktop,production,devtools"
BIN_DIR="$SCRIPT_DIR/bin"
HOST_OS="$(go env GOHOSTOS)"

mkdir -p "$BIN_DIR"

# --- Frontend ---
echo "==> Building frontend"
rm -rf ./frontend/dist ./cmd/wails/dist
cd ./frontend
pnpm install --frozen-lockfile
pnpm run build
cd ..
cp -R ./frontend/dist ./cmd/wails
cp ./cmd/main/wintun.dll ./cmd/wails/wintun.dll 2>/dev/null || true

# --- Build helpers ---
build_windows() {
    echo "==> Building windows/amd64"
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
        -tags "$TAGS" -trimpath -ldflags "$LDFLAGS -H windowsgui" \
        -o "$BIN_DIR/tunnels-desktop-windows-amd64.exe" \
        ./cmd/wails
    echo "    -> bin/tunnels-desktop-windows-amd64.exe"
}

build_linux() {
    if [ "$HOST_OS" != "linux" ]; then
        echo "==> Skipping linux (requires native Linux host, current: $HOST_OS)"
        return
    fi
    echo "==> Building linux/amd64"
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
        -tags "$TAGS" -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-desktop-linux-amd64" \
        ./cmd/wails
    echo "    -> bin/tunnels-desktop-linux-amd64"
}

build_darwin() {
    if [ "$HOST_OS" != "darwin" ]; then
        echo "==> Skipping darwin (requires native macOS host, current: $HOST_OS)"
        return
    fi
    echo "==> Building darwin/arm64"
    CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build \
        -tags "$TAGS" -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-desktop-darwin-arm64" \
        ./cmd/wails
    echo "    -> bin/tunnels-desktop-darwin-arm64"

    echo "==> Building darwin/amd64"
    CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build \
        -tags "$TAGS" -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-desktop-darwin-amd64" \
        ./cmd/wails
    echo "    -> bin/tunnels-desktop-darwin-amd64"
}

# --- Dispatch ---
case "$TARGET" in
    windows) build_windows ;;
    linux)   build_linux ;;
    darwin)  build_darwin ;;
    all)
        build_windows
        build_linux
        build_darwin
        ;;
    *)
        echo "Unknown target: $TARGET"
        echo "Usage: $0 [windows|linux|darwin|all]"
        exit 1
        ;;
esac

echo ""
echo "==> Done"
ls -lh "$BIN_DIR"/tunnels-desktop-* 2>/dev/null || echo "    (no binaries built)"
