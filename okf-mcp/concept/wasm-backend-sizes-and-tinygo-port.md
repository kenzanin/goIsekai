---
type: Concept
title: WASM backend sizes and the TinyGo port (Go vs TinyGo vs C++ vs Odin)
description: Measured size comparison of the four wasm backends, why TinyGo needed an ABI split, and the half-finished-TinyGo-upgrade failure mode that produced a bogus "sync.Map.Clear already declared" error.
tags:
  - goisekai
  - plugins
  - wasm
  - tinygo
  - toolchain
  - abi
generated:
  by: process:reasonix
  at: "2026-10-19T00:00:00Z"
---

# WASM backend sizes and the TinyGo port

All numbers below are **measured**, not estimated: every `main.wasm` was built
and `stat`-ed. The comparison table in `README.md` is generated from this.

## Size: which backend to pick

Same `manga_uno` source, four backends:

| backend | size | notes |
| --- | ---: | --- |
| C++17 (wasi-sdk clang) | 108 KB | no runtime; `-fno-exceptions -fno-rtti`; RapidJSON header-only |
| Odin (`-target:wasi_wasm32`) | 416 KB | `core:encoding/json`; exports come free |
| TinyGo (`-target wasi -buildmode=c-shared`) | 1.5 MB | same Go source; runtime exports malloc/free |
| standard Go (`-buildmode=c-shared`) | 3.7 MB | full runtime + reflection table DCE cannot strip |

Difficulty, easiest → hardest, for an *existing* Go plugin:

1. **standard Go** — it already builds; the only work is the `go.mod` relaxation the build script does.
2. **TinyGo** — same sources, different build flags + the ABI split below. Buys ~50–58%.
3. **Odin** — a real rewrite (different language, its own JSON decoding, manual `@(export)`), but no C toolchain and no `-Wl,--export` list.
4. **C++** — smallest output but needs wasi-sdk, manual `-Wl,--export=malloc` markers and a hand-rolled JSON layer in `wasm_abi.h`.

TinyGo is only worth it when the Go source must stay Go. On a fresh plugin,
C++ or Odin win by another order of magnitude — TinyGo removes only about half
of the Go runtime.

Measured std-Go → TinyGo for all eight Go plugins:

| plugin | standard Go | TinyGo | saved |
| --- | ---: | ---: | ---: |
| `manga_uno` | 3,709,966 | 1,530,371 | 58% |
| `reimanga_net` | 4,129,582 | 1,834,751 | 55% |
| `mangak` | 4,223,419 | 1,899,228 | 55% |
| `komix_to` | 4,337,955 | 2,110,343 | 51% |
| `kunmanga_online` | 4,354,477 | 2,122,986 | 51% |
| `mangakakalot` | 4,407,558 | 2,181,593 | 50% |
| `manhawato` | 4,385,986 | 2,164,253 | 50% |
| `ksgroupscans` | 4,427,832 | 2,186,868 | 50% |

## Why TinyGo needs an ABI split

Two things break when you take a working standard-Go plugin to TinyGo:

1. **`-buildmode=c-shared` is required.** Plain `-target wasi` emits a
   *command* module (`_start`); the host loads a *reactor*, so it needs
   `_initialize`.
2. **Do not declare your own `malloc`/`free`.** TinyGo's runtime already
   exports `malloc`/`free`/`calloc`/`realloc`. A plugin that declares them too
   aborts `wasm-opt` with `[parse exception: duplicate export name]`.

So the ABI glue is split by build tag, in every Go plugin:

- `abi_std.go` (`//go:build !tinygo`) — the `buffers` map plus
  `//go:wasmexport malloc` / `free` and `allocCopy`.
- `abi_tinygo.go` (`//go:build tinygo`) — no redeclarations; `allocCopy`
  against TinyGo's allocator.

Both files are byte-identical across all eight plugins (verified by md5).

`readString` must also change: with TinyGo's allocator the host's pointer is
**not** in the plugin's `buffers` map, so a table lookup always returns `""`.
It reads linear memory directly instead:

```go
func readString(ptr, n uint32) string {
	if n == 0 || ptr == 0 { return "" }
	return string(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), n))
}
```

Note `//go:wasmexport` in a **non-`main` package** gets a prefixed export name,
so this glue has to stay in `package main`.

TinyGo does set a `tinygo` build tag (confirmed with a probe build).

## The bogus `sync.Map.Clear already declared` error

Symptom: every TinyGo build failed with

```
tinygo/src/sync/map_go123.go:9:15: method Map.Clear already declared at tinygo/src/sync/map.go:86:15
```

Root cause was **not** the plugins, and not `sync` being imported by them.

**It was a half-finished TinyGo upgrade.** TinyGo `0.43` moved `Clear()` into
`src/sync/map.go` and deleted `src/sync/map_go123.go`; `0.41` still had both.
Extracting the new tarball *on top of* the old install left the stale
`map_go123.go` behind, where its `//go:build go1.23` tag satisfied and it
collided with the new `map.go`. Confirmed by inspecting both tarballs:
`tinygo0.41.1...tar.gz` contains `src/sync/map_go123.go`, `tinygo0.43.0...tar.gz`
does not.

Evidence it was the environment and not the code: `tinygo version` changed from
`0.41.1` to `0.43.0` **during the session**, and a minimal standalone module
with no `goisekai` imports failed identically. After the install settled,
`sync/` held only `map.go` and the same builds succeeded.

**Fix / prevention:** extract each TinyGo release into a *clean* directory
(`tinygo-<version>/`) rather than over the previous one; never `cp -r` a new
release on top of an old tree.

Amusingly the failure is self-consistent with the toolchain advice: the plugin
sources carry `go 1.26` directives, but the *TinyGo stdlib's* `go1.23` tag is
what pulled the orphan file in — the plugin's own `go` directive was never the
cause (the same failure occurred at `go 1.21`, `1.22`, `1.23` and `1.26`).

## Toolchain pinning

TinyGo wants Go ≤ 1.26, but the repo `go.mod` declares `go 1.27.1`. Two traps:

- `GOTOOLCHAIN=auto` silently upgrades a 1.26 binary to 1.27 to satisfy
  `go.mod`, so `go version` keeps reporting 1.27 even after switching. You must
  set `GOTOOLCHAIN=local` **and** an explicit `GOROOT`.
- `tools/build-wasm.sh` handles both: it locates
  `$HOME/.govman/versions/go1.26.9` (overridable via `GO126`), relaxes the
  `go 1.27.*` directives in the root and plugin `go.mod`s to `go 1.26` for the
  duration of the build, and restores them via a `trap` on exit.

The script falls back to `GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared`
if TinyGo fails, and prints that fallback — a silent 3.7 MB `main.wasm` means
the fallback ran.
