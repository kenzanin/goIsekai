#!/usr/bin/env bash
# Build TinyGo WASM plugins for goIsekai
set -e

# TinyGo requires Go 1.19-1.26, but we have Go 1.27.
# Workaround: use go build with -buildmode=c-shared instead
# TinyGo's wasip1 build uses wasip1-go which is compatible with c-shared

echo "=== Building reimanga_net plugin ==="
cd /home/kenzanin/tmp/goIsekai/examples/plugins/wasm/reimanga_net

# TinyGo build for wasip1
tinygo build -o main.wasm -target wasip1 -buildmode=c-shared main.go 2>&1 || {
  echo "TinyGo failed, trying alternative build..."
  # Fallback: use standard Go with tinygo's wasip1 toolchain
  GOOS=wasip1 GOARCH=wasm tinygo build -o main.wasm main.go
}

ls -la main.wasm
