#!/usr/bin/env bash
# Build this plugin's main.wasm (delegates to tools/build-wasm.sh:
# TinyGo pinned to Go 1.26.9, standard-Go wasip1 fallback).
set -e
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
exec "$ROOT/tools/build-wasm.sh" "$(basename "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)")"
