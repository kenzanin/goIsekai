---
type: Concept
title: WASM plugin build scripts (TinyGo pinned to Go 1.26.9)
description: How to rebuild every wasm plugin's main.wasm; TinyGo 0.41 needs Go <=1.26 so the build pins goup's 1.26.9 and relaxes go.mod temporarily.
tags:
  - goisekai
  - plugin
  - wasm
  - tinygo
  - build
generated:
  by: process:reasonix
  at: "2026-10-17T00:00:00+07:00"
---

# WASM plugin build scripts

## Paths

- Master script: `tools/build-wasm.sh` (executable)
  - Usage: `./tools/build-wasm.sh [pluginID...]` (default: every dir with `main.go`)
- Per-plugin shim: `examples/plugins/wasm/<id>/build.sh` (executable, delegates
  with the plugin dir name). Exists for all Go plugins:
  `komix_to`, `ksgroupscans`, `kunmanga_online`, `mangak`, `mangakakalot`,
  `manga_uno`, `manhawato`, `reimanga_net`.
- C++ plugin `komix_to_cpp/build.sh` uses `zig cc` freestanding (see its README);
  it has its own `justfile` too — do not route it through TinyGo.

## Why pinned Go 1.26.9 (do not "fix" by upgrading)

- TinyGo 0.41 rejects Go 1.27 (`requires Go 1.19-1.26`); host default is 1.27.x.
- Correct path is `$HOME/.goup/go1.26.9` (note: `~/.goup/1.26.9` does NOT exist).
- Script sets `GOROOT=$HOME/.goup/go1.26.9`, `PATH` prepend, `GOTOOLCHAIN=local`,
  `CGO_ENABLED=0`. Verified: `tinygo version` then reports `go version go1.26.9`.
- Repo root + plugin `go.mod` files declare `go 1.27.1`, which the old toolchain
  rejects (`go.mod requires go >= 1.27.1`). The script temporarily relaxes those
  lines to `go 1.26` and restores them on exit via `trap` — verified no `.bak`
  leftovers after run.

## TinyGo flags that work (and one that breaks)

- Working: `tinygo build -target wasi -o main.wasm .`
- BROKEN, do not re-add: `-buildmode=c-shared` (standard-Go only) makes
  wasm-opt fail with `[parse exception: duplicate export name]`.
- Fallback when TinyGo fails: standard-Go reactor build
  `GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm .`
  (this is what currently produces working `main.wasm` ~4.1 MB for plugins
  written with `//go:wasmexport` + `//go:wasmimport` standard-Go style).

## ponytail

TinyGo path produces smaller binaries but current plugin sources use the
standard-Go reactor ABI (`malloc`/`free` map + packed i64 returns); a pure
TinyGo-ABI rewrite would shrink binaries — do it only if size matters.
