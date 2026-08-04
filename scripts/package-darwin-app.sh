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
# CFBundleExecutable: admin launcher script (double-click entrypoint).
EXEC_NAME="tunnels-app"
# Real Wails binary lives next to the launcher inside Contents/MacOS/.
REAL_BIN_NAME="tunnels-app.bin"

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

# Launcher + elevated runner — only elevate and start the binary.
# No basePath / HOME / config-dir changes; the app uses its normal defaults.
write_launcher() {
	local macos_dir="$1"
	local launcher="$macos_dir/tunnels-app"
	local runner="$macos_dir/tunnels-app-run"

	cat >"$runner" <<'RUNNER'
#!/bin/bash
# Runs as root (via administrator privileges). Detaches tunnels-app.bin, then exits.
# Does not pass flags or change paths — binary behaves as if launched directly.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
BIN="$DIR/tunnels-app.bin"

if [ ! -x "$BIN" ]; then
	exit 1
fi

# Double-fork + new session so the GUI survives when the elevated shell exits,
# and so the user-level launcher can exit (one Dock icon).
if [ -x /usr/bin/perl ]; then
	/usr/bin/perl -e '
		use strict;
		use warnings;
		use POSIX qw(setsid);
		exit 0 if fork;
		setsid();
		exit 0 if fork;
		open STDIN,  "<",  "/dev/null" or exit 1;
		open STDOUT, ">",  "/dev/null" or exit 1;
		open STDERR, ">",  "/dev/null" or exit 1;
		exec { $ARGV[0] } @ARGV;
		exit 127;
	' "$BIN" "$@"
	exit 0
fi

trap '' HUP
"$BIN" "$@" &
exit 0
RUNNER
	chmod +x "$runner"

	cat >"$launcher" <<'LAUNCHER'
#!/bin/bash
# Tunnels.app entrypoint — admin prompt, start binary, exit.
# No basePath / env / config changes.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
RUNNER="$DIR/tunnels-app-run"
BIN="$DIR/tunnels-app.bin"

if [ ! -x "$BIN" ]; then
	osascript -e 'display dialog "Tunnels is missing its application binary (tunnels-app.bin)." buttons {"OK"} default button 1 with title "Tunnels" with icon stop' >/dev/null 2>&1 || true
	exit 1
fi
if [ ! -x "$RUNNER" ]; then
	osascript -e 'display dialog "Tunnels is missing its elevated runner (tunnels-app-run)." buttons {"OK"} default button 1 with title "Tunnels" with icon stop' >/dev/null 2>&1 || true
	exit 1
fi

if [ "$(id -u)" -eq 0 ]; then
	exec "$RUNNER" "$@"
fi

if ! osascript -e "do shell script quoted form of \"${RUNNER}\" with administrator privileges"; then
	osascript -e 'display dialog "Tunnels needs your Mac administrator password to run (DNS port 53 and network setup)." buttons {"OK"} default button 1 with title "Tunnels" with icon caution' >/dev/null 2>&1 || true
	exit 1
fi

exit 0
LAUNCHER
	chmod +x "$launcher"
}

TMP_ICNS="$(mktemp -t tunnels-icon).icns"
trap 'rm -f "$TMP_ICNS"' EXIT

make_icns "$APPICON_PNG" "$TMP_ICNS"

rm -rf "$APP_PATH"
mkdir -p "$APP_PATH/Contents/MacOS" "$APP_PATH/Contents/Resources"

# Real binary (Wails) + launcher scripts (CFBundleExecutable = tunnels-app).
cp "$BINARY" "$APP_PATH/Contents/MacOS/$REAL_BIN_NAME"
chmod +x "$APP_PATH/Contents/MacOS/$REAL_BIN_NAME"
write_launcher "$APP_PATH/Contents/MacOS"

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

echo "packaged $APP_PATH (launcher + $REAL_BIN_NAME)"
