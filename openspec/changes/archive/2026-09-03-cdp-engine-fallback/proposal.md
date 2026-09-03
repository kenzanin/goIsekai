## Why

Manga sources increasingly sit behind anti-bot interstitials (Cloudflare Turnstile, DataDome) that the current tls-client fast path cannot pass — it only *detects* the challenge (403/503 + marker) and hands off to a manual cookie-paste flow. That flow is slow and friction-heavy: every protected site needs a human to open DevTools and copy `cf_clearance`. A CDP browser engine (lightpanda or Chrome, user-selected in settings) lets the host solve the challenge automatically, harvest the session cookies, inject them into the plugin's existing cookie jar, and retry the fast path — no manual paste.

## What Changes

- Add a host-owned CDP engine layer that drives an external browser (lightpanda or Chrome, selected in settings) via the Chrome DevTools Protocol to solve an anti-bot challenge and extract the resulting cookies.
- Extend the hostnet request flow so that when a challenge response is detected, the host (a) optionally runs the CDP engine to harvest cookies, (b) injects them via the existing per-plugin verify-cookie path, and (c) retries the fast path — instead of immediately surfacing `ChallengeError`.
- Add a plugin ABI hint `needs_js` (analogous to the existing `needs_human_verify` / `verify_url`) so a plugin can declare its site renders client-side and should skip the fast path in favor of the browser engine.
- Add user settings for the CDP engine: `cdp_engine` (`off` | `lightpanda` | `chrome`) and `cdp_path` (binary location). Default `off`; the feature is opt-in and no binary ships with goIsekai.

**Non-goals:** JS-rendered HTML extraction (a `render()` ABI) is explicitly out of scope — most JS-rendered official sites expose a hidden JSON API, and the fan-scanlation aggregators this app targets are server-rendered. No per-plugin engine *selection*; the engine is a host infrastructure concern. No auto-download of the browser binary.

## Capabilities

### New Capabilities

- `cdp-engine`: Host-managed browser engine for solving anti-bot challenges and harvesting session cookies, selectable and configured at the host level (not by plugins).

### Modified Capabilities

- `host-network`: The host HTTP request flow gains an automatic CDP fallback — on challenge detection, solve via the browser engine, inject cookies, and retry — rather than only surfacing a challenge error.
- `plugin-runtime`: Plugin Init metadata gains an optional `needs_js` hint so a plugin can declare its site requires a JavaScript-capable engine.

## Impact

- `internal/hostnet/` — new `cdp.go` (engine driver) and a fallback hook in `request.go`; `verify.go` reused for cookie injection.
- `internal/config/` — new `cdp_engine` / `cdp_path` settings (ini + CLI flags).
- `internal/pluginmanager/` — pass `needs_js` through Init metadata (WASM + Lua parity).
- New dependency: a pure-Go CDP driver (`github.com/chromedp/chromedp`, CGO-free).
- No breaking changes; the challenge banner and manual verify flow remain as the fallback when the engine is `off` or fails.
