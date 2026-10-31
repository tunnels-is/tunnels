#!/bin/bash
# Sign GoReleaser archives, SBOMs, and the checksum, then attach the
# bundles to the draft release. Usage: release-sign-dist.sh [dist-dir] [--no-upload]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

dist="$ROOT/builds"
upload=1
for arg in "$@"; do
	case "$arg" in
	--no-upload) upload=0 ;;
	-*)
		echo "error: unknown flag: $arg" >&2
		exit 1
		;;
	*) dist="$arg" ;;
	esac
done

art="$dist/artifacts.json"
if [ ! -f "$art" ]; then
	echo "error: artifacts file not found: $art" >&2
	exit 1
fi

py=python3
if ! command -v python3 >/dev/null 2>&1; then
	py=python
fi

list="$(mktemp)"
trap 'rm -f "$list"' EXIT
"$py" - "$art" >"$list" <<'PY'
import json, sys
want = {"Archive", "SBOM", "Checksum"}
with open(sys.argv[1], encoding="utf-8") as fh:
    arts = json.load(fh)
for item in arts:
    if item.get("type") in want:
        print(item["path"])
PY

files=()
while IFS= read -r line; do
	[ -n "$line" ] || continue
	files+=("$line")
done <"$list"

if [ "${#files[@]}" -eq 0 ]; then
	echo "error: no archive, SBOM, or checksum artifacts in $art" >&2
	exit 1
fi

resolve() {
	local p="$1" base
	if [ -f "$p" ]; then
		printf '%s\n' "$p"
		return
	fi
	if [ -f "$ROOT/$p" ]; then
		printf '%s\n' "$ROOT/$p"
		return
	fi
	base="$(basename "$p")"
	if [ -f "$dist/$base" ]; then
		printf '%s\n' "$dist/$base"
		return
	fi
	echo "error: artifact not found: $p" >&2
	exit 1
}

bundles=()
for rel in "${files[@]}"; do
	path="$(resolve "$rel")"
	bash "$SCRIPT_DIR/release-sign.sh" "$path"
	bundles+=("${path}.sigstore.json")
done

if [ "${#bundles[@]}" -eq 0 ]; then
	echo "error: no signature bundles were written" >&2
	exit 1
fi

if [ "$upload" -eq 0 ]; then
	exit 0
fi

tag="${GITHUB_REF_NAME:-}"
if [ -z "$tag" ]; then
	echo "error: GITHUB_REF_NAME is empty" >&2
	exit 1
fi

gh release upload "$tag" "${bundles[@]}" --clobber
