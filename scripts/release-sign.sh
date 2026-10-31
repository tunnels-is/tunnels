#!/bin/bash
# Keyless signature for one published file.
# The tag workflow calls this. GoReleaser does not sign: a signs block
# runs on snapshot builds, and a missing signature fails a local publish.
set -euo pipefail

file="${1:?usage: release-sign.sh <file>}"
if [ ! -f "$file" ]; then
	echo "error: file not found: $file" >&2
	exit 1
fi

bundle="${file}.sigstore.json"
rm -f "$bundle"
cosign sign-blob --yes --bundle="$bundle" "$file"
if [ ! -s "$bundle" ]; then
	echo "error: signature bundle is missing or empty: $bundle" >&2
	exit 1
fi
