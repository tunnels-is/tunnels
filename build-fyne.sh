#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

TARGET="${1:-linux}"
VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X github.com/tunnels-is/tunnels/version.Version=${VERSION}"
BIN_DIR="$SCRIPT_DIR/bin"
HOST_OS="$(go env GOHOSTOS)"

mkdir -p "$BIN_DIR"

if [ ! -f cmd/fyne/wintun.dll ]; then
    if [ -f cmd/main/wintun.dll ]; then
        cp -f cmd/main/wintun.dll cmd/fyne/wintun.dll
    elif [ -f cmd/wails/wintun.dll ]; then
        cp -f cmd/wails/wintun.dll cmd/fyne/wintun.dll
    else
        echo "error: cmd/fyne/wintun.dll is missing (copy from cmd/main)"
        exit 1
    fi
fi

if [ ! -f cmd/fyne/appicon.png ]; then
    if [ -f cmd/wails/build/appicon.png ]; then
        cp -f cmd/wails/build/appicon.png cmd/fyne/appicon.png
    fi
fi

build_linux() {
    if [ "$HOST_OS" != "linux" ]; then
        echo "==> Skipping linux (requires native Linux host, current: $HOST_OS)"
        return
    fi
    echo "==> Building linux/amd64 (Fyne)"
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
        -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-app-linux-amd64" \
        ./cmd/fyne
    echo "    -> bin/tunnels-app-linux-amd64"
}

windows_cc() {
    if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
        echo x86_64-w64-mingw32-gcc
        return
    fi
    if command -v zig >/dev/null 2>&1; then
        echo "zig cc -target x86_64-windows-gnu"
        return
    fi
    echo ""
}

build_windows() {
    echo "==> Building windows/amd64 (Fyne, CGO)"
    local cc
    cc="$(windows_cc)"
    if [ -z "$cc" ]; then
        echo "error: Windows CGO needs x86_64-w64-mingw32-gcc or zig"
        exit 1
    fi
    echo "    CC=$cc"
    # -H windowsgui is ignored by the external linker; also set PE subsystem.
    CC="$cc" CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build \
        -trimpath \
        -ldflags "$LDFLAGS -H windowsgui -extldflags=-Wl,--subsystem,windows" \
        -o "$BIN_DIR/tunnels-app-windows-amd64.exe" \
        ./cmd/fyne
    echo "    -> bin/tunnels-app-windows-amd64.exe"
}

build_darwin() {
    if [ "$HOST_OS" != "darwin" ]; then
        echo "==> Skipping darwin (requires native macOS host, current: $HOST_OS)"
        return
    fi
    echo "==> Building darwin/arm64 (Fyne)"
    CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build \
        -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-app-darwin-arm64" \
        ./cmd/fyne
    echo "    -> bin/tunnels-app-darwin-arm64"
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
