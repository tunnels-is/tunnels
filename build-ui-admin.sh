#!/bin/bash
set -e

rm -Rf ./frontend-admin/dist
rm -Rf ./server/admin_dist
cd ./frontend-admin
pnpm install
pnpm run build
cd ..
cp -R ./frontend-admin/dist ./server/admin_dist
