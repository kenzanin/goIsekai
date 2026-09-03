## Context

See proposal.md — Why. The hostnet layer (`internal/hostnet/`) already detects anti-bot challenges in `request.go` (403/503 + `IsChallengeResponse` marker, text/html guard) and surfaces a typed `ChallengeError`. It also already persists per-plugin verify cookies via `SetVerifyCookies` + a per-plugin UA override, and every plugin (WASM + Lua) routes its HTTP through `proxy.HandleRequest`. The only missing piece is a browser engine to *solve* the challenge automatically so cookies flow back into that existing jar.

Key constraints: builds must stay `CGO_ENABLED=0`; no browser binary ships with the app (lightpanda is AGPL + external + no Windows; Chrome is user-installed); tls-client can't execute JS.

## Goals / Non-Goals

**Goals:**
- Solve anti-bot challenges automatically via an external CDP browser (lightpanda or Chrome) selected in settings.
- Harvest solved cookies and hand them to the existing verify-cookie path, then retry the fast path.
- Give plugins a `needs_js` hint to skip the fast path for known-JS sites.
- Keep the manual verify flow as the fallback when the engine is off or fails.

**Non-Goals:**
- No `render()` ABI / JS-rendered HTML extraction (see proposal — most official sites expose a JSON API).
- No per-plugin engine *selection* — engine is host-owned.
- No auto-download or bundling of the browser binary.
- No headless-browser image capture or full browser proxying of every request.

## Decisions

### D1: Pure-Go CDP driver — chromedp over rolling our own
Drive the browser over CDP with `github.com/chromedp/chromedp` (pure Go, no CGO). Alternatives: `go-rod` (also pure Go, similar), or raw websocket. chromedp is the most widely used and well-documented; both keep `CGO_ENABLED=0`. Chose chromedp for ecosystem maturity. Both drivers treat lightpanda and Chrome the same — they both speak CDP.

### D2: Solve = navigate + wait, then harvest cookies; retry the *original* request
The engine's job is narrow: launch the browser, navigate to the challenge URL, wait for the interstitial to clear (poll for a "solved" signal or a timeout), then read cookies via `network.getAllCookies`. Harvested cookies + the browser's User-Agent go to `SetVerifyCookies`, and `request.go` retries the original request once on the fast path. We do NOT return the browser's rendered HTML — the fast path still does the real fetch (cheap, keeps the pipeline unchanged). This matches the Suwayomi reactive-fallback shape.

### D3: Engine selection is a host setting, not a plugin ABI field
`cdp_engine` (`off`|`lightpanda`|`chrome`) + `cdp_path` live in config/ini (like `host`/`port`/`log_level`). Plugins only get the boolean `needs_js` hint. Rationale: engine choice is infrastructure (which binary the user has installed), orthogonal to "does this site need JS". This keeps the ABI stable and lets the user switch engines without touching plugins.

### D4: Reuse the verify-cookie path for handoff
The engine writes into the same `SetVerifyCookies` store the manual flow uses. This means the UI's existing "Human Verification" status panel and the challenge banner work unchanged — the only difference is cookies arrive automatically instead of by paste. No new storage schema.

### D5: Process-per-solve, no long-lived pool (YAGNI)
Each challenge triggers a bounded browser launch (navigate → wait → harvest → close). No pool, no warm-keepalive — solves are rare and single-user. lightpanda's ~100ms cold start makes this cheap; Chrome's ~1s is acceptable for an occasional solve. Revisit only if solves become hot-path.

## Risks / Trade-offs

- **[Latency on first protected request]** — fast-fail then solve adds seconds once. → Mitigation: after a successful solve, cookies persist in the jar so later requests are full fast-path; cost is paid once per re-challenge window.
- **[cf_clearance is UA/fingerprint-bound]** — a cookie harvested by Chrome may be rejected if the fast path sends a different UA/fingerprint. → Mitigation: hand off the browser's exact UA via the existing UA override; document that lightpanda-solved cookies may not pass Cloudflare's stricter fingerprinting (Chrome is the reliable choice for CF).
- **[Binary absence / AGPL / platform]** — lightpanda is AGPL + no Windows; Chrome is heavy. → Mitigation: engine is opt-in (`off` default), `cdp_path` is user-supplied, and every failure path degrades to the existing challenge banner.
- **[Hung browser]** — a solve that never completes could block a request goroutine. → Mitigation: per-solve timeout (config, default ~30s) + process close on timeout.
- **[Scope creep into render()]** — tempting to also return rendered HTML. → Mitigation: explicit non-goal; the `needs_js` hint still routes through the solve→cookie→fast-path retry, never returns browser HTML.

## Migration Plan

Additive and opt-in: new settings default to `off`, new ABI field is optional and default-absent. Existing plugins and the manual verify flow are unaffected. No data migration. Rollback = set `cdp_engine=off` (or revert); no schema or protocol changes to undo.

## Open Questions

- Default solve timeout value (30s?) — safe to tune later without changing specs/tasks.
