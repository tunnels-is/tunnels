#!/bin/bash
goreleaser build --snapshot --clean
cp builds/server_linux_amd64_v1/tunnels devops/build/

