# UI Polish Review

Purpose: consolidate the UI/UX polish state of the goIsekai reader into one actionable list for follow-up fixes.
Scope: all Jet templates (`internal/templates/views/*.jet`, `layouts/base.jet`, `partials/nav.jet`) + `cmd/goisekai/frontend/lib/reader.js` + generated `frontend/lib/tailwind.css`.

Legend: **P0** = broken/invisible, **P1** = noticeable improvement, **P2** = nice-to-have.

---

## P0 — Broken / Invisible

### G1 · Reader bottom progress bar too thin
- **Problem:** the read-progress line at the bottom of the reader is `h-1.5` (6px). When the nav bar auto-hides it is barely visible against dark manga pages.
- **Where:** `internal/templates/views/reader.jet:64` (`#progress-line`)
- **Fix:** increase height (`h-2` / 8px) and add a subtle glow/outline so it reads on any page color. Keep `opacity-0` → shown-on-hide behavior.

### G2 · Library + detail pagination do not render (backend/template mismatch)
- **Problem:** pagination data was computed in `views.go` but the templates had no controls (library) or referenced keys the handler never sent (detail `Ch*`). Verified broken — pagination was invisible.
- **Where:** `internal/httpserver/views.go` (`viewLibrary`, `viewMangaDetail`) + `views/library.jet`, `views/detail.jet`
- **Fix (done):** `viewMangaDetail` now slices chapters (page size 50) and passes `ChCurrentPage`/`ChTotalPages`/`ChHasNext`/`ChHasPrev`; `library.jet` gained Prev/Next + page indicator; detail.jet pagination above and below the list. Verify library renders at >24 titles and detail at >50 chapters.

### G3 · Search results lack full-title hover
- **Problem:** long manga titles truncate with no tooltip.
- **Where:** `views/search.jet`
- **Fix (done):** added `title="{{r.Title}}"` to the title element. Confirm on a long-title result.

---

## P1 — Noticeable Improvements

### D1 · Chapter action buttons are raw emoji
- **Problem:** ✅/🔄/📚/⬇ cbz buttons have no chrome — hard to discover, poor touch targets.
- **Where:** `views/detail.jet:92-104`
- **Fix:** give them `size-7` ghost-icon buttons with `border` + `hover:bg-neutral-800` (partial — still raw on some).

### D2 · Detail cover image not aspect-ratio locked
- **Problem:** `views/detail.jet:10` uses `w-full rounded-lg object-cover` with no fixed ratio — tall covers blow the layout.
- **Fix:** constrain to `aspect-[2/3]`.

### R1 · Reader page slider thumb barely visible
- **Problem:** `#page-slider` (`reader.jet:58`) is `h-1` — thin range input, easy to miss.
- **Fix:** `h-2` + larger accent thumb.

### R2 · Reader top bar wraps awkwardly on narrow screens
- **Problem:** many buttons (Back, ⌨, title, page counter, Fit/Zoom/Dir) crowd the top bar on mobile.
- **Fix:** prioritize page counter + title; move zoom controls to bottom bar on `sm` breakpoint.

### H1 · History "Xm ago" timestamps update only on load
- **Problem:** relative times don't refresh while the page is open.
- **Where:** `views/history.jet:33-42`
- **Fix:** `setInterval` refresh every 60s (cheap).

---

## P2 — Nice-to-have

### S1 · Search pagination naked links at corners
- **Where:** `views/search.jet:43-53`
- **Fix:** match the library/detail centered `flex items-center justify-center gap-4` pattern for consistency.

### D3 · Detail genre pills uncapped
- **Where:** `views/detail.jet:22-24`
- **Fix:** cap at ~6 pills + "+N more".

### D4 · Zero-value dates show "Jan 1, 0001"
- **Where:** `views/detail.jet:89` (`formatDate`)
- **Fix:** hide the date when the timestamp is zero.

### G4 · `tailwind.config.js` palette is dead
- **Problem:** custom tokens (accent, surface) defined but unused in generated CSS.
- **Fix:** remove the config or actually wire tokens into classes.

### G5 · No `prefers-reduced-motion` handling
- **Problem:** transitions (hover lifts, bar slide) ignore the reduced-motion preference.
- **Fix:** add a `motion-reduce:transition-none` utility to interactive elements.

### R3 · No pinch-zoom / touch gesture on the canvas
- **Where:** `cmd/goisekai/frontend/lib/reader.js`
- **Fix:** pointer-events pinch handling (or leave zoom buttons as primary).

### PL1 · Plugins file input label styling
- **Where:** `views/plugins.jet`
- **Fix:** hidden input + styled "Choose .wasm…" label (partially done).

### LG1 · Log lines not color-coded
- **Where:** `views/logs.jet`
- **Fix:** color WARN amber, ERROR red, DEBUG dim.

---

## Recommended Next Batch
1. G1 (progress bar height) — user-reported, highest visibility.
2. G3 verify + D2 (detail cover ratio) — quick wins.
3. D1 (chapter button chrome) — consistency.
4. R1 (slider thumb) + R2 (top bar responsive).

> Last updated: 2026-09-04. Written by orchestrator after three designer attempts stalled.
