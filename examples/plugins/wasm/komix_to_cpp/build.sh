#!/usr/bin/env bash
# Build komix_to_cpp main.wasm via zig cc (freestanding C++, no stdlib).
# Requires: zig (tested with 0.18.0-dev). Mirrors the `just build` recipe.
set -e
cd "$(dirname "${BASH_SOURCE[0]}")"
zig cc -x c++ -std=c++20 -target wasm32-freestanding -O2 main.cpp -o main.wasm -nostdlib -Wl,--no-entry
ls -la main.wasm
