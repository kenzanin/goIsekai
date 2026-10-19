#!/usr/bin/env bash
# Build WASM plugins for goIsekai.
#
# Toolchain: TinyGo 0.41 wants Go <= 1.26, so a 1.26.x Go is located first
# (govman: ~/.govman/versions/go1.26.9). The repo (root + plugin go.mods)
# declares `go 1.27.1`, which that toolchain rejects, so those directives are
# temporarily relaxed to `go 1.26` and restored on exit via trap.
#
# TinyGo needs -buildmode=c-shared: plain `-target wasi` emits a *command*
# module (_start) while the host loads a *reactor* (_initialize). TinyGo also
# exports malloc/free/calloc/realloc from its own runtime, so the plugins' Go
# sources keep their own pair behind a build tag (abi_std.go / abi_tinygo.go) —
# declaring it twice aborts wasm-opt with "duplicate export name".
#
# Usage: ./tools/build-wasm.sh [pluginID...]   (default: every dir with main.go)
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

GO126="${GO126:-$HOME/.govman/versions/go1.26.9}"
if [ -x "$GO126/bin/go" ]; then
  export GOROOT="$GO126" PATH="$GO126/bin:$PATH"
elif command -v go >/dev/null && go version | grep -qE 'go1\.(19|2[0-6])\.'; then
  : # a Go <= 1.26 is already on PATH — use it
else
  echo "need Go <= 1.26 for TinyGo (set GO126 to such a GOROOT); got: $(go version 2>&1 || echo none)" >&2
  exit 1
fi
export GOTOOLCHAIN=local CGO_ENABLED=0

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
  # c-shared, not plain -target wasi: the host wants a reactor (_initialize).
  # malloc/free come from TinyGo's runtime; abi_tinygo.go stops the plugin from
  # redeclaring them, which is what used to make wasm-opt fail.
  if ( cd "$dir" && tinygo build -target wasi -buildmode=c-shared -o main.wasm . && ls -la main.wasm ); then
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
