#!/bin/bash
# Wrap a Mach-O binary in a double-clickable macOS .app bundle.
#
# Usage:
#   ./scripts/package-darwin-app.sh <binary> <out.app> <version> [icon.png]
#
# version is used for CFBundleShortVersionString / CFBundleVersion (leading "v" stripped).
# icon.png defaults to cmd/fyne/appicon.png relative to the repo root.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BINARY="${1:?usage: package-darwin-app.sh <binary> <out.app> <version> [icon.png]}"
APP_PATH="${2:?}"
VERSION="${3:?}"
APPICON_PNG="${4:-$REPO_ROOT/cmd/fyne/appicon.png}"

BUNDLE_VERSION="${VERSION#v}"
BUNDLE_ID="is.tunnels.app"
APP_NAME="Tunnels"
# CFBundleExecutable: admin launcher script (double-click entrypoint).
EXEC_NAME="tunnels-app"
# Real Fyne binary lives next to the launcher inside Contents/MacOS/.
REAL_BIN_NAME="tunnels-app.bin"
ASKPASS_NAME="tunnels-askpass"

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

# Launcher elevates via `sudo -A` (GUI askpass), then exec's the real binary.
#
# Why not `osascript … with administrator privileges` + double-fork?
# That spawns an anon root process outside the user's LaunchServices/Aqua
# session → CFPasteboard XPC "Connection invalid", RunningBoard
# `anon<tunnels-app.bin>` / NotVisible, and a frozen Fyne UI on click.
#
# Starting through this .app entrypoint and `exec sudo -A …` keeps the process
# in the user GUI session chain (same pattern as `sudo App` from Terminal).
# No basePath / HOME / config-dir changes.
write_launcher() {
	local macos_dir="$1"
	local launcher="$macos_dir/$EXEC_NAME"
	local askpass="$macos_dir/$ASKPASS_NAME"

	cat >"$askpass" <<'ASKPASS'
#!/bin/bash
# Password helper for `sudo -A`. Prints the password on stdout; errors cancel sudo.
set -euo pipefail
osascript <<'APPLESCRIPT'
try
	set dlg to display dialog "Tunnels needs administrator privileges for network access (routes, tunnel interface, DNS)." default answer "" with title "Tunnels" with hidden answer buttons {"Cancel", "OK"} default button "OK"
	if button returned of dlg is "Cancel" then error number 1
	return text returned of dlg
on error
	error number 1
end try
APPLESCRIPT
ASKPASS
	chmod 755 "$askpass"

	cat >"$launcher" <<'LAUNCHER'
#!/bin/bash
# Tunnels.app entrypoint — elevate with sudo askpass, then become the Fyne binary.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
BIN="$DIR/tunnels-app.bin"
ASKPASS="$DIR/tunnels-askpass"

if [ ! -x "$BIN" ]; then
	osascript -e 'display dialog "Tunnels is missing its application binary (tunnels-app.bin)." buttons {"OK"} default button 1 with title "Tunnels" with icon stop' >/dev/null 2>&1 || true
	exit 1
fi

# Already root (e.g. nested sudo) — run the app directly.
if [ "$(id -u)" -eq 0 ]; then
	exec "$BIN" "$@"
fi

if [ ! -x "$ASKPASS" ]; then
	osascript -e 'display dialog "Tunnels is missing tunnels-askpass." buttons {"OK"} default button 1 with title "Tunnels" with icon stop' >/dev/null 2>&1 || true
	exit 1
fi

export SUDO_ASKPASS="$ASKPASS"
# -A: use askpass GUI instead of a TTY.
# exec replaces this launcher so Dock keeps a single Tunnels.app process that
# stays in the LaunchServices/GUI session (unlike osascript+double-fork).
exec sudo -A -p "" "$BIN" "$@"
LAUNCHER
	chmod 755 "$launcher"
}

TMP_ICNS="$(mktemp -t tunnels-icon).icns"
trap 'rm -f "$TMP_ICNS"' EXIT

make_icns "$APPICON_PNG" "$TMP_ICNS"

# Replace any existing bundle. Root-owned leftovers (logs/config from elevated
# runs) can make a plain rm -rf fail — move aside first when needed.
if [ -e "$APP_PATH" ]; then
	if rm -rf "$APP_PATH" 2>/dev/null; then
		:
	else
		trash="${APP_PATH}.old.$$"
		mv "$APP_PATH" "$trash"
		rm -rf "$trash" 2>/dev/null || true
		if [ -e "$APP_PATH" ]; then
			echo "error: cannot replace $APP_PATH (root-owned files?). Remove it with sudo and retry." >&2
			exit 1
		fi
	fi
fi
mkdir -p "$APP_PATH/Contents/MacOS" "$APP_PATH/Contents/Resources"

# Real Fyne binary + sudo askpass launcher (CFBundleExecutable = tunnels-app).
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
