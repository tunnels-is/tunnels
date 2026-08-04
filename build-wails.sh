#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

TARGET="${1:-all}"
VERSION="${VERSION:-dev}"
# CFBundleShortVersionString should be a simple marketing version (no leading "v")
BUNDLE_VERSION="${VERSION#v}"
LDFLAGS="-s -w -X github.com/tunnels-is/tunnels/version.Version=${VERSION}"
TAGS="desktop,production"
BIN_DIR="$SCRIPT_DIR/bin"
HOST_OS="$(go env GOHOSTOS)"
APPICON_PNG="$SCRIPT_DIR/cmd/wails/build/appicon.png"
BUNDLE_ID="is.tunnels.app"
APP_NAME="Tunnels"
EXEC_NAME="tunnels-app"

mkdir -p "$BIN_DIR"

echo "==> Building frontend"
./build-ui.sh

# Build an .icns from the PNG source (required for Finder / Dock icon).
make_icns() {
    local png="$1"
    local out_icns="$2"
    local tmp iconset
    tmp="$(mktemp -d)"
    iconset="$tmp/icon.iconset"
    mkdir -p "$iconset"

    # iconutil requires this exact set of filenames/sizes.
    # Write via temp *.png names first — sips warns on @2x suffixes in --out paths.
    local size name
    for size in 16 32 128 256 512; do
        name="icon_${size}x${size}"
        sips -z "$size" "$size" "$png" --out "$iconset/${name}.png" >/dev/null
        sips -z $((size * 2)) $((size * 2)) "$png" --out "$tmp/${name}@2x.png" >/dev/null
        mv "$tmp/${name}@2x.png" "$iconset/${name}@2x.png"
    done

    iconutil -c icns "$iconset" -o "$out_icns"
    rm -rf "$tmp"
}

# Wrap a Mach-O binary in a double-clickable macOS .app bundle.
package_darwin_app() {
    local binary="$1"
    local app_path="$2"
    local icns_path="$3"

    if [ ! -f "$binary" ]; then
        echo "    error: binary not found: $binary" >&2
        return 1
    fi
    if [ ! -f "$icns_path" ]; then
        echo "    error: icon not found: $icns_path" >&2
        return 1
    fi

    rm -rf "$app_path"
    mkdir -p "$app_path/Contents/MacOS" "$app_path/Contents/Resources"

    cp "$binary" "$app_path/Contents/MacOS/$EXEC_NAME"
    chmod +x "$app_path/Contents/MacOS/$EXEC_NAME"
    cp "$icns_path" "$app_path/Contents/Resources/iconfile.icns"

    # CFBundleIconFile is the basename without .icns
    cat > "$app_path/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>en</string>
	<key>CFBundleExecutable</key>
	<string>${EXEC_NAME}</string>
	<key>CFBundleIconFile</key>
	<string>iconfile</string>
	<key>CFBundleIdentifier</key>
	<string>${BUNDLE_ID}</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>${APP_NAME}</string>
	<key>CFBundleDisplayName</key>
	<string>${APP_NAME}</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>${BUNDLE_VERSION}</string>
	<key>CFBundleVersion</key>
	<string>${BUNDLE_VERSION}</string>
	<key>LSMinimumSystemVersion</key>
	<string>10.15.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>NSHumanReadableCopyright</key>
	<string>Copyright © Tunnels.is</string>
</dict>
</plist>
EOF

    # PkgInfo marks this as an application package (optional but conventional).
    printf 'APPL????' > "$app_path/Contents/PkgInfo"

    # Ad-hoc sign so macOS treats it as a local app (avoids some Gatekeeper friction).
    # Distribution still needs a Developer ID cert + notarization.
    if command -v codesign >/dev/null 2>&1; then
        codesign --force --deep --sign - "$app_path" 2>/dev/null || true
    fi
}

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
    local icns_path="$BIN_DIR/tunnels-app.icns"

    if [ ! -f "$APPICON_PNG" ]; then
        echo "    error: missing app icon: $APPICON_PNG" >&2
        exit 1
    fi

    echo "==> Creating macOS app icon (.icns)"
    make_icns "$APPICON_PNG" "$icns_path"
    echo "    -> bin/tunnels-app.icns"

    echo "==> Building darwin/arm64"
    CGO_ENABLED=1 CGO_LDFLAGS="$darwin_cgo_ldflags" GOOS=darwin GOARCH=arm64 go build \
        -tags "$TAGS" -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-app-darwin-arm64" \
        ./cmd/wails
    echo "    -> bin/tunnels-app-darwin-arm64"
    package_darwin_app \
        "$BIN_DIR/tunnels-app-darwin-arm64" \
        "$BIN_DIR/Tunnels-darwin-arm64.app" \
        "$icns_path"
    echo "    -> bin/Tunnels-darwin-arm64.app"

    echo "==> Building darwin/amd64"
    CGO_ENABLED=1 CGO_LDFLAGS="$darwin_cgo_ldflags" GOOS=darwin GOARCH=amd64 go build \
        -tags "$TAGS" -trimpath -ldflags "$LDFLAGS" \
        -o "$BIN_DIR/tunnels-app-darwin-amd64" \
        ./cmd/wails
    echo "    -> bin/tunnels-app-darwin-amd64"
    package_darwin_app \
        "$BIN_DIR/tunnels-app-darwin-amd64" \
        "$BIN_DIR/Tunnels-darwin-amd64.app" \
        "$icns_path"
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
