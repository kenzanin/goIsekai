#!/usr/bin/env bash
# Build this plugin's main.wasm with the Odin compiler (wasip1 reactor) and
# verify the exports the host requires.
#
# Unlike the C++ plugins there is no wasi-sdk, no rapidjson and no
# -Wl,--export=malloc/free: Odin emits the exports itself and allocates through
# core:mem's wasm allocator.
#
# _initialize is NOT exported and not needed — the module is a plain wasip1
# module (imports wasi_snapshot_preview1 + env.host_call), which the host
# instantiates with WithStartFunctions() and only then calls the ABI entries.
set -e
cd "$(dirname "${BASH_SOURCE[0]}")"

ODIN="${ODIN:-odin}"

"$ODIN" build . -target:wasi_wasm32 -no-entry-point -out:main.wasm
test -f main.wasm

# ── verify the exports the host resolves ──
OBJDUMP="${WASM_OBJDUMP:-/sbin/wasm-objdump}"
for sym in contract_version malloc free Search GetMangaDetail GetChapterList GetPageList; do
    "$OBJDUMP" -x main.wasm | grep -q "\"$sym\"" || { echo "missing export: $sym" >&2; exit 1; }
done

# The host resolves helpers through this one import; without it the plugin loads
# but every network call returns nothing.
"$OBJDUMP" -x main.wasm | grep -q "env.host_call" || { echo "missing import: env.host_call" >&2; exit 1; }

echo "exports OK ($(stat -c%s main.wasm) bytes)"
