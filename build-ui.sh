#!/bin/bash
rm -Rf ./frontend/dist
rm -Rf ./cmd/main/dist
cd ./frontend
vite build
cd ..
cp -R ./frontend/dist ./cmd/main
