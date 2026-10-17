#!/usr/bin/env bash
# Build komix_to_cpp main.wasm via wasi-sdk clang++ (C++17 reactor).
#
# Toolchain: /opt/wasi-sdk (clang 13, wasm32-wasi sysroot).
# JSON: RapidJSON (system header at /usr/include/rapidjson, header-only).
# wasm-ld links against libxml2.so.2; on distros shipping only libxml2.so.16
# a user-space symlink is added to LD_LIBRARY_PATH (no root needed).
set -e
cd "$(dirname "${BASH_SOURCE[0]}")"

WASI_SDK="${WASI_SDK:-/opt/wasi-sdk}"

# ── libxml2 shim for wasm-ld (needed on distros with libxml2.so.16 only) ──
SHIM=""
if ldd "$WASI_SDK/bin/wasm-ld" 2>/dev/null | grep -q "libxml2.so.2 => not found"; then
    SHIM="${TMPDIR:-/tmp}/wasi-libs"
    mkdir -p "$SHIM"
    [ -e "$SHIM/libxml2.so.2" ] || ln -sf "$(ldconfig -p | awk '/libxml2\.so /{print $NF; exit}')" "$SHIM/libxml2.so.2"
    export LD_LIBRARY_PATH="$SHIM${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
fi

"$WASI_SDK/bin/clang++" \
    -std=c++17 -O2 -fno-exceptions -fno-rtti \
    --target=wasm32-wasi -mexec-model=reactor \
    -idirafter /usr/include \
    main.cpp -o main.wasm \
    -Wl,--export=malloc -Wl,--export=free \
    -Wl,-z,stack-size=1048576 \
    2>&1 | grep -v "no version information available" || true

test -f main.wasm
ls -la main.wasm
