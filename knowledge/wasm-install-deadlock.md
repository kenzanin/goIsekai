---
type: finding
title: Installing a wasm plugin deadlocked the host - ensureWasmRuntime took the manager lock its caller already held
description: Installing a wasm plugin deadlocked the host - ensureWasmRuntime took m.mu which its caller Install already held. Fixed with a separate runtimeMu.
tags: [goisekai, plugin, wasm, pluginmanager, concurrency, deadlock, bug]
generated: { by: agent/mcp, at: "2026-10-26T15:54:06Z" }
status: stable
---

# Installing a wasm plugin deadlocked the host

Found 2026-10-20 while writing a live test: the test hung for the full 5-minute timeout and the panic stack pointed at a self-deadlock in the plugin manager, not the network.

## The lock cycle

```
Manager.Install        m.mu.Lock()          <- holds the manager lock
  └─ loadWasm
       └─ ensureWasmRuntime
            m.mu.Lock()                   <- takes it AGAIN
```

sync.RWMutex is NOT reentrant. The second Lock() on the same goroutine blocks forever.

ensureWasmRuntime guards the shared wazero runtime, and reached for m.mu because the runtime lives on the Manager struct. But m.mu guards m.plugins, not the runtime. The two were conflated.

The same cycle existed on LoadPlugin (→ loadWasm → same deadlock) and Close.

This was PRE-EXISTING, not introduced by the info-script work: the m.mu.Lock() in ensureWasmRuntime was already there before the wasm info-script commits. Never hit because the wasm path was only reached through Discover → lazy ensureLoaded, which takes m.mu.RLock() only to look the plugin up, releasing it before loading.

## The fix: a separate lock for the runtime

```go
runtimeMu sync.Mutex
runtime   wazero.Runtime
```

ensureWasmRuntime and Close now take runtimeMu; lock order stays mu -> runtimeMu, no cycle.

## Why invisible

- No test installed wasm through Install or LoadPlugin; wasm tests use Discover + lazy load (read lock in a separate scope).
- Lua/JS/Yaegi install never touched ensureWasmRuntime.
- A deadlock in a request handler looks exactly like a slow network call from the outside.

## Regression tests

internal/pluginmanager/wasm_install_deadlock_test.go: TestInstallWasmPluginDoesNotDeadlock, TestLoadPluginHotWasmDoesNotDeadlock (30s timeout).

**General rule: a lock taken by a helper must never be one its caller already holds.**
