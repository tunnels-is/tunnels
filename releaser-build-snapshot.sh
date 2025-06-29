#!/bin/bash
rm -R builds
goreleaser build --snapshot --clean
