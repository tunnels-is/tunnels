#!/bin/bash
rm -R builds
goreleaser release --clean
