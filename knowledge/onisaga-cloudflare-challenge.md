---
type: Concept
title: Onisaga Cloudflare challenge behavior
description: Observed Cloudflare blocking on onisaga fetches and the cookie/TLS evidence so far.
tags: [goisekai, onisaga, cloudflare, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:39:05Z" }
status: stable
---

# Onisaga Cloudflare challenge behavior

## Observed blocking

- Direct genre-page curl → 403 (Cloudflare "Just a moment..." page).
- Direct /browse fetch → 403 now (worked before — cf_clearance expiry, not a code change).
- Direct sitemap fetch → 403 without fresh cookies.
- curl-impersonate-chrome with the freshest pasted Netscape cookies still → 403, 5.4KB challenge page, on both / and /genre/action.
- cf_clearance TTL is short (~30min); stored plugin_verify cookies go stale fast.

## Cookie evidence

- User pastes use Netscape cookies.txt format (Get cookies.txt Locally extension): XSRF-TOKEN, onisaga_session on onisaga.com, cf_clearance on .onisaga.com.
- Wizard parser (internal/hostnet, parseVerifyCookies) accepts raw cookies.txt paste, skipping comments and QueryUnescaping values; legacy formats preserved.
- Fresh cookie set (cf_clearance bound to timestamp 1791706620) still did not clear the challenge — cookies saved to /tmp/onisaga_cookies.txt and tested via impersonated curl, still 403.

## Known project facts reused here

- Onisaga Cloudflare challenge not solved by CDP: both engines (obscura, lightpanda) time out at 120s (cdp_solve_timeout) without clearing it — human-verify wizard (manual cookie paste) is the fallback path (memory #4282).
- Host TLS stack: tls-client v1.15.1, Chrome_146 profile ladder with WAF-escalation and per-plugin pinning (memory #2599).
- Host has curl-impersonate-* binaries (/usr/bin, ~/bin, ~/.local/bin) for out-of-band challenge probing.

## Implication for genre work

- Vocabulary harvesting must use the local DB or the app proxy rather than direct sitemap fetches (memory #4514).
- Do NOT live-test destructive maintenance knobs against the user's production cache (rule #3796).
