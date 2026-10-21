#!/usr/bin/env bash
# Builds the dummy native plugin used by the .so ABI test.
# The output stays out of git: *.so is gitignored, and the test skips without it.
#
# Toolchain: tools/build-native.sh picks the best available tier -
# zig c++ against static musl (a .so with NO shared dependency at all),
# else zig c++ (glibc, needs libc.so.6 only), else g++ (needs
# libstdc++.so.6 + libgcc_s.so.1).
set -euo pipefail
cd "$(dirname "$0")"

exec ../../../../tools/build-native.sh . libdummy.so \
	-std=c++17 -O2 -fPIC -shared -Wl,-s
