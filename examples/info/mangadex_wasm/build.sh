#!/usr/bin/env bash
# Build the wasm MangaDex ENRICHMENT script with Odin (wasip1 reactor) and
# verify the exports the host requires of an info script.
#
# An info script exports GetEnrichment, NOT the scraper ABI (Search/
# GetMangaDetail/GetChapterList/GetPageList) — the host picks the required set
# from the "info:" id prefix.
set -e
cd "$(dirname "${BASH_SOURCE[0]}")"

ODIN="${ODIN:-odin}"

"$ODIN" build . -target:wasi_wasm32 -no-entry-point -out:main.wasm
test -f main.wasm

OBJDUMP="${WASM_OBJDUMP:-/sbin/wasm-objdump}"
for sym in contract_version malloc free GetEnrichment; do
    "$OBJDUMP" -x main.wasm | grep -q "\"$sym\"" || { echo "missing export: $sym" >&2; exit 1; }
done

# The host resolves every helper through this one import; without it the plugin
# loads but every network call returns nothing.
"$OBJDUMP" -x main.wasm | grep -q "env.host_call" || { echo "missing import: env.host_call" >&2; exit 1; }

echo "exports OK ($(stat -c%s main.wasm) bytes)"

"../../../tools/stamp-built-with.sh" . odin "$("$ODIN" version | awk '{print $3}')"
