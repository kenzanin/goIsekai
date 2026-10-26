---
id: mangadex-api-title-phrase-vs-website-search
title: MangaDex API /manga?title= returns 0 for multi-word phrases that the website /search matches
description: >
  The MangaDex REST API (api.mangadex.org/manga?title=<q>) uses exact-ish phrase
  matching that differs from mangadex.org/search. A 4-word phrase
  ("Isekai Shoukansareta kedo") yields 0 API hits while the site shows matches.
  The mangadex Odin wasm plugin now falls back to shorter title prefixes.
type: finding
tags: [goisekai, plugin, mangadex, wasm, odin, search, api, phrase-matching, fallback]
generated:
  at: "2026-10-27T04:30:00Z"
---

# MangaDex API `/manga?title=` returns 0 for phrases the website `/search` matches

## Summary

The mangadex Odin wasm plugin (`examples/plugins/wasm/mangadex_odin/main.odin`)
hit `api.mangadex.org/manga?title=<q>` and got `[]` for multi-word phrases that
`mangadex.org/search?q=<q>` resolves. The API `title=` param does exact-ish
phrase matching; the website uses word/token ranking. A 4-word query like
`"Isekai Shoukansareta kedo"` returned 0 from the API while the site showed
matches — so `/view/search?pluginID=mangadex&q=Isekai+Shoukansareta+kedo`
rendered no cards.

## Root cause

Two things stacked:

1. **API `title=` ≠ website `/search`.** `api.mangadex.org/manga?title=X` requires
   the words in `X` to appear together as a phrase in a manga title. The
   website `/search?q=X` tokenizes and ranks word-by-word, so a phrase with a
   rare last word (e.g. `...kedo` as in *"Isekai Shoukansareta kedo Demo
   Tabi Hajimemashita"*) matches the site but not the API.

2. **No fallback.** `search_impl` sent the full query once, broke on `#data == 0`,
   and returned `[]` — never tried a shorter prefix.

> Note: `r.URL.Query().Get("q")` in the host sandbox handler **does** decode
> `+`→space and `%20`→space correctly — this was first suspected (and a
> `+`-decode patch was applied then reverted) but was a red herring. The `+`
> handling was fine; the API genuinely returned 0 results.

## The fix

`search_impl` now splits the query into words (`searchWords`) and retries with
progressively shorter prefixes: full phrase → drop last word → drop again →
single first word. Stops on the first pass that returns data, or when only one
word remains.

```odin
searchWords := strings.split(title, " ") if title != "" else nil
maxWords := len(searchWords)
for w := maxWords; w >= 1; w -= 1 {
    searchQuery = searchWords[:w]   // first w words
    // ... build /manga?title=<join(searchQuery, " ")> ...
    // ... on n == 0 with no results, fall through to next w ...
    if hadResult || !searchedOnce || len(searchQuery) == 0 { break }
}
```

## Build

Build **only** via the plugin's own `build.sh` (Odin + wasm-objdump export check).
`odin` is at `/sbin/odin`. Never rebuild by hand:

```sh
cd examples/plugins/wasm/mangadex_odin
bash ./build.sh
```

## Verification (live)

- `q=isekai` → 200+ results (single-word, always worked).
- `q=Isekai+Shoukansareta+kedo` → **3 results** before fix: `[]`; after fix:
  3 matches (e.g. *"Powerless Without You"*). `view/search?pluginID=mangadex&q=...`
  renders 3 cards.

## Notes

- The same `/manga?title=` phrase-matching gap could affect the Lua reference
  (`examples/plugins/lua/mangadex/main.lua`) — it has the same URL shape. Not
  ported in this change but flagged.
- MangaDex is an **Odin** wasm plugin (`built_with: "odin dev-2026-10:84bc3fc21"`),
  not TinyGo — builds via `odin build . -target:wasi_wasm32`, not the Makefile.
