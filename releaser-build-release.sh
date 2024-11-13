#!/bin/bash
export GITHUB_TOKEN=$1
goreleaser release --clean
cp builds/server_linux_amd64_v1/tunnels devops/build/

