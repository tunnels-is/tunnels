#!/bin/bash
rm -R ./frontend/dist 
rm -R ./cmd/main/dist 
cd ./frontend
npm run build
cd ..
cp -R ./frontend/dist ./cmd/main
