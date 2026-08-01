#!/bin/bash
set -e

rm -rf ./frontend/dist ./cmd/main/dist ./cmd/wails/dist

cd ./frontend
pnpm install --frozen-lockfile
pnpm run build
cd ..

cp -R ./frontend/dist ./cmd/main
cp -R ./frontend/dist ./cmd/wails

if [ -f ./cmd/main/wintun.dll ]; then
	cp -f ./cmd/main/wintun.dll ./cmd/wails/wintun.dll
fi
