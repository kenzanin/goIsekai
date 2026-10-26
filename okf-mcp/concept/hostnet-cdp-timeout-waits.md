---
description: "Hostnet CDP timeouts and page waits for SPA sites"
id: hostnet-cdp-timeout-waits
title: Hostnet CDP timeouts and page waits for SPA sites
type: finding
tags: [hostnet, cdp, rod, timeout, spa, plugin]
---

# Hostnet CDP timeouts and page waits for SPA sites

Source: comix/mangataro arcs (commits `29d7b87`, `9ac8fda`). Applies to every SPA/challenge site — the class keeps recurring, so treat this as the default CDP posture for new plugins.

## Rules

- **Per-plugin CDP solve budget**: `PLUGIN.timeout` key threads into `Proxy.pluginCDPTimeout` (`internal/hostnet/proxy.go`) — a plugin declaring `timeout = 60` gets 60s browser solves, not the host default `cdp_solve_timeout`. Without this, plugins with slow challenges die at the host budget (comix's 19.6s failure).
- **Never `page.WaitStable` / `WaitLoad` on SPA pages**: never-idle pages (mangataro) or missing `loadEventFired` block until the full ctx budget, and the old code discarded its own timeout (`_ = ctx`). Fix shape: `page = page.Context(ctx)` then `Navigate` + bounded `time.Sleep(2s)` — rod returns on commit; the sleep is the render window.
- **Bounded waits must stay ctx-bound**: any rod waiter without `page.Context(ctx)` runs on `context.Background` with no cap.

## go-rod Eval contract

- `page.Eval` here = `Evaluate(Eval(js).ByPromise())` — AwaitPromise always on, async function expressions fine.
- Input must be a **function expression**: rod wraps as `function(){return (<js>).apply(this, arguments)}` — an IIFE with trailing `()` self-invokes and `.apply` lands on the return value (`apply is not a function`). Strip `()` from any JS passed to Eval.

## Failure signatures

| Symptom | Cause |
|---|---|
| evaluate hangs 2m, no `cdp evaluate stable` log | WaitStable on never-idle SPA / `_ = ctx` |
| `apply is not a function` | IIFE trailing `()` in Eval input |
| plugin timeout ignored, dies at host budget | `PLUGIN.timeout` not wired to `pluginCDPTimeout` |
| `obscura cdp_path must be a ws:// URL, got "/sbin/google-chrome-stable"` | fallback chain gave a daemon the chrome binary path |
| `lightpanda` never attempted although configured | the instant obscura failure consumed the `tried >= 2` budget |
| `module closed with context deadline exceeded` at exactly 15001 ms | solve outlived the 15 s invoke budget instead of respecting it |

## Fallback-chain bugs (fixed 2026-10-20, commit `21244f3`)

A 20-line log covering 12:10–12:19 mixed five problem classes; only the first two
were host bugs (the rest — site-side blocking on onisaga/comix.to, and the
`legacy ABI name` warning at `internal/pluginmanager/lua.go:82` — are expected).

**1. Daemons were handed the chrome binary path.** `solveChallenge` copied the
whole config and swapped only `attempt.Engine`, leaving `Path` as the chrome
binary. `cdp_rod.go` requires `ws://` for `obscura`/`lightpanda`, so both failed
instantly — and because the loop breaks at `tried >= 2`, a chrome timeout plus one
instant daemon failure consumed the entire budget: **lightpanda was never reached**
even when installed. Fix: the chain is now a method on `CDPConfig`
(`cfg.fallbackChain()`) and an engine only enters it when `cfg.pathFor(engine)` has
the right shape — `ws://`/`wss://` for daemons, anything non-empty for chrome.
Per-engine endpoints are new config keys `cdp_obscura_path` / `cdp_lightpanda_path`,
because `cdp_path` must stay the chrome binary; empty means "not installed" and the
engine is skipped. There is deliberately **no** fallback to `cdp_path` for a daemon
— that inheritance *was* the bug.

**2. Solves outlived their caller.** `cdp_solve_timeout` is often 120 s while a
plugin invoke is 15 s (`invokeTimeout`, `manager.go:22`) or 30 s
(`interactiveExecTimeout`, `bridge/jobs.go:22`). Each attempt burned its full
engine timeout for a caller that had already left, twice over. `solveAndSeed` now
clamps the per-attempt timeout to `time.Until(ctx.Deadline())` when the ctx has one
and it is shorter. Without a deadline the configured value is untouched.

**3. The yaegi bridge dropped the invoke context.** `callYaegi` builds a deadline
but the host bridge called `proxy.Request()` (`context.Background()`), so an HTTP
fetch or CDP solve from interpreted code could outlive the invoke — the exact class
the Lua/JS/Go runtimes already handled. It now threads `m.invokeCtx(id)` through
`RequestContext` like the others.

**Testing note:** all three are covered by `internal/hostnet/cdp_engines_test.go`
(chain shape, per-engine endpoint, circuit breaker, timeout clamp). The chain is
pure logic — no browser needed. `go vet` still reports the pre-existing
`unsafe.Pointer` finding at `goplug.go:127`, which carries `//nolint:govet`.
