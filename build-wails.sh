#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

TARGET="${1:-all}"
VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X github.com/tunnels-is/tunnels/version.Version=${VERSION}"
TAGS="desktop,production"
BIN_DIR="$SCRIPT_DIR/bin"
HOST_OS="$(go env GOHOSTOS)"

mkdir -p "$BIN_DIR"

echo "==> Building frontend"
./build-ui.sh

build_windows() {
    echo "==> Building windows/amd64"
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
        -tags "$TAGS" -trimpath -ldflags "$LDFLAGS -H windowsgui" \
        -o "$BIN_DIR/tunnels-app-windows-amd64.exe" \
        ./cmd/wails
    echo "    -> bin/tunnels-app-windows-amd64.exe"
}

build_linux() {
    if [ "$HOST_OS" != "linux" ]; then
        echo "==> Skipping linux (requires native Linux host, current: $HOST_OS)"
        return
    fi
    echo "==> Building linux/amd64"
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
        -tags "$TAGS,webkit2_41" -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-app-linux-amd64" \
        ./cmd/wails
    echo "    -> bin/tunnels-app-linux-amd64"
}

build_darwin() {
    if [ "$HOST_OS" != "darwin" ]; then
        echo "==> Skipping darwin (requires native macOS host, current: $HOST_OS)"
        return
    fi
    echo "==> Building darwin/arm64"
    CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build \
        -tags "$TAGS" -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-app-darwin-arm64" \
        ./cmd/wails
    echo "    -> bin/tunnels-app-darwin-arm64"

    echo "==> Building darwin/amd64"
    CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build \
        -tags "$TAGS" -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-app-darwin-amd64" \
        ./cmd/wails
    echo "    -> bin/tunnels-app-darwin-amd64"
}

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
ls -lh "$BIN_DIR"/tunnels-app-* 2>/dev/null || echo "    (no binaries built)"
