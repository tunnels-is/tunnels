#!/bin/bash
rm -R ./frontend/dist 
rm -R ./cmd/main/dist 
cd ./frontend
vite build
cd ..
cp -R ./frontend/dist ./cmd/main

