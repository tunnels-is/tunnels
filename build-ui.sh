#!/bin/bash
BASE=/home/sveinn/go-code/tunnels
m -R $BASE/frontend/dist 
m -R $BASE/cmd/main/dist 
cd $BASE/frontend
npm run build
cd ..
cp -R $BASE/frontend/dist ./cmd/main

