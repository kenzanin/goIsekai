#!/usr/bin/env bash
# Builds the dummy native plugin used by the .so ABI test.
# The output stays out of git: *.so is gitignored, and the test skips without it.
set -euo pipefail
cd "$(dirname "$0")"
g++ -std=c++17 -O2 -fPIC -shared main.cpp -o libdummy.so
echo "built libdummy.so"
