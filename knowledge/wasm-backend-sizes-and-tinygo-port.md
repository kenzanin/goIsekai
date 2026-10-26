---
type: Concept
title: WASM backend sizes and the TinyGo port (Go vs TinyGo vs C++ vs Odin)
description: "Measured size comparison of 4 wasm backends (C++ 108KB, Odin 416KB, TinyGo 1.5MB, std-Go 3.7MB), why TinyGo needs an ABI split, and the bogus sync.Map.Clear build error."
tags: [goisekai, plugins, wasm, tinygo, toolchain, abi, build]
generated: { by: agent/mcp, at: "2026-10-26T15:57:40Z" }
status: stable
---

# WASM backend sizes and the TinyGo port

All numbers measured. Same manga_uno source, four backends:

| backend | size | notes |
|---|---|---|
| C++17 (wasi-sdk clang) | 108 KB | no runtime |
| Odin (-target:wasi_wasm32) | 416 KB | core:encoding/json |
| TinyGo (-target wasi -buildmode=c-shared) | 1.5 MB | runtime exports malloc/free |
| standard Go (-buildmode=c-shared) | 3.7 MB | full runtime + reflection |

TinyGo only worth it when Go source must stay Go. On fresh plugin, C++ or Odin win ~10x.

## Why TinyGo needs an ABI split

1. -buildmode=c-shared required (command module vs reactor).
2. Do NOT declare own malloc/free - TinyGo runtime already exports them. wasm-opt aborts with duplicate export name.

Split by build tag: abi_std.go (!tinygo) has buffers map + wasmexport malloc/free + allocCopy; abi_tinygo.go (tinygo) has no redeclarations, allocCopy against TinyGo allocator. readString reads linear memory directly (host ptr not in buffers map).

## The bogus sync.Map.Clear error

NOT the plugins. A half-finished TinyGo upgrade (0.41 -> 0.43) left stale src/sync/map_go123.go, which collided with new map.go (both satisfied go1.23 tag). Extracting tarball on top of old install. Fix: extract each release into clean dir.

## Toolchain pinning

TinyGo needs Go <= 1.26, host default is 1.27. GOTOOLCHAIN=auto silently upgrades. Must set GOTOOLCHAIN=local + explicit GOROOT. tools/build-wasm.sh handles: locates $HOME/.goup/go1.26.9, relaxes go.mod to go 1.26 temporarily, restores via trap.

Build flag: `tinygo build -target wasi -o main.wasm .` (NOT -buildmode=c-shared - causes duplicate export). Fallback: std-Go wasip1 (3.7MB, working).
