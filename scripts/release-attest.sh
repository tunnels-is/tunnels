#!/bin/bash
# SHA-256, SPDX SBOM, and keyless signatures for one desktop package.
# Usage: release-attest.sh <file>
set -euo pipefail

file="${1:?usage: release-attest.sh <file>}"
if [ ! -f "$file" ]; then
	echo "error: file not found: $file" >&2
	exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

py=python3
if ! command -v python3 >/dev/null 2>&1; then
	py=python
fi

"$py" - "$file" <<'PY'
import hashlib, os, sys
path = sys.argv[1]
digest = hashlib.sha256()
with open(path, "rb") as fh:
    for chunk in iter(lambda: fh.read(1024 * 1024), b""):
        digest.update(chunk)
name = os.path.basename(path)
out = path + ".sha256"
with open(out, "w", newline="\n") as fh:
    fh.write("%s  %s\n" % (digest.hexdigest(), name))
PY

sum="${file}.sha256"
if [ ! -s "$sum" ]; then
	echo "error: checksum is missing or empty: $sum" >&2
	exit 1
fi

sbom="${file}.sbom.json"
rm -f "$sbom"
syft "$file" --output "spdx-json=${sbom}"
if [ ! -s "$sbom" ]; then
	echo "error: SBOM is missing or empty: $sbom" >&2
	exit 1
fi

bash "$SCRIPT_DIR/release-sign.sh" "$file"
bash "$SCRIPT_DIR/release-sign.sh" "$sum"
bash "$SCRIPT_DIR/release-sign.sh" "$sbom"
