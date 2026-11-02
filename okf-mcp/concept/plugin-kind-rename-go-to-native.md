---
id: plugin-kind-rename-go-to-native
title: Rename the .so plugin kind from `go` to `native`
type: decision
created: 2026-11-02T00:00:00Z
author: human:kenzanin
status: accepted
---

# Rename plugin kind `go` → `native`

## Context

The plugin engine registry added in the
`plugin-engine-registry-quickjs` change resolves one runtime per plugin
`kind` through a single `map[string]engine`.

Five kinds are registered: `lua`, `js`, `go`, `yaegi`, `wasm`.

The `go` key does not point at a Go-script runtime — `loadGo` opens a shared
library with `openNativeLib` (purego `dlopen`/`dlsym`). The kind string therefore
names a language, not a runtime, and collides with `yaegi` (the Go-script
backend).

## Decision

Rename the `.so` plugin kind from `go` to `native`.

- `native` names the runtime the host runs (shared library via native loader).
- Covers Go-compiled and C/C++ shared libraries under the same backend.
- `pureGo` was rejected: it would mislabel C/C++ `.so` plugins that already run
  under this backend.

## Scope

- Registry key, kind literals, comments, tests, `KIND_BADGE` key updated.
- No DB migration: `plugins` table has no `kind` column (kind is runtime memory
  only).

## Verification

- `go build ./...` → BUILD_OK.
- Filtered pluginmanager tests → ok.
- `go test ./internal/templates/ -count=1` → ok.
- `just check` → 0 issues.
- `just build` → exit 0.
- Full pluginmanager suite: FAIL only on live-network Fenyk tests (upstream 530)
  — external, not a rename regression.

## Open items

- None.
