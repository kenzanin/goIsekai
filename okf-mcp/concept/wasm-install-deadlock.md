---
description: "Installing a wasm plugin deadlocked the host"
id: wasm-install-deadlock
title: Installing a wasm plugin deadlocked the host — ensureWasmRuntime took the manager lock its caller already held
type: finding
tags: [goisekai, plugins, wasm, pluginmanager, concurrency, deadlock]
---

# Installing a wasm plugin deadlocked the host

Found 2026-10-20 while writing a live test for the weebcentral cover fix: the
test hung for the full 5-minute timeout and the panic stack pointed at a
self-deadlock in the plugin manager, not at the network.

## The lock cycle

```
Manager.Install        m.mu.Lock()          ← holds the manager lock
  └─ loadWasm
       └─ ensureWasmRuntime
            m.mu.Lock()                   ← takes it AGAIN
```

`sync.RWMutex` is **not reentrant**. The second `Lock()` on the same goroutine
blocks forever, so the host never returns from the install and the whole
process is stuck — not just that request.

`ensureWasmRuntime` guards the shared wazero runtime, and it reached for
`m.mu` because the runtime lives on the `Manager` struct. But `m.mu` guards
`m.plugins`, not the runtime. The two were conflated.

The same cycle existed on the other two entry points:

- `Manager.LoadPlugin` (hot-load from a folder) → `loadWasm` → same deadlock
- `Manager.Close` → touched `m.runtime` under `m.mu` while `ensureWasmRuntime`
  created it under `m.mu` too (not a deadlock on its own, but the same
  conflation)

This was **pre-existing**, not introduced by the info-script work: the
`m.mu.Lock()` in `ensureWasmRuntime` was already there before the wasm info
script commits. It was simply never hit, because the wasm plugin path was
only ever reached through `Discover` → lazy `ensureLoaded`, and `ensureLoaded`
takes `m.mu.RLock()` only to look the plugin up, releasing it before loading.

## The fix: a separate lock for the runtime

```go
// runtimeMu guards it separately from mu: ensureWasmRuntime is reached
// from Install and LoadPlugin, which already hold mu, and sync.RWMutex is
// not reentrant — taking it again self-deadlocks the host.
runtimeMu sync.Mutex
runtime   wazero.Runtime
```

`ensureWasmRuntime` and `Close` now take `runtimeMu`; the lock order stays
`mu -> runtimeMu` everywhere, so there is no cycle.

## Why it was invisible

- No test ever installed a wasm plugin through `Install` or `LoadPlugin`. The
  wasm tests used `Discover` + lazy load, which takes the read lock in a
  separate scope.
- The Lua/JS/Yaegi install paths never touched `ensureWasmRuntime`, so the
  plugin-install UI worked for them and only hung for `.wasm`.
- A deadlock in a request handler looks exactly like a slow network call from
  the outside, which is why it was first misread as a timeout.

## Regression tests

`internal/pluginmanager/wasm_install_deadlock_test.go` covers both entry
points (`Install` and `LoadPlugin`) with a 30s timeout — a regression
deadlocks there instead of hanging the suite:

- `TestInstallWasmPluginDoesNotDeadlock`
- `TestLoadPluginHotWasmDoesNotDeadlock`

**The general rule: a lock taken by a helper must never be one its caller
already holds.** When a helper needs its own state, it gets its own lock —
`m.mu` guards the plugin registry, `runtimeMu` guards the runtime,
`challengeMu` guards the challenge map, and `p.mu` guards one plugin.
