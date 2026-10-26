---
type: finding
title: Hostnet CDP timeouts and page waits for SPA sites
description: Hostnet CDP timeouts and page waits for SPA sites
tags: [goisekai, hostnet, cdp, rod, timeout, spa, plugin, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:41:13Z" }
status: stable
---

# Hostnet CDP timeouts and page waits for SPA sites

Source: comix/mangataro arcs (commits 29d7b87, 9ac8fda). Applies to every SPA/challenge site.

## Rules

- Per-plugin CDP solve budget: PLUGIN.timeout key threads into Proxy.pluginCDPTimeout (internal/hostnet/proxy.go). Without this, plugins with slow challenges die at the host default (cdp_solve_timeout).
- Never page.WaitStable / WaitLoad on SPA pages: never-idle pages or missing loadEventFired block until the full ctx budget, and the old code discarded its own timeout (_ = ctx). Fix: page = page.Context(ctx) then Navigate + bounded time.Sleep(2s).
- Bounded waits must stay ctx-bound: any rod waiter without page.Context(ctx) runs on context.Background with no cap.

## go-rod Eval contract

- page.Eval here = Evaluate(Eval(js).ByPromise()) — AwaitPromise always on.
- Input must be a function expression: rod wraps as `function(){return (<js>).apply(this, arguments)}` — an IIFE with trailing () self-invokes and .apply lands on the return value. Strip () from any JS passed to Eval.

## Failure signatures

| Symptom | Cause |
|---|---|
| evaluate hangs 2m, no cdp evaluate stable log | WaitStable on never-idle SPA / _ = ctx |
| apply is not a function | IIFE trailing () in Eval input |
| plugin timeout ignored, dies at host budget | PLUGIN.timeout not wired to pluginCDPTimeout |
| obscura cdp_path must be a ws:// URL, got chrome binary | fallback chain gave a daemon the chrome binary path |
| lightpanda never attempted although configured | the instant obscura failure consumed the tried >= 2 budget |
| module closed with context deadline exceeded at exactly 15001 ms | solve outlived the 15 s invoke budget |

## Fallback-chain bugs (fixed 2026-10-20)

1. Daemons were handed the chrome binary path. solveChallenge copied the whole config and swapped only attempt.Engine, leaving Path as the chrome binary. Fix: cfg.fallbackChain() method; an engine only enters it when cfg.pathFor(engine) has the right shape — ws:///wss for daemons, anything non-empty for chrome.
2. Solves outlived their caller. cdp_solve_timeout often 120s while invoke is 15s. solveAndSeed now clamps the per-attempt timeout to time.Until(ctx.Deadline()) when the ctx has one and it is shorter.
3. yaegi bridge dropped the invoke context. callYaegi builds a deadline but the host bridge called proxy.Request() (context.Background()). Now threads m.invokeCtx(id) through RequestContext.

All three covered by internal/hostnet/cdp_engines_test.go. go vet still reports pre-existing unsafe.Pointer finding at goplug.go:127 (//nolint:govet).
