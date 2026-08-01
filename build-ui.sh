#!/bin/bash
set -e

# Build the React frontend and stage it into every client entrypoint that
# embeds dist/ via go:embed (CLI + Wails desktop).

rm -rf ./frontend/dist ./cmd/main/dist ./cmd/wails/dist

cd ./frontend
pnpm install --frozen-lockfile
pnpm run build
cd ..

cp -R ./frontend/dist ./cmd/main
cp -R ./frontend/dist ./cmd/wails

# wintun.dll is go:embed'd by both Windows clients; keep the Wails copy in sync.
if [ -f ./cmd/main/wintun.dll ]; then
	cp -f ./cmd/main/wintun.dll ./cmd/wails/wintun.dll
fi
