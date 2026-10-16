#!/usr/bin/env bash
# Build TinyGo WASM plugins for goIsekai.
#
# TinyGo 0.41 wants Go <= 1.26, so this pins the toolchain to goup's 1.26.9:
#   $HOME/.goup/go1.26.9   (note: NOT ~/.goup/1.26.9, that path doesn't exist)
# The repo (root + plugin go.mods) declares `go 1.27.1`, which the old
# toolchain rejects, so the script temporarily relaxes those lines to
# `go 1.26` and restores them on exit via trap. No permanent changes.
#
# Usage: ./tools/build-wasm.sh [pluginID...]   (default: every dir with main.go)
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO126="$HOME/.goup/go1.26.9"
[ -x "$GO126/bin/go" ] || { echo "missing $GO126/bin/go (goup 1.26.9)" >&2; exit 1; }
export GOROOT="$GO126" PATH="$GO126/bin:$PATH" GOTOOLCHAIN=local CGO_ENABLED=0

# --- temporarily relax `go` directives for the old toolchain ---
TOUCHED=()
relax() { # $1 = go.mod path
  if grep -q '^go 1\.27' "$1"; then
    cp "$1" "$1.bak-wasmbuild"
    sed -i 's/^go 1\.27.*/go 1.26/' "$1"
    TOUCHED+=("$1")
  fi
}
restore() { for f in ${TOUCHED[@]+"${TOUCHED[@]}"}; do mv -f "$f.bak-wasmbuild" "$f"; done; }
trap restore EXIT INT TERM
relax "$ROOT/go.mod"
for d in "$ROOT"/examples/plugins/wasm/*/go.mod; do relax "$d"; done

build_one() { # $1 = plugin dir
  local dir="$1"
  echo "=== $(basename "$dir") ==="
  # NOTE: no -buildmode flag — that is standard-Go only and makes
  # wasm-opt fail with "duplicate export name" under TinyGo.
  if ( cd "$dir" && tinygo build -target wasi -o main.wasm . && ls -la main.wasm ); then
    return 0
  fi
  echo "-- tinygo failed, fallback: standard-go wasip1 c-shared"
  ( cd "$dir" && GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm . && ls -la main.wasm )
}

if [ "$#" -gt 0 ]; then
  for p in "$@"; do build_one "$ROOT/examples/plugins/wasm/$p"; done
else
  for f in "$ROOT"/examples/plugins/wasm/*/main.go; do build_one "$(dirname "$f")"; done
fi
