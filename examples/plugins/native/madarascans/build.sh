#!/usr/bin/env bash
# Builds the madarascans native .so plugin.
#
# Only header-only RapidJSON is needed: HTTP, text stripping and HTML handling
# all go through the host bridge (native_abi.h), so there is no libcurl/libxml2
# link here.
#
# Toolchain: tools/build-native.sh picks the best available tier -
# zig c++ against static musl (a .so with NO shared dependency at all),
# else zig c++ (glibc, needs libc.so.6 only), else g++ (needs
# libstdc++.so.6 + libgcc_s.so.1). A cold musl build is ~5 s; warm ones
# are <100 ms via zig's cache.
set -euo pipefail
cd "$(dirname "$0")"

# -fvisibility=hidden hides everything except the EXPORT-marked entry points
# (util.h), and -s strips the symbol tables the loader never reads.
exec ../../../../tools/build-native.sh . main.so \
	-std=c++17 -O2 -fPIC -shared -fvisibility=hidden -Wl,-s
