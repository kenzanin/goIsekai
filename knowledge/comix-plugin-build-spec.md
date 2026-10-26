---
type: finding
title: Comix.to plugin build spec
description: "Comix.to plugin build spec - CSR Next.js, encrypted page blob harvested via CDP DOM, no Referer on images."
tags: [goisekai, plugins, comix, lua, cdp, dom-harvest, referer]
generated: { by: agent/mcp, at: "2026-10-26T15:55:51Z" }
status: stable
---

# Comix.to plugin build spec

Source: comix_to.har + live rod probes (commit 9ac8fda).

## Rules

- Images need NO Referer - any Referer (origin or foreign) -> 403.
- Never the signed API: search/chapters REST wants _=gfs.{jwt} time-bound token; signer lives in secure-*.js VM. Abandoned.
- no_pages error instead of []: host GetPageListCached masks empty results.
- Chapter URL: /title/{slug}/{threadid}-chapter-N.

## GetPageList recipe (CDP DOM harvest)

1. initJS: localStorage.setItem('reader.webtoon.v3', 'all') - preload all pages.
2. Navigate, bounded wait (never WaitStable).
3. Sweep: document.querySelectorAll('.rpage-page') + el.scrollIntoView() per element, poll-converge on img.src count. .rpage is 800px fixed viewport - window scroll is dead.
4. All harvested URLs are plain <img src> webp.
5. go-rod Eval: JS must be function expression - strip trailing () from IIFEs.

## Status

All four ABI paths live-verified (111-page e2e), committed 9ac8fda + hostnet 29d7b87.
