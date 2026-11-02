---
type: decision
title: "Mangzio genres: per-item search genres via balanced RSC object; catalog GetGenres via filter buttons"
description: "Mangzio search genres were missing because the genre array sits far (>800B) from the slug fields in the SSR RSC payload - fixed by parsing the balanced object with findJSONObject instead of a window guess; the plugin-level genre FILTER comes from optional GetGenres export which must scrape catalog button labels, and the plugins.genres cache is sticky so a stale \"[]\" must be nulled before refetch."
tags: [goisekai, mangzio, plugins, genres, getGenres, rsc, quickjs, cache-gotcha, deploy]
generated: { by: agent/mcp, at: "2026-11-02T15:01:38Z" }
status: stable
---

## What broke and why

Mangzio (JS plugin, quickjs) served search results without genres, and the /view/search genre FILTER was empty.

Per-item `genres` in search results:
- The SSR RSC payload places `"genres":["Action",...]` far after `slug/title/coverImageUrl` (~1.5KB past it), so a naive `chunk.indexOf('"genres":[', re.lastIndex)` with an 800-byte window missed it (and an earlier draft referenced an undefined `combined`, causing `ReferenceError` in live search - fixed by removing the stray variable).
- Fix: for each manga-card regex match, parse the FULL balanced object with the plugin's existing `findJSONObject([chunk], '"slug":"' + slug + '"')` helper and read `obj.genres || []`. This aborts a window guess and matches the detail-page approach.

Genre FILTER (checkbox list on /view/search):
- It is served by the bridge `AppService.ListGenres(pluginID)`, which is cached in `plugins.genres` column with sticky semantics: NULL = fetch once and persist; "[]" = plugin has no genre export, NEVER re-invoke.
- Mangzio previously had no `getGenres` (the optional export; JS ABI maps `types.GetGenresFunc -> getGenres`), so the cache was poisoned with "[]".
- Fix: added `function getGenres()` in main.js that fetches `<BASE>/en/manga`, regexes pill buttons `<button>...</button>` (skipping sort/status labels like Latest Update, A-Z, Most Chapters, Rating, All, Ongoing, Completed, Hiatus), and returns `{name, slug}` pairs with name==slug.
- IMPORTANT: after adding the export, the cached "[]" had to be nulled manually: `UPDATE plugins SET genres = NULL WHERE id='mangzio'` - sticky cache otherwise hides the new export forever.

## Commits
- 3b58cff mangzio: chapter released_at reads `createdAt` (site has no `releaseDate`; live objects carry createdAt/updatedAt) + search_page_size 24 declaration.
- ce39aab search per-item genres via findJSONObject (balanced RSC object), +4 -9.
- e6e0b3f mangzio: implement GetGenres from catalog filter buttons, +22.
- All pushed to origin (3b58cff..e6e0b3f). Verified live: /api/search?pluginID=mangzio&q=hero returns genre arrays per item; /view/search?pluginID=mangzio renders 20+ genre checkboxes (Action..Sci-fi etc.).

## Gotchas for future JS plugins
1. For per-item metadata attached to a card, parse the whole balanced object, don't guess a byte window; the producer of the data has no contract on field ordering or distance.
2. After adding/removing an OPTIONAL export like GetGenres, the `plugins.genres` cache must be cleared (NULL) or the host will keep the previous answer forever.
3. A JS ReferenceError in a native surface aborts plugin calls until fixed (`combined` was undefined in one draft) - node --check catches syntax but not reference errors; live curl of the API was needed to catch the ReferenceError.
4. quickjs `SetEvalTimeout` surfaces as a timeout error; long-running fetch-loops in getGenres-like helpers must stay under the VM budget.
5. Similar plugins that lack `get_genres`: the audit earlier this session flagged NO-genres plugins: mangatown, sanascans, violetmanga (lua), and NOFILE compiled wasm sources under app_data (need source rebuild). The same get_genres + current cache reset treatment applies to those.

# Related Concepts
- Related to [Host owns the chapter date format: epoch seconds or verbatim prose, never a layout guess](host-date-source-of-truth.md)
