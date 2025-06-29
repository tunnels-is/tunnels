#!/bin/bash
# export GITHUB_TOKEN=$1
rm -R builds
goreleaser release --clean
