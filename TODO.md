# TODO — goIsekai Gaps & Improvements

Items from honest project review (2026-09-03). Each item is a candidate for an OpenSpec change.

---

## 🔴 High Priority

### 1. httpserver Test Coverage (currently 6.8%)
All user-facing logic lives here — handlers, views, actions, API, sandbox, reader-data — but almost zero automated tests. Every bug so far (qrm alias trap, chapter order, status normalization) was caught by manual browser testing.

**Scope:**
- `api.go` — 11 endpoints, 0 handler tests (only middleware test exists)
- `views.go` — viewMangaDetail, viewSearch, viewHistory (render + data assembly)
- `actions.go` — all POST action handlers (toggle-library, mark-read, mark-read-range, reset-progress, export-cbz, clear-cache, sync, save-settings)
- `reader.go` — readerData assembly (chapter list fallback, read-ahead prefetch)
- `sandbox.go` — load/unload/reload/call endpoints

**Approach:** httptest.NewServer + chi router, mock bridge service interface, table-driven tests per handler.

**OpenSpec:** `httpserver-test-coverage` — planning complete, ready for `/opsx-apply`

---

### 2. Config File Persistence
Settings (CDP engine, API key, port, host, log level, user-agent, referer) are CLI-flag-only. Restart = lost. `goisekai.ini` struct exists in config.go but has no read/write logic.

**Scope:**
- Read `goisekai.ini` (or `config.yaml`) on startup if it exists
- CLI flags override file values (flag > file > default)
- Settings page "Save" writes config file
- Hot-reload for safe subset (log level, user-agent) without restart

**OpenSpec:** `config-file-persistence` — planning complete, ready for `/opsx-apply`

---

## 🟡 Medium Priority

### 3. Frontend Error Boundary
Reader.js (~360 lines vanilla JS) has no try-catch on critical paths. If `/api/reader-data` returns error or corrupt JSON, user sees blank canvas with no feedback. Search/detail pages use HTMX which has its own error handling, but the reader is pure fetch + canvas.

**Scope:**
- try-catch around all fetch calls in reader.js
- Error toast / overlay on reader canvas (network error, parse error, plugin error)
- Consistent error display across HTMX views (htmx:responseError event handler)

**OpenSpec:** `frontend-error-boundary` — planning complete, ready for `/opsx-apply`

---

### 4. Plugin Error Reporting in UI
Plugin failure → host logs error → user sees "No chapters yet" or empty page. No per-plugin error state, no retry button, inconsistent challenge detection feedback.

**Scope:**
- Per-plugin error state in search/detail views (spinner → error message with reason)
- Retry button on transient errors (network timeout, 502)
- Consistent ChallengeError handling across all views (some handle it, some don't)
- Plugin health indicator on plugins page (last successful call, error count)

**OpenSpec:** `plugin-error-reporting` — planning complete, ready for `/opsx-apply`

---

### 5. CDP Fallback Chain
`-cdpEngine obscura` is optional. Without it, challenge-blocked sites show a "paste cookies" banner. No automatic fallback: tls-client → detect 403/503+CF markers → try CDP → cookies back to jar → retry. This pattern is proven in Suwayomi/Mihon.

**Scope:**
- Detect challenge response (403/503 + CF markers) in bridge layer
- Automatic CDP fallback when engine is configured
- Cookie jar injection after CDP solves challenge
- Retry original request with new cookies
- Configurable: auto-fallback vs manual-only

**OpenSpec:** `cdp-fallback-chain` — planning complete, ready for `/opsx-apply`

---

### 6. Graceful Shutdown
Server is killed via `pkill`. No signal handler for SIGTERM/SIGINT to drain in-flight requests, close DB connections, flush logs, and stop plugin runtimes cleanly.

**Scope:**
- `signal.NotifyContext` on SIGTERM/SIGINT
- `http.Server.Shutdown(ctx)` with timeout
- DB close, plugin manager close, log flush
- PID file for clean stop (`goisekai stop`)

**OpenSpec:** `graceful-shutdown` — planning complete, ready for `/opsx-apply`

---

### 7. HTTP Access Logging
No request logging middleware. Cannot diagnose slow requests, error rates, or traffic patterns.

**Scope:**
- Middleware: method, path, status, latency, size
- Structured logging (slog) integration
- Skip health-check and static asset noise
- Optional request ID for tracing

**OpenSpec:** `http-access-logging` — planning complete, ready for `/opsx-apply`

---

### 8. Unified Plugin Build
`make build` at root does not rebuild WASM plugins. User must manually `cd examples/wasm/X && make build && cp dist/X.wasm ../../app_data/plugins/`. Easy to forget, leads to stale plugin binaries.

**Scope:**
- Root `make build-plugins` target that builds all WASM plugins
- `make all` = build-plugins + build host
- Optional: `make install-plugins` copies to app_data/plugins/
- CI-friendly: exit non-zero on any plugin build failure

**OpenSpec:** `unified-plugin-build` — planning complete, ready for `/opsx-apply`

---

## 🟢 Low Priority

### 9. Reader Keyboard Shortcuts
Reader has no keyboard navigation. Arrow keys for prev/next page, space for next page, escape to go back to detail — standard manga reader UX.

**OpenSpec:** `reader-keyboard-shortcuts` — planning complete, ready for `/opsx-apply`

---

### 10. Plugin Build Size Optimization
Binary + embedded assets + 4 WASM plugins = large. No compression or lazy-load for WASM files (each ~4-5MB).

**Scope:**
- `go:embed` with gzip for WASM files (decompress on load)
- Or: lazy-load WASM on first plugin call instead of startup
- Binary size audit (strip debug symbols, UPX)

**OpenSpec:** `lazy-load-plugins` — scan-only boot + load-on-first-call (planning complete, ready for `/opsx-apply`)

---

## Deferred to v1.0
- **Database migration versioning** — intentionally deferred until stable schema

---

## UI/UX Audit — Library & Plugins Pages (2026-09-10)

Reviewed `library.lua` and `plugins.lua` templates against live browser rendering. These pages share the same base layout but have several consistency gaps and individual issues.

---

### Cross-Page Consistency Issues

#### [Cross-page] Card border inconsistency
- **File:** internal/templates/views/library.lua (line 33) + plugins.lua (line 30)
- **Current state:** Library sidebar cards use `border-neutral-700`, plugin cards use `border-neutral-800`. Visually different border weight on the same page shell.
- **Suggested fix:** Standardize on one value. `border-neutral-800` is more subtle and matches the nav separator — use it everywhere.
- **Priority:** medium

#### [Cross-page] No page-level subtitle or item count
- **File:** internal/templates/views/library.lua (line 282) + plugins.lua (line 170)
- **Current state:** Both pages show just `<h1>` with the page name ("Library", "Plugins"). No subtitle, item count, or secondary info under the heading.
- **Suggested fix:** Add a small muted subtitle: e.g. `61 titles · 3 pages` and `10 plugins · 8 active`. Gives immediate context without scrolling.
- **Priority:** medium

#### [Cross-page] Header layout diverges
- **File:** internal/templates/views/library.lua (line 282) vs plugins.lua (line 170)
- **Current state:** Library uses `flex items-center justify-between` with 4 elements (title, search, view-toggle, update button). Plugins uses `flex items-center justify-between` with 2 elements (title, install form). The structural gap in the middle of the library header feels unbalanced — the search bar is `max-w-md` but gets pushed to center by `justify-between`.
- **Suggested fix:** For the library, consider `flex items-center gap-4 flex-wrap` instead of `justify-between`, grouping the search + toggle together, and the update button separately. Or add a `flex-1` wrapper around the search area so it fills the gap naturally.
- **Priority:** medium

---

### Library Page Issues

#### [Library] Pagination rendered twice (top AND bottom of grid)
- **File:** internal/templates/views/library.lua (lines 173–176 and 277–279)
- **Current state:** Pagination appears above the manga grid AND below it. On page 1, the top pagination is mostly redundant since the user can see the bottom one after scrolling. On long lists it adds noise.
- **Suggested fix:** Keep only the bottom pagination. If top pagination is desired for long lists, consider a "sticky" variant or show only when scrolled past the grid top. At minimum, hide the top instance on page 1.
- **Priority:** medium

#### [Library] Search input lacks a visual search icon
- **File:** internal/templates/views/library.lua (line 288)
- **Current state:** Plain text input with placeholder "Search library…". No magnifying glass icon. The adjacent "Search" button serves as the affordance, but the input itself doesn't signal its purpose at a glance.
- **Suggested fix:** Add an SVG magnifying glass icon inside the input (as a `pointer-events-none absolute` element) and give the input `pl-9` to make room. Consider removing the separate Search button entirely — pressing Enter in a search field is universal.
- **Priority:** low

#### [Library] View toggle buttons lack accessible state
- **File:** internal/templates/views/library.lua (lines 291–295)
- **Current state:** Grid/list toggle buttons use `id` and `data-*` attributes but no `aria-pressed` or `role="button"`. The active state is styled via JS adding a class (`bg-neutral-700` + indigo SVG color), which is fine visually but invisible to screen readers.
- **Suggested fix:** Add `role="group"` and `aria-label="View mode"` to the container. Add `aria-pressed="true"` to the active button and `aria-pressed="false"` to the inactive one. Update the JS `setActive` function to toggle `aria-pressed`.
- **Priority:** medium

#### [Library] Stats sidebar has no visual hierarchy
- **File:** internal/templates/views/library.lua (lines 28–43)
- **Current state:** All stat cards use identical `bg-neutral-900 border-neutral-700 rounded-lg px-4 py-3` styling. The total count ("61 titles") is the most important metric but looks the same as "fewest chapters: 2 ch · 2 titles".
- **Suggested fix:** Make the total count card larger or use a different accent (e.g. `border-indigo-500/30` or `bg-indigo-500/10`). Use `text-2xl font-bold` for the primary stat value vs `text-sm` for secondary stats. Consider grouping: primary stats (count, reading time) vs secondary (status breakdown, most/least).
- **Priority:** low

#### [Library] "Update" button has inconsistent spacing from header
- **File:** internal/templates/views/library.lua (line 297)
- **Current state:** The Update button's `⟳` emoji renders with varying size across browsers. On some it appears larger than the surrounding text, throwing off the header balance.
- **Suggested fix:** Replace `⟳` with an SVG icon (a refresh/cycle arrow) at `w-4 h-4` for consistent sizing. Use `inline-flex items-center gap-1.5` on the button to keep icon and text aligned.
- **Priority:** low

#### [Library] Empty state is minimal
- **File:** internal/templates/views/library.lua (lines 168–171)
- **Current state:** Empty library shows just "Your library is empty — search for manga first" with a link. No illustration, no encouragement, no context about what the app does.
- **Suggested fix:** Add a subtle SVG illustration (bookshelf or manga pages) above the text. Make the CTA more prominent — a full indigo button instead of an inline link. Consider adding "or install a plugin to browse sources" as a secondary action.
- **Priority:** low

---

### Plugins Page Issues

#### [Plugins] ⚠️ Cards are nested instead of siblings (cascading indent bug)
- **File:** internal/templates/views/plugins.lua (lines 14–167)
- **Current state:** All 10 plugin cards are rendered as nested `<div>` elements inside the first card, not as siblings. The `space-y-3` container has only 1 direct child (the 1Manga card), with 9 more `.bg-neutral-900` divs nested inside it. This creates a visible cascading indent — each card is ~17px further right and ~34px narrower than the previous one. Verified via DOM inspection: card positions go from left=166 to left=319 across 10 cards.
- **Suggested fix:** The bug is in the template loop. Each card's `cardHTML` must be appended to `pluginCards` and the card must be properly closed before the next iteration. Check that the `</div>` closing tag for each card is at the correct nesting level — currently the next card's opening `<div class="bg-neutral-900 ...">` is being appended INSIDE the previous card's DOM.
- **Priority:** high

#### [Plugins] No search or filter functionality
- **File:** internal/templates/views/plugins.lua
- **Current state:** With 10+ plugins, there's no way to search/filter by name, status (active/inactive), or loaded state. User must scroll the full list.
- **Suggested fix:** Add a search input similar to the library page. Even a simple `input[type=search]` with client-side filtering (hide non-matching cards) would help. Could also add status filter buttons: All | Active | Inactive | Deferred.
- **Priority:** medium

#### [Plugins] "Choose plugin…" upload label doesn't look like a button
- **File:** internal/templates/views/plugins.lua (line 173)
- **Current state:** The file upload trigger is styled as `border border-dashed border-neutral-700 rounded-md px-3 py-2 text-sm text-neutral-400 hover:border-indigo-500`. It looks like a decorative border, not an interactive control. The dashed border pattern is commonly associated with drag-and-drop zones, not click-to-browse.
- **Suggested fix:** Either make it look more like a button (solid border, background on hover, icon) or lean into the drop-zone metaphor with a cloud-upload icon and "or drag a file here" text. The current middle-ground is ambiguous.
- **Priority:** low

#### [Plugins] No confirmation before deactivating a plugin
- **File:** internal/templates/views/plugins.lua (lines 127–132)
- **Current state:** The "Deactivate" button is a plain form submit — clicking it immediately toggles the plugin with no confirmation dialog. Deactivating a plugin the user relies on would break their library browsing.
- **Suggested fix:** Add a JavaScript confirmation: `onclick="return confirm('Deactivate this plugin?')"`. Or use the app's existing `confirmModal` (from alpine-components.js) for a styled confirmation.
- **Priority:** medium

#### [Plugins] Human verification section is nested inside the card (structure issue)
- **File:** internal/templates/views/plugins.lua (lines 135–162)
- **Current state:** The human verification section opens with `<div class="bg-neutral-900 rounded-lg p-4 border border-neutral-800">` — the exact same classes as the outer plugin card. This creates a visually confusing nested-card-inside-card pattern. The indentation in the DOM confirms this creates a visual "card within card" effect.
- **Suggested fix:** Use different styling for the inner verification section: `bg-neutral-800/50 border border-neutral-700 rounded-md mt-3` to visually distinguish it as a subsection, not a peer card. Or use a `<details>` element with a summary toggle to keep it collapsed by default.
- **Priority:** medium

#### [Plugins] Plugin icon fallback shows only first character
- **File:** internal/templates/views/plugins.lua (lines 39–42)
- **Current state:** When no icon URL is provided, the fallback is `string.sub(name, 1, 1)` — just the first letter. For a plugin named "1Manga" this shows "1", which is not distinctive.
- **Suggested fix:** Use the same `getInitials()` helper the library uses (which shows first 2 chars). Or generate a deterministic color from the plugin name for the background, making each fallback visually distinct.
- **Priority:** low

#### [Plugins] Test button uses inline styles that override Tailwind classes
- **File:** internal/templates/views/plugins.lua (line 91) + alpine-components.js (lines 211–224)
- **Current state:** The `setBtnState` function in alpine-components.js sets `s.padding`, `s.borderRadius`, `s.border`, `s.fontSize`, `s.fontWeight`, `s.transition` directly via `element.style`. This hardcodes visual properties that bypass Tailwind's utility classes and the global `transition` styles from base.lua.
- **Suggested fix:** Use `classList.add/remove` with Tailwind classes instead: `btn.classList.add('bg-amber-500', 'text-white', 'border-amber-500')`. This keeps styling consistent with the rest of the app and respects the global transition rules.
- **Priority:** low

#### [Plugins] All plugins show "Active" and "Loaded" simultaneously — no visual way to distinguish from Deferred
- **File:** internal/templates/views/plugins.lua (lines 110–123)
- **Current state:** Every plugin in the list shows both an "Active" badge (green) and a "Loaded" badge (sky blue). The "Deferred" state (amber) never appears. When all items show the same badges, the badges lose their informational value.
- **Suggested fix:** If all plugins are always active+loaded in this build, consider collapsing the status into a single "Active" badge. If the states can vary, ensure the data model actually differentiates them. Currently the badges are noise.
- **Priority:** low
