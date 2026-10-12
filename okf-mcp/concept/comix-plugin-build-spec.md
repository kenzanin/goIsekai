---
id: comix-plugin-build-spec
title: Comix.to plugin build spec
type: finding
tags: [comix, plugin, lua, cdp, dom-harvest, referer]
---

# Comix.to plugin build spec

Source: `comix_to.har` + live rod probes (commit `9ac8fda` arc). CSR Next.js shell; page list lives in an encrypted blob (`out_len=0` via JSON.parse hook — dead), so the only working path is rendered-DOM harvesting through CDP.

## Rules

- **Images need NO Referer** — any `Referer` (origin or foreign) → 403. Site uses `referrerpolicy=no-referrer`. Host ladder must append an empty-string candidate (done: `internal/bridge/image.go`).
- **Never the signed API**: search/chapters REST wants `_=gfs.{jwt}` time-bound token; signer lives in `secure-*.js` = virtualized obfuscator VM. Reversal abandoned; token replay is time-dead within minutes.
- **`no_pages` error instead of returning `[]`** — host `GetPageListCached` masks empty results, so an empty return caches as a permanent failure.
- Chapter URL scheme: `/title/{slug}/{threadid}-chapter-N`.

## GetPageList recipe (CDP DOM harvest)

1. initJS (EvalOnNewDocument): `localStorage.setItem('reader.webtoon.v3', 'all')` — preload all pages.
2. Navigate, then bounded wait (never `WaitStable` — see hostnet-cdp-timeout-waits).
3. Sweep: `document.querySelectorAll('.rpage-page')` + `el.scrollIntoView()` per element, poll-converge on `img.src` count. `.rpage` is an 800px fixed viewport — **window scroll is dead**; only per-element `scrollIntoView` triggers the virtualized img injection (static DOM has ~3 img tags, real count 28+).
4. No canvas/scramble path needed — all harvested URLs are plain `<img src>` webp.
5. **go-rod Eval**: JS must be a function *expression* — strip trailing `()` from IIFEs or `.apply is not a function` fires (rod wraps as `(<js>).apply(this, arguments)`).

## Status

All four ABI paths live-verified (111-page e2e via sandbox), committed `9ac8fda` + hostnet `29d7b87`.
