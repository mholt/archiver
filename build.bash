#!/usr/bin/env bash
set -ex

# This script builds archiver for most common platforms.

export CGO_ENABLED=0

pushd cmd/arc
GOOS=linux   GOARCH=amd64         go build -o ../../builds/arc_linux_amd64
GOOS=linux   GOARCH=arm64         go build -o ../../builds/arc_linux_arm64
GOOS=linux   GOARCH=arm           go build -o ../../builds/arc_linux_armv7
GOOS=linux   GOARCH=arm   GOARM=6 go build -o ../../builds/arc_linux_armv6
GOOS=darwin  GOARCH=amd64         go build -o ../../builds/arc_mac_amd64
GOOS=windows GOARCH=amd64         go build -o ../../builds/arc_windows_amd64.exe
GOOS=windows GOARCH=arm           go build -o ../../builds/arc_windows_armv7.exe
popd
