#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

TARGET="${1:-linux}"
VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X github.com/tunnels-is/tunnels/version.Version=${VERSION}"
BIN_DIR="$SCRIPT_DIR/bin"
HOST_OS="$(go env GOHOSTOS)"
PACKAGE_DARWIN="$SCRIPT_DIR/scripts/package-darwin-app.sh"
APPICON="$SCRIPT_DIR/cmd/fyne/appicon.png"

mkdir -p "$BIN_DIR"

if [ ! -f cmd/fyne/wintun.dll ]; then
    if [ -f cmd/main/wintun.dll ]; then
        cp -f cmd/main/wintun.dll cmd/fyne/wintun.dll
    else
        echo "error: cmd/fyne/wintun.dll is missing (copy from cmd/main)"
        exit 1
    fi
fi

if [ ! -f "$APPICON" ]; then
    echo "error: cmd/fyne/appicon.png is missing"
    exit 1
fi

ensure_windows_icon() {
    if [ -f cmd/fyne/rsrc_windows_amd64.syso ]; then
        return
    fi
    if command -v go-winres >/dev/null 2>&1; then
        echo "==> Generating Windows PE icon (go-winres)"
        (cd cmd/fyne && go-winres make --in winres.json --arch amd64,arm64 --out rsrc)
        return
    fi
    echo "error: cmd/fyne/rsrc_windows_amd64.syso missing — the .exe will have no Explorer/taskbar icon"
    echo "    go install github.com/tc-hib/go-winres@latest"
    echo "    (cd cmd/fyne && go-winres make --in winres.json --arch amd64,arm64 --out rsrc)"
    exit 1
}

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
    # ELF files cannot embed an icon; ship a themed PNG + .desktop so the
    # file manager and dock can resolve "tunnels" / StartupWMClass=Tunnels.
    cp -f "$APPICON" "$BIN_DIR/tunnels.png"
    cp -f cmd/fyne/packaging/tunnels.desktop "$BIN_DIR/tunnels.desktop"
    echo "    -> bin/tunnels.png"
    echo "    -> bin/tunnels.desktop"
}

windows_cc() {
    if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
        echo x86_64-w64-mingw32-gcc
        return
    fi
    if [ "$(go env GOHOSTOS)" = "windows" ] && command -v gcc >/dev/null 2>&1; then
        echo gcc
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
    ensure_windows_icon
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

package_darwin() {
    local bin="$1"
    local app="$2"
    "$PACKAGE_DARWIN" "$bin" "$app" "$VERSION" "$APPICON"
    echo "    -> $app"
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
    package_darwin "$BIN_DIR/tunnels-app-darwin-arm64" "$BIN_DIR/Tunnels-darwin-arm64.app"

    echo "==> Building darwin/amd64 (Fyne)"
    CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build \
        -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-app-darwin-amd64" \
        ./cmd/fyne
    echo "    -> bin/tunnels-app-darwin-amd64"
    package_darwin "$BIN_DIR/tunnels-app-darwin-amd64" "$BIN_DIR/Tunnels-darwin-amd64.app"
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
ls -lh "$BIN_DIR"/tunnels-app-* "$BIN_DIR"/tunnels.png "$BIN_DIR"/tunnels.desktop "$BIN_DIR"/Tunnels-*.app 2>/dev/null || true
