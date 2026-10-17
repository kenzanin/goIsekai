#!/usr/bin/env bash
# Build this plugin's main.wasm via wasi-sdk clang++ (C++17 reactor) and verify
# the exports the host requires.
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

# ── verify the exports the host loads (ABI + reactor CRT) ──
OBJDUMP="${WASM_OBJDUMP:-/sbin/wasm-objdump}"
for sym in _initialize contract_version malloc free Search GetMangaDetail GetChapterList GetPageList; do
    "$OBJDUMP" -x main.wasm | grep -q "\"$sym\"" || { echo "missing export: $sym" >&2; exit 1; }
done

echo "exports OK ($(stat -c%s main.wasm) bytes)"
