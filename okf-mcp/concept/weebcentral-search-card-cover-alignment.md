---
id: weebcentral-search-card-cover-alignment
title: Weebcentral search covers were shifted onto the wrong title — one card renders two <img>, so covers must be matched by ULID, never by position
type: finding
tags: [goisekai, plugins, weebcentral, odin, wasm, scraping, cover]
---

# Weebcentral search covers landed on the wrong title

Reported 2026-10-20: searching "dungeon" showed each result's cover next to
the **previous** result's title. The HAR (`weebcentral.har`, 138 entries) and a
live fetch confirmed it: **31 of 32 covers belonged to another card**.

## Root cause: two <img> per card, one assumed

The plugin collected two parallel lists and zipped them by position:

```lua
local hrefs = host.html.find_list_attr(doc, "article.bg-base-300 a[href*='/series/']", "href")
local imgs   = host.html.find_list_attr(doc, "article.bg-base-300 img", "src")
...
cover_url = imgs[#out + 1] or "",
```

The code even carried a comment saying the two lists must not be zipped — and
then zipped them anyway, on a wrong premise:

> `hrefs` carries 2 anchors per card (picture + title) while `imgs` has 1 per
> card, so never index imgs by the href position.

`imgs` does **not** have 1 per card. Measured on the live `/search/data`
response, every one of the 32 cards renders **two** `<img>`:

| selector | count (32 cards) |
| --- | --- |
| `article.bg-base-300 a[href*='/series/']` | 64 (2 per card) |
| `article.bg-base-300 img` | 64 (2 per card) |
| `article.bg-base-300 a.line-clamp-1` | 32 (1 per card) |

Each card renders a **desktop** variant (`<article class="hidden lg:block">`)
and a **mobile** variant (`<article class="lg:hidden">`), each with its own
`<picture>` and `<img>`. Both carry the same URL, so a positional zip consumed
one image per card and drifted by half a card from the second card on — only
the first cover was right.

## The fix: match by identity, not position

The cover URL carries the series ULID
(`https://temp.compsci88.com/cover/fallback/<ULID>.jpg`), so a cover is
matched to its card by looking for that ULID in the URL:

```odin
cover_for :: proc (imgs: []string, ulid: string) -> string {
	for src in imgs {
		if strings.contains(src, ulid) {
			return src
		}
	}
	return ""
}
```

The same change went into the Lua reference plugin
(`examples/plugins/lua/weebcentral/main.lua`), which had the identical bug —
the wasm port inherited it rather than introducing it.

`ponytail:` the ULID in the cover URL is a property of the CDN's URL scheme. If
it ever goes away the cover comes back empty and the title stays right. Upgrade
path: fall back to one img per picture anchor
(`article.bg-base-300 a[href*='/series/']:not(.line-clamp-1) > article img`
measured 32, one per card, on the live page).

## Why this class of bug is invisible in the output

Both lists are non-empty and the same length-ish, so nothing errors: the
response is well-formed JSON with plausible covers. Only the pairing is wrong.
`find_list_attr` skips elements without the attribute, so the two lists can
also have *different* lengths with no signal at all.

**The general rule: never zip two independently-extracted lists from a web
page. Join them on an identifier the page itself provides** (a ULID, a slug,
a data attribute). Positional zipping is only safe when the page renders
exactly one element per record, which is exactly the assumption that was
never verified here.

## Verification

- Live before: 1/32 covers matched their own title. Live after: **32/32**.
- `348/348` cover URLs across every `/search/data` response in the HAR contain
  a ULID, so the join is total, not incidental.
- New regression test `TestLuaWeebcentralCoversMatchTheirTitle` (offline, stub
  server, renders the real two-`<img>` card shape) and
  `TestWasmWeebcentralCoversMatchTheirTitle` (live, `GOISEKAI_LIVE=1`).

## The same pattern elsewhere — not yet checked

`mangakatana`, `mangataro`, `violetmanga`, `zeroscann` and `htmlscrape` all
pair `titles[i]` with `covers[i]` positionally. Their selectors are narrower
(`.media img`, `.bsx > a img`), which may well yield one cover per card, but
that was never measured. The cheap check is the same one used here: count the
matches of the title selector and the cover selector against a live response
and compare them to the card count.
