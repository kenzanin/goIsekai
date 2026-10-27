---
type: Reference
title: go-dl Go launcher reference
description: "Reference for the go-dl Go toolchain launcher (go1.26.9 for TinyGo wasm builds, go1.27.3 for host builds, no bare 'go' on PATH)."
tags: [go-dl, toolchain, tinygo, build, wasm, migrasi]
generated: { by: agent/mcp, at: "2026-10-27T13:24:51Z" }
status: stable
---

# go-dl Go launcher reference

## Launcher & SDK layout

The Go toolchain is managed by **go-dl** (command-versioned installs). Launchers live in
`~/go/bin/`; the matching GOROOT SDKs live in `~/sdk/`.

- Launcher: `~/go/bin/go1.26.9`, `~/go/bin/go1.27.3`  (command-versioned, NOT bare `go`)
- SDK / GOROOT: `~/sdk/go1.26.9`, `~/sdk/go1.27.3`
- NOTE: the user said "go1.26.0" but the installed binary is **go1.26.9** — this is what TinyGo needs.

## Bare `go` is NOT on PATH

`go` alone is not found; use the versioned launcher **or** set PATH explicitly:

```sh
export PATH="$HOME/sdk/go1.27.3/bin:$PATH"   # host builds
export PATH="$HOME/sdk/go1.26.9/bin:$PATH"    # TinyGo wasm plugin builds
```

## Which version for what

| Use                       | Go version | Why                                                                 |
| ------------------------- | ---------- | ------------------------------------------------------------------- |
| TinyGo wasm plugin builds | go1.26.9   | TinyGo 0.43.0 requires Go <= 1.26 (the SDK at ~/sdk/go1.26.9)        |
| Host binary (`go build`)  | go1.27.3   | go.mod requires go >= 1.27.2; go1.27.3 satisfies it (CGO_ENABLED=0) |

Do **NOT** use go1.27.3 for TinyGo — it breaks the wasm build.

## wasm plugin Makefile (TinyGo)

Each wasm plugin `examples/plugins/wasm/<id>/Makefile` sets:

```make
GO126  ?= $(HOME)/sdk/go1.26.9   # TinyGo needs Go <=1.26; NOT ~/.govman/.../go1.26.9
TINYGO ?= $(HOME)/opt/tinygo/bin/tinygo
```
Build via the Makefile only — never a manual `GOOS=wasip1 go build` (stdlib fallback OOMs the wasm runtime):

```sh
cd examples/plugins/wasm/<id>
make fix && make clean && make
cp main.wasm app_data/plugins/<id>/main.wasm
# clear cache + reload if server is up
```

## Verification

```sh
~/sdk/go1.26.9/bin/go version   # -> go version go1.26.9 linux/amd64
~/sdk/go1.27.3/bin/go version   # -> go version go1.27.3 linux/amd64
~/opt/tinygo/bin/tinygo version # -> tinygo version 0.43.0 linux/amd64
```
