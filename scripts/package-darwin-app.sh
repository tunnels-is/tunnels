#!/bin/bash
# Wrap a Mach-O binary in a double-clickable macOS .app bundle.
#
# Usage:
#   ./scripts/package-darwin-app.sh <binary> <out.app> <version> [icon.png]
#
# version is used for CFBundleShortVersionString / CFBundleVersion (leading "v" stripped).
# icon.png defaults to cmd/wails/build/appicon.png relative to the repo root.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BINARY="${1:?usage: package-darwin-app.sh <binary> <out.app> <version> [icon.png]}"
APP_PATH="${2:?}"
VERSION="${3:?}"
APPICON_PNG="${4:-$REPO_ROOT/cmd/wails/build/appicon.png}"

BUNDLE_VERSION="${VERSION#v}"
BUNDLE_ID="is.tunnels.app"
APP_NAME="Tunnels"
EXEC_NAME="tunnels-app"

if [ ! -f "$BINARY" ]; then
	echo "error: binary not found: $BINARY" >&2
	exit 1
fi
if [ ! -f "$APPICON_PNG" ]; then
	echo "error: icon not found: $APPICON_PNG" >&2
	exit 1
fi

make_icns() {
	local png="$1"
	local out_icns="$2"
	local tmp iconset size name
	tmp="$(mktemp -d)"
	iconset="$tmp/icon.iconset"
	mkdir -p "$iconset"

	# iconutil requires this exact set of filenames/sizes.
	# Write @2x via temp names first — sips warns on @2x suffixes in --out paths.
	for size in 16 32 128 256 512; do
		name="icon_${size}x${size}"
		sips -z "$size" "$size" "$png" --out "$iconset/${name}.png" >/dev/null
		sips -z $((size * 2)) $((size * 2)) "$png" --out "$tmp/${name}@2x.png" >/dev/null
		mv "$tmp/${name}@2x.png" "$iconset/${name}@2x.png"
	done

	iconutil -c icns "$iconset" -o "$out_icns"
	rm -rf "$tmp"
}

TMP_ICNS="$(mktemp -t tunnels-icon).icns"
trap 'rm -f "$TMP_ICNS"' EXIT

make_icns "$APPICON_PNG" "$TMP_ICNS"

rm -rf "$APP_PATH"
mkdir -p "$APP_PATH/Contents/MacOS" "$APP_PATH/Contents/Resources"

cp "$BINARY" "$APP_PATH/Contents/MacOS/$EXEC_NAME"
chmod +x "$APP_PATH/Contents/MacOS/$EXEC_NAME"
cp "$TMP_ICNS" "$APP_PATH/Contents/Resources/iconfile.icns"

cat >"$APP_PATH/Contents/Info.plist" <<EOF
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

printf 'APPL????' >"$APP_PATH/Contents/PkgInfo"

# Ad-hoc sign so macOS treats it as a local app.
# Distribution still needs a Developer ID cert + notarization.
if command -v codesign >/dev/null 2>&1; then
	codesign --force --deep --sign - "$APP_PATH" 2>/dev/null || true
fi

echo "packaged $APP_PATH"
