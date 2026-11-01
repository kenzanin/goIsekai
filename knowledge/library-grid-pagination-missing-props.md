---
type: finding
title: "Library grid had no pagination: the view never passed Page/TotalPages to the toolbar partial"
description: "Library/home grid rendered no pagination because views/library.lua never passed Page/TotalPages to libraryToolbar, so both defaulted to 1 and partials/pagination returned an empty string."
tags: [goisekai, library, web-ui, pagination, lua-templates, bug, golden-test-gap]
generated: { by: agent/mcp, at: "2026-11-01T02:45:34Z" }
status: stable
---

## Symptom

`/` and `/view/library` rendered the grid with **no pagination at all** — no page numbers, no Next, no Prev — even with 613 in-library titles (25 pages at pageSize 25).

## Root cause

`internal/templates/partials/library_toolbar.lua:13-14` reads the two values it needs:

```lua
local page = data.Page or 1
local totalPages = data.TotalPages or 1
```

but the caller, `internal/templates/views/library.lua:64-73`, never passed them:

```lua
local statusChips, tagBar, sortCarried, searchCarried,
    topToolbar, bottomPagination = libraryToolbar({
    csrf_token = data.csrf_token or "",
    Q = q, PluginID = pluginID, Sort = data.Sort,
    Status = data.Status, Tag = data.Tag, View = data.View,
    CategoryCounts = data.CategoryCounts,
})
```

Both fields therefore fell back to `1`, and `internal/templates/partials/pagination.lua:58` does `if not p or not p.Total or p.Total <= 1 then return "" end` — the partial returned an empty string, silently deleting the whole control.

## Fix

`fb7e396` — pass `Page = page, TotalPages = totalPages` at the call site.

## Why the golden test could not catch it

`TestGoldenTemplateRenders` builds its data map by hand and always supplies complete pagination fields, so a *caller* that omits them is invisible to it. This is the general lesson: golden tests validate the template against data, not that the view assembles that data correctly. A missing-wire bug between view and partial is only visible from a rendered HTTP response.

## Related defect found at the same time

`internal/httpserver/views_library.go` clamped `page` only from below (`if page < 1 { page = 1 }`). `?page=999` produced `start == end == total`, rendering an empty grid with no explanation. Fixed in the same commit by computing `totalPages` up front and pinning `page` to it.

## Verification

Rendered home page after fix contains 2× `aria-label="Next page"`, 2× `aria-label="Previous page"`, and links `page=2,3,4,5,25`. Before the fix, the Next/Prev count was 0.

# Related Concepts
- [Library view URL is the source of truth (no localStorage filter persistence)](library-view-url-is-source-of-truth.md): Both are library-grid UI behaviour driven by the request URL: this finding records why the page controls disappeared, while that concept records the rule that the URL (not localStorage) is the source of truth for which filters and page the grid renders.
