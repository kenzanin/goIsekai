#!/usr/bin/env bash
# Builds the madarascans native .so plugin.
#
# Only header-only RapidJSON is needed: HTTP, text stripping and HTML handling
# all go through the host bridge (native_abi.h), so there is no libcurl/libxml2
# link here.
#
# Toolchain: zig c++ is preferred, g++ is the fallback (see
# tools/build-native.sh). zig links its own libc++ and libgcc statically, so
# the result depends on libc.so.6 only - no libstdc++.so.6 / libgcc_s.so.1
# needed on the host. It costs size (roughly twice the g++ build) and a cold
# build of ~38 s; warm builds are <100 ms via zig's cache.
set -euo pipefail
cd "$(dirname "$0")"

# -fvisibility=hidden hides everything except the EXPORT-marked entry points
# (util.h), and -s strips the symbol tables the loader never reads.
exec ../../../../tools/build-native.sh . main.so \
	-std=c++17 -O2 -fPIC -shared -fvisibility=hidden -Wl,-s
