#!/usr/bin/env bash
# Builds the dummy native plugin used by the .so ABI test.
# The output stays out of git: *.so is gitignored, and the test skips without it.
#
# Toolchain: zig c++ is preferred, g++ is the fallback (see
# tools/build-native.sh). This fixture keeps the default visibility (no
# -fvisibility=hidden) so the extern "C" block is exported as-is.
set -euo pipefail
cd "$(dirname "$0")"

exec ../../../../tools/build-native.sh . libdummy.so \
	-std=c++17 -O2 -fPIC -shared -Wl,-s
