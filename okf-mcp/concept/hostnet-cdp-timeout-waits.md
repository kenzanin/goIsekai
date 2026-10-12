---
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
