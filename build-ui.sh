#!/bin/bash
m -R ./frontend/dist 
m -R ./cmd/main/dist 
cd ./frontend
npm run build
cd ..
cp -R ./frontend/dist ./cmd/main

