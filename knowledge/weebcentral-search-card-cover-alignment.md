---
type: finding
title: Weebcentral search covers landed on the wrong title - one card renders two img
description: "Weebcentral search covers landed on the wrong title - 2 img per card, positional zip drifted. Fix: match cover by ULID in URL, not by position."
tags: [goisekai, plugin, weebcentral, odin, wasm, scraping, cover, bug]
generated: { by: agent/mcp, at: "2026-10-26T15:54:05Z" }
status: stable
---

# Weebcentral search covers landed on the wrong title

Reported 2026-10-20: searching "dungeon" showed each result's cover next to the **previous** result's title. Root cause: every card renders **two `<img>` (desktop + mobile variants)**; the positional zip drifted by half a card - only the first cover was right.

## The fix: match by identity, not position

The cover URL carries the series ULID (https://temp.compsci88.com/cover/fallback/<ULID>.jpg), so a cover is matched to its card by looking for that ULID in the URL. The same change went into the Lua reference plugin (identical bug). ponytail: if the ULID ever leaves the URL the cover comes back empty and the title stays right - upgrade path: fall back to one img per picture anchor.

## Why this is invisible in the output

Both lists are non-empty, so nothing errors; only the pairing is wrong. find_list_attr skips elements without the attribute, so lists can have different lengths with no signal.

**General rule: never zip two independently-extracted lists. Join on an identifier the page itself provides.**

## Verification

- Live before: 1/32 covers matched. Live after: 32/32.
- 348/348 cover URLs contain a ULID.
- Tests: TestLuaWeebcentralCoversMatchTheirTitle (offline) + TestWasmWeebcentralCoversMatchTheirTitle (live).

## Same pattern elsewhere - not yet checked

mangakatana, mangataro, violetmanga, zeroscann, htmlscrape all pair titles[i] with covers[i] positionally.
