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
PACKAGE_DARWIN="$SCRIPT_DIR/scripts/package-darwin-app.sh"

mkdir -p "$BIN_DIR"

echo "==> Building frontend"
./build-ui.sh

build_windows() {
    echo "==> Building windows/amd64"
    # rsrc_windows_amd64.syso in cmd/wails embeds the taskbar/exe icon (see cmd/wails/build/)
    if [ ! -f cmd/wails/rsrc_windows_amd64.syso ]; then
        echo "    warning: cmd/wails/rsrc_windows_amd64.syso missing — Windows build will have no app icon"
        echo "    regenerate with: see cmd/wails/build/README.md"
    fi
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
    # Wails uses UTType for file dialogs on modern macOS SDKs but does not
    # link UniformTypeIdentifiers; without this, linking fails with:
    #   Undefined symbols: _OBJC_CLASS_$_UTType
    local darwin_cgo_ldflags="-framework UniformTypeIdentifiers"

    echo "==> Building darwin/arm64"
    CGO_ENABLED=1 CGO_LDFLAGS="$darwin_cgo_ldflags" GOOS=darwin GOARCH=arm64 go build \
        -tags "$TAGS" -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-app-darwin-arm64" \
        ./cmd/wails
    echo "    -> bin/tunnels-app-darwin-arm64"
    "$PACKAGE_DARWIN" \
        "$BIN_DIR/tunnels-app-darwin-arm64" \
        "$BIN_DIR/Tunnels-darwin-arm64.app" \
        "$VERSION"
    echo "    -> bin/Tunnels-darwin-arm64.app"

    echo "==> Building darwin/amd64"
    CGO_ENABLED=1 CGO_LDFLAGS="$darwin_cgo_ldflags" GOOS=darwin GOARCH=amd64 go build \
        -tags "$TAGS" -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-app-darwin-amd64" \
        ./cmd/wails
    echo "    -> bin/tunnels-app-darwin-amd64"
    "$PACKAGE_DARWIN" \
        "$BIN_DIR/tunnels-app-darwin-amd64" \
        "$BIN_DIR/Tunnels-darwin-amd64.app" \
        "$VERSION"
    echo "    -> bin/Tunnels-darwin-amd64.app"
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
ls -lh "$BIN_DIR"/tunnels-app-* "$BIN_DIR"/Tunnels-*.app 2>/dev/null || echo "    (no binaries built)"
