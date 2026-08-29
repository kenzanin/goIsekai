# Reader Features Plan

## Overview
Improve the manga reader UX with5 features + 3 bonus UX improvements. All are frontend-only changes in `cmd/goisekai/frontend/` (Alpine.js + Tailwind CSS). No backend changes needed — uses existing Wails bindings.

---

## Feature 1: Image Rendering Mode Setting

**What:** Toggle between rendering modes for manga pages.
- **Smooth** (default): `image-rendering: auto` — bilinear interpolation, best for most manga
- **Sharp Pixels**: `image-rendering: pixelated` — nearest-neighbor, best for pixel-art manga
- **GPU Compositing**: `will-change: transform` + `transform: translateZ(0)` — forces GPU-backed layer, improves scroll performance for large images

**Where:** New Settings view (`#settings` route) + persists to `localStorage`.

**Implementation:**
1. Add `#settings` to router in app.js
2. Add Settings nav button (gear icon) in top bar
3. Settings page with: image rendering dropdown (smooth/sharp), GPU compositing toggle
4. Apply `image-rendering` CSS to `.reader-page img` via Alpine reactive class
5. Persist to `localStorage.setItem('gi_renderMode', ...)` / `gi_gpuCompositing`

---

## Feature 2: Reader View Mode (Fit Width / Fit Height)

**What:** How manga pages scale in the reader viewport.
- **Fit Width** (default): `width: 100%` — page fills viewport width, scroll vertically
- **Fit Height**: `max-height: 100vh; width: auto` — page fills viewport height, no vertical scroll
- **Original**: `width: auto; height: auto` — actual pixel size, may overflow

**Where:** Toggle buttons in reader controls bar (top-right, near existing controls).

**Implementation:**
1. Add `viewMode` state to reader store: `'fitWidth' | 'fitHeight' | 'original'`
2. Three icon buttons in reader controls: ↔ (fit width), ↕ (fit height), 1:1 (original)
3. Apply CSS class to reader page container: `.reader-fit-width`, `.reader-fit-height`, `.reader-original`
4. Persist to `localStorage.setItem('gi_viewMode', ...)`
5. CSS rules:
   - `.reader-fit-width img { width: 100%; height: auto; }`
   - `.reader-fit-height img { max-height: 100vh; width: auto; }`
   - `.reader-original img { width: auto; height: auto; }`

---

## Feature 3: Start / Continue Reading Button

**What:** On manga detail view, show a prominent button:
- **"Start Reading"** if no chapters have been read
- **"Continue Reading Ch.X"** if there's progress (last read chapter)

**Where:** Manga detail view, above the chapter list.

**Implementation:**
1. After loading manga details + chapters, check if any chapter has `isRead` or `lastPageRead > 0` from the bridge's progress data
2. Find the last-read chapter → suggest next chapter (or the last-read one to continue)
3. Show button: `@click="openChapter(nextChapterID)"`
4. If no progress: button opens first chapter in list
5. Style: `btn btn-accent btn-lg w-full` (purple accent, full width, prominent)

---

## Feature 4: Read-Ahead Prefetch

**What:** When reading chapter N, automatically prefetch pages for chapters N+1..N+K in the background. K is configurable (0-10, default 3).

**Where:** Settings view (K slider/dropdown) + reader view (prefetch controller).

**Implementation:**
1. Settings: add "Read-ahead chapters" number input (0-10, default 3), persist to `localStorage.setItem('gi_readAhead', ...)`
2. Reader: after loading current chapter's pages, spawn background prefetch:
   - Get next K chapter IDs from the chapter list
   - For each: call `bindings.getPageList(pluginID, chapterID)` to get page URLs
   - For each page URL: call `bindings.getImage(pluginID, url, headers)` → blob cache
3. Prefetch runs silently in background, no UI blocking
4. If user navigates to a prefetched chapter, pages load instantly from blob cache
5. Show subtle indicator: "Prefetching 2/3 chapters..." in reader footer (auto-hide when done)

---

## Feature 5: Reading Direction (LTR/RTL)

**What:** Manga reading direction affects page order and keyboard navigation.
- **LTR** (left-to-right): ← prev, → next (default for webtoons)
- **RTL** (right-to-left): → prev, ← next (traditional manga)

**Where:** Reader controls + Settings default.

**Implementation:**
1. Add `readingDirection` state: `'ltr' | 'rtl'`
2. Toggle button in reader controls: "LTR" / "RTL" label
3. Flip keyboard behavior: if RTL, `ArrowLeft` = next page, `ArrowRight` = prev page
4. Flip chapter navigation: "Next Chapter" becomes previous in reading order
5. Persist to `localStorage.setItem('gi_direction', ...)`
6. Default: LTR

---

## Bonus UX Improvements

### B1: Page Counter + Keyboard Shortcut
- Show "Page 3 / 24" in reader controls
- `Space` key = next page (common manga reader convention)
- `Home` = first page, `End` = last page

### B2: Reading Progress Bar
- Thin progress bar at top of reader showing position in chapter
- Updates as user scrolls/navigates
- Color: accent purple

### B3: Keyboard Shortcuts Legend
- `?` key opens a small overlay showing all keyboard shortcuts
- Helps discoverability

---

## Task Sequence (single worker, no conflicts)

1. **Settings view** — new route, render mode, read-ahead, direction defaults
2. **View mode toggle** — fit width/height/original in reader controls + CSS
3. **Continue Reading** — button on manga detail view
4. **Read-ahead prefetch** — background controller in reader
5. **Reading direction** — LTR/RTL toggle + keyboard flip
6. **Page counter + shortcuts** — page indicator, Space/Home/End
7. **Progress bar** — thin bar at top of reader
8. **Shortcuts legend** — `?` overlay

Estimated: ~400 lines of app.js changes, ~50 lines of CSS additions, minor index.html changes.

---

## Not Included (defer)
- WebGL canvas rendering: webkit2gtk doesn't expose GPU rendering toggle for `<img>`; CSS `image-rendering` covers the use case
- Double-page spread: complex layout change, defer to future
- Swipe gestures: desktop app, not needed
- Offline caching: would need backend changes
