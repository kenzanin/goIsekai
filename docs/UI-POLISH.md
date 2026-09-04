# goIsekai UI Polish

> **Scope**: Web UI review of all server-rendered views (library, search, detail,
> reader, history, settings, plugins, logs) plus the global shell.
> **Method**: Full read of every template + generated CSS + reader.js, verified
> against the live app at `http://127.0.0.1:8080` with rendered screenshots at
> desktop width (and a constrained-width simulation).
> **Stack reality check**: HTMX ships and the nav uses `hx-boost`, but Alpine.js
> is **not loaded anywhere** (no script tag in any layout) — templates are pure
> Tailwind classes plus tiny inline `<script>` blocks. Keep that in mind before
> proposing Alpine-based fixes; they would require adding the dependency first.

**Priorities**: P0 = broken or actively ugly · P1 = noticeably rough · P2 = nice-to-have polish.

---

## 0. Current State Assessment

### What works well

- **Consistent dark shell.** `bg-neutral-950` page, `bg-neutral-900` cards, `neutral-800` borders — used without drift on every page. The base look is clean and coherent.
- **Cover-first grids.** Library and search cards lead with real `aspect-ratio` (learned from the source when available, `2/3` fallback otherwise), `object-cover`, lazy loading, and `line-clamp-2` titles. The skeleton — image top, padded meta below — reads well at all sizes.
- **Reader interaction model is genuinely good.** Click zones (sides = pages, top/bottom strips = chapters, center = toggle bars), auto-hiding bars with slide transitions, a 3px indigo progress line that appears when bars hide, canvas rendering with DPR awareness, cursor-anchored wheel zoom, read-ahead prefetch with next-chapter spill, LTR/RTL toggle, and full keyboard nav. The mechanics rival commercial readers.
- **Form controls are consistently styled.** Every input/select uses the same `bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm` recipe.
- **Good empty states.** Library, history, and logs all have centered, friendly "nothing here yet" states with a follow-up link where relevant.
- **Semantic details done right:** real `<label for>` bindings, `title` tooltips on icon-only buttons, `loading="lazy"` on every grid image.

### What holds it back

1. **Dead design tokens.** `tailwind.config.js` defines a full custom palette (`accent #7c5cfc`, `surface.*`, `muted`, `success`, `error`, `border`, `shadow-glow`, `shadow-card-hover`, `rounded-card`) — **none of which appear in the generated CSS**. Every template hardcodes `indigo-*` and `neutral-*` classes instead. The config is aspirational; the UI never got migrated.
2. **Zero motion outside the reader.** Buttons, cards, and nav links have `hover:` color changes but **no `transition` property** (verified in the live DOM: `transition-duration: 0s`). Every state change snaps instantly, which reads as "unpolished" even when the colors are right.
3. **Typography is default Inter with no hierarchy above `text-2xl`.** Page titles (`text-xl font-semibold`) look like slightly-big body text. No display font, no tracking, no letterform personality. The whole app is one mid-gray voice.
4. **Focus states are missing everywhere.** No `focus-visible:` styles on any interactive element — keyboard navigation is invisible (accessibility gap, and it shows on every form page).
5. **Button palette is semantically inverted in places.** Destructive actions (`↺ Reset all read status`, `🗑 Clear cached images`, `Clear all cached images`) use `bg-neutral-700 hover:bg-red-600` — calm gray until the exact moment you're hovering *and about to commit*, which is backwards. Meanwhile `✓ In Library` (a state, not an action) is a big saturated emerald block competing with `Continue Reading` (the actual primary action).
6. **Page titles are small and lonely**, and each page invents its own header layout: library = flex row with button, search = plain h1, settings = h1 + mono path line, history = plain h1. No shared header pattern.
7. **Crud on full-page forms.** Every action (mark read, reset, toggle library) is a plain `POST` that reloads the page. `hx-boost` is on the nav but nothing else. That's a backend concern, but the *felt* result — full flash on every click of the ✅/🔄/📚 buttons — is a UI problem: rapid chapter triage flickers the entire page.

The bones are good. The gap between "functional dark theme" and "feels designed" is mostly: motion, focus states, the dead token palette, and type hierarchy.

---

## 1. Global Shell & Navigation

### G1 · P0 — No focus-visible styles anywhere
- **File**: `internal/templates/layouts/base.jet` (CSS custom props or a `<style>` block), or better: `cmd/goisekai/frontend/lib/tailwind.css` source + rebuild.
- **Wrong**: Keyboard users can't see where they are. Verified live: `outline-style: null`, no focus classes on any element.
- **Should be**: A consistent `focus-visible` ring. Add to base (or a utilities layer):
  ```html
  <style>
    :where(a, button, input, select, textarea, summary):focus-visible {
      outline: 2px solid theme-indigo (#6366f1);
      outline-offset: 2px;
      border-radius: 4px;
    }
  </style>
  ```
  One rule fixes the entire app.

### G2 · P1 — No transitions on interactive elements
- **Files**: every template — the five recurring button recipes, nav links (`internal/templates/partials/nav.jet:2-7`), cards (`library.jet:20`, `search.jet:27`), chapter rows (`detail.jet:66`), history rows (`history.jet:14`), plugin rows (`plugins.jet:17`).
- **Wrong**: hover states snap (`transition-duration: 0s` measured live).
- **Should be**: `transition-colors duration-150` on every hover-styled element; `transition-[transform,opacity]` already exists on the reader bars — replicate that discipline globally. Cheapest global fix: one CSS rule `a, button, input, select, textarea { transition: color .15s, background-color .15s, border-color .15s; }` in base.jet.

### G3 · P1 — Dead `tailwind.config.js` palette (pick a lane)
- **Files**: `tailwind.config.js:6-21`, `cmd/goisekai/frontend/lib/tailwind.css` (generated), all templates.
- **Wrong**: Custom tokens (`accent`, `surface`, `shadow-glow`, `card-hover`, `rounded-card`) are configured but **zero of them appear in the built CSS** — the build ran without the config, or predates it. Templates use raw `indigo-600`/`neutral-900`.
- **Should be**: Either (a) delete the unused config block and standardize on the Tailwind classes actually in use (honest, zero work), or (b) migrate: rebuild CSS with the config, then find/replace `indigo-600→accent`, `indigo-500→accent-hover`, `neutral-900→surface`, `neutral-950→bg`, `rounded-lg→rounded-card`, add `shadow-card-hover` to grids. Option (b) gives the app its own identity vs. stock Tailwind indigo; option (a) removes a landmine. Either way, don't leave the config half-wired.

### G4 · P1 — No typographic identity
- **File**: `internal/templates/layouts/base.jet` (add font links), `tailwind.config.js:15`.
- **Wrong**: `font-family: Inter, system-ui` (tailwind.css:157) with Inter not even loaded — every OS renders its own fallback. Titles max out at `text-2xl font-semibold`.
- **Should be**: Load one characterful display face for headings + keep system sans for body. Zero-dependency option: self-host a variable font in `cmd/goisekai/frontend/lib/` (e.g. "Bricolage Grotesque" or "Space Grotesk" for headings — fits the isekai/manga vibe without being cartoonish) and set `h1, h2 { font-family: …; letter-spacing: -0.02em; }`. Two `<link>` lines + one config change.

### G5 · P1 — Page titles lack presence; no shared header pattern
- **Files**: `library.jet:4-9`, `search.jet:4`, `history.jet:4`, `settings.jet:4-5`, `plugins.jet:4-10`, `logs.jet:4-5`.
- **Wrong**: Each page hand-rolls a header. Titles are `text-xl` — visually equal to card text nearby. Settings shows a raw file path (`{{.Path}}`) as the subtitle with no context.
- **Should be**: A shared header pattern in `base.jet` — a `{{block header()}}{{end}}` that pages fill, rendering `text-2xl font-semibold tracking-tight` titles with consistent bottom margin. For settings, replace the raw path with `Config file: <code class="...">path</code>` so it reads as intentional.

### G6 · P2 — Nav has no brand and no bottom edge on active state
- **File**: `internal/templates/partials/nav.jet`.
- **Wrong**: Brand-less tab row; active state is just a gray pill (`bg-neutral-800`), so "where am I" relies on squinting. Nav wraps to two lines on narrow screens (verified in the constrained-width screenshot) which is fine, but the pills look stranded.
- **Should be**: Add a small wordmark (the app has a favicon already — `cmd/goisekai/frontend/favicon.svg`) at the left; give the active link a 2px bottom accent border or an indigo tint (`bg-indigo-500/15 text-indigo-300`) instead of the same gray as hover. Consider `overflow-x-auto whitespace-nowrap` so mobile scroll is horizontal and clean instead of wrapping.

### G7 · P2 — `main` lacks a min-height; short pages leave a huge dead void
- **File**: `internal/templates/layouts/base.jet:12`.
- **Wrong**: `max-w-4xl` is narrow (verified: search results, settings feel cramped at 1836px wide) and short pages (empty search) show endless black.
- **Should be**: `min-h-[calc(100vh-8rem)]` on `main`, and consider `max-w-5xl` (or `6xl` for library/search grids specifically) — 4xl forces covers into a 6-column squeeze that isn't needed on desktop.

### G8 · P2 — No loading feedback for slow actions
- **Files**: `library.jet:6-8` (Update), all POST buttons.
- **Wrong**: `⟳ Update` triggers a sync that can take seconds — during which the button looks idle and users re-click.
- **Should be**: A tiny inline snippet (matches the existing inline-script pattern, no Alpine needed): on submit, disable the button and swap its label to a spinner (`⟳` with `animate-spin` on an inline SVG or the character itself). 5 lines in base.jet applied via `document.querySelectorAll('form button[type=submit]')`.

### G9 · P2 — Toast/feedback for completed actions is absent
- Wrong: "Marked as read", "Settings saved", "Plugin activated" all communicate via page reload only.
- Should be: Minimal — an HTMX `hx-swap-oob` toast, or a `:target`-based banner, or (simplest, matches existing code style) a 3-line JS snippet that reads a `?msg=` param set by redirect handlers and shows a dismissible bottom-left toast for 3s. P2 because the app works without it, but it's the single biggest "feels finished" upgrade.

---

## 2. Library (`internal/templates/views/library.jet`)

### L1 · P1 — Progress + plugin + New + Status badges stack into a cramped blob
- **File**: `library.jet:33-43`.
- **Wrong** (verified in screenshot): Under each title, up to four tiny elements crowd one line — `0/31/7` count, a gray `kaliscan` pill, a red `New` pill, then a *second row* `Ongoing` pill. On 6-column mobile the count and pills truncate/overlap. Two different pill styles (`rounded` neutral-800 vs `rounded-full` neutral-700/50) sit side by side.
- **Should be**: One meta line: `<count> · <plugin>`, with `New` as the only pill (it's the actionable signal). Move `Status` (Ongoing/Completed) to a *corner ribbon or small overlay chip on the cover itself* (`absolute top-2 left-2 bg-black/60 backdrop-blur rounded-full px-2 py-0.5 text-[10px]`) — it's cover-metadata, not card-metadata. Unify pill style: always `rounded-full text-[11px] px-2 py-0.5`.

### L2 · P1 — Unread count has no visual weight
- **File**: `library.jet:34`.
- **Wrong**: `12/31` in `text-neutral-400` is the most important signal on the card (did chapters drop?) and it's gray on gray, identical weight to the plugin name next to it.
- **Should be**: Make unread portion explicit: `<span class="text-indigo-400 font-medium">19 unread</span> <span class="text-neutral-500">of 31</span>` (or a thin 2px progress bar under the title: indigo fill = read fraction — cheap, glanceable, and matches the reader's progress line motif).

### L3 · P2 — No hover lift on cards
- **File**: `library.jet:20`.
- **Wrong**: Hover = 1px indigo ring only. Fine, but flat.
- **Should be**: `hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40` alongside the ring (needs G2 transitions). 2 classes, big feel upgrade. Same for `search.jet:27`.

### L4 · P2 — `[New]` badge could pop harder
- **File**: `library.jet:37`.
- **Wrong**: `bg-red-500/20 text-red-400` is tasteful but easy to miss in a grid scan.
- **Should be**: `bg-red-500 text-white` solid (it's the only solid-accent element on cards — appropriate, since it demands action), or keep the tint and add `animate-pulse`... no, pulse is annoying; solid + slight `shadow-red-500/30` glow is better. (If migrating to config palette: `bg-error text-white`.)

### L5 · P2 — Empty state is text-only
- **File**: `library.jet:12-15`.
- **Should be**: A large muted glyph (📚 or inline SVG) above the text, and a proper bordered CTA button for "Search manga" instead of a bare link. 3 lines.

---

## 3. Search (`internal/templates/views/search.jet`)

### S1 · P1 — Search form is not aligned and looks unfinished
- **File**: `search.jet:6-22` (verified in screenshot).
- **Wrong**: Plugin select and query input bottom-align, but the button floats right of a very wide input with 200px+ of dead space; labels sit *above* the two fields but the button has no label-row so the trio reads lopsided. The whole form stretches `max-w-4xl` making a 2-result search feel empty.
- **Should be**: `flex gap-2 items-center` one-liner: `[select][input flex-1 max-w-md][button]` — labels become placeholders or sr-only (or keep labels but constrain the form with `max-w-xl mx-auto` on mobile). A big centered hero-style search bar (`text-lg py-3` input, `Search` button beside) would also give this page an identity it currently lacks.

### S2 · P1 — No "searching" state, and Enter-on-empty quirks
- **File**: `search.jet:6-22`.
- **Wrong**: Submitting navigates; until the server responds you stare at the old page. For remote plugin queries (mangadex latency, challenge sites) that's seconds of nothing.
- **Should be**: Same G8 submit-spinner fix. Optionally `hx-boost="true"` on the form so HTMX handles it with a subtle opacity fade on `main` (`htmx.config.defaultSwapStyle` fallback is fine).

### S3 · P1 — Pagination is two naked links at extreme corners
- **File**: `search.jet:43-56` (verified: `← Prev` far left, `Next →` far right, tiny indigo text).
- **Wrong**: No page indicator ("Page 2"), no button affordance, and on mobile the corners are thumb-unfriendly.
- **Should be**: Centered `[← Prev]  Page 2  [Next →]` as bordered buttons matching the reader's (`border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-sm`), with the current page between them in `text-neutral-400`. `HasNext` is already in the template context — show it.

### S4 · P2 — Result cards lack the meta line library cards have
- **File**: `search.jet:37-39`.
- **Wrong**: Title only — fine, but a year/status if the plugin returns one would help pick between the two "Solo Leveling" results.
- **Should be**: P2 — only if the plugin payload has it cheaply; otherwise leave.

### S5 · P2 — Challenge banner is easy to miss / dismiss-less
- **File**: `search.jet:62-64`.
- **Wrong**: The amber banner renders *below* results (and below pagination) — for a big result set it's entirely below the fold; the user searches, gets garbage/mystery failures, never scrolls.
- **Should be**: Move the banner to the **top** of the body block (line 4, before the h1) — it's page-critical status. Same for `detail.jet:4-6` (that one is already on top — good; keep them consistent). Add a dismiss ✕ if trivial.

---

## 4. Manga Detail (`internal/templates/views/detail.jet`)

### D1 · P0 — Chapter action buttons are raw emoji with no affordance
- **File**: `detail.jet:84-94`.
- **Wrong** (verified in full-page screenshot): Every chapter row ends with `✅ 🔄 📚 ⬇ cbz` — bare emoji glyphs render differently per OS (Windows/Android/Linux vary wildly, sometimes as tofu), they're tiny (`text-xs`), have no button chrome, and sit close enough to mis-click. `✅` for "mark read" is semantically a *checkmark result*, not an action verb. They look like status text, not controls.
- **Should be**: Consistent ghost-icon buttons: `size-7 inline-flex items-center justify-center rounded-md border border-transparent hover:border-neutral-700 hover:bg-neutral-800 text-sm` with SVG icons (inline SVG, 4 icons total — check, rotate-ccw, layers, download; stroke-based, `stroke-current`) or at minimum letter badges (`✓ ↺ ≡ ⬇`) in a `title`-tooltipped button shell. Same actions, same tooltips, massively better readability. The `⬇ cbz` one can keep its text label.

### D2 · P0 — Massive chapter lists make the page unusable (100+ rows, no virtualization)
- **File**: `detail.jet:63-98` (full-page screenshot: ~6200px tall page).
- **Wrong**: `The Great Cleric` renders 97+ rows; scrolling to chapter 1 from the top is a finger-marathon, and there's no sort control.
- **Should be**: Two-part fix, both cheap: (a) **sort toggle** (Newest ↔ Oldest) — a single link/button that reverse-iterates is hard in Jet without sorting in the handler; if handler changes are out of scope, an Alpine/JS `display: revert` toggle on DOM order works; (b) **collapsible ranges** — group by volume/tens ("Ch. 90–97", "Ch. 80–89" as `<details>`) which is pure template logic (`if c.ChapterNum/10 != prev…`). At minimum, add a sticky "↑ Top" affordance. This page is the app's core workspace and 100-row flat lists don't scale.

### D3 · P1 — Read-progress chip is noise-dense
- **File**: `detail.jet:75`.
- **Wrong**: `7/25 read · 6 cached` in a gray pill per row × 97 rows = a wall of tiny text (screenshot confirms). "Cached" is an implementation detail users rarely act on per-row.
- **Should be**: Drop to `7/25` with a mini progress bar (3px under the row's text span, indigo fill) — cached count can live in the row `title` tooltip. Halves the visual noise.

### D4 · P1 — Cover image has no fixed aspect ratio
- **File**: `detail.jet:10`.
- **Wrong**: `w-full rounded-lg object-cover` without `aspect-[2/3]` — before the image loads (or if the plugin returns a wide banner), the md:col-span-1 collapses/jumps. Every other view learned the ratio lesson; this one didn't.
- **Should be**: Wrap in `aspect-[2/3] overflow-hidden rounded-lg bg-neutral-800` container (image inside `w-full h-full object-cover`), or reuse the ratio-map pattern from library if `Ratios` is available to this handler. Also add `loading="eager"`/`fetchpriority="high"` — it's the LCP element.

### D5 · P1 — Buttons row: state vs action hierarchy inverted
- **File**: `detail.jet:32-41`.
- **Wrong**: `✓ In Library` (state) is solid emerald and `▶ Continue Reading` (action) is solid indigo — two saturated primaries compete; the eye doesn't know the page's job.
- **Should be**: `Continue Reading` is THE primary: keep solid indigo. `In Library` becomes an outlined/ghost state: `border border-emerald-600/50 text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20` when in-library (solid indigo-neutral only for the *Add* variant, which is a real action).

### D6 · P1 — Genre pill soup (verified: 18 pills on Solo Leveling)
- **File**: `detail.jet:22-24`.
- **Wrong**: mangadex returns 15+ genres; they wrap into a full second column of purple chips, shoving the description down.
- **Should be**: Cap at ~8 with a `+N more` `<details>`/click-to-expand, or reduce pill weight: `text-[11px] text-neutral-400 bg-neutral-800/60` (no purple — genres are metadata, not links).

### D7 · P1 — Bulk actions bar has unclear grouping & the destructive is level with the rest
- **File**: `detail.jet:49-61`.
- **Wrong**: Three buttons in a row: indigo "Mark selected as read", gray "↺ Reset all read status", gray→red-hover "🗑 Clear cached images". The destructive-hover trick (G5 in assessment) means *reaching for* the button turns it red — the scariest moment is when it looks most armed. "Mark selected" is enabled even with zero checkboxes ticked (no-ops with a full reload).
- **Should be**: Destructive pair styled as quiet-red *from rest*: `bg-transparent border border-red-900/60 text-red-400 hover:bg-red-950/40`; require `onsubmit confirm()` (the cache button already does — add to Reset all too). Disable "Mark selected" until a checkbox is checked (one-line JS in the existing inline-script style, or `:has()` CSS + form validation).

### D8 · P2 — Chapter rows: date format is noise ("Jan 1, 0001")
- **File**: `detail.jet:80`.
- **Wrong**: Zero-value timestamps render as literal "Jan 1, 0001" (visible in screenshot) on many chapters — reads as a bug even though it's honest data.
- **Should be**: Template-side: `{{if c.ReleasedAt.Year > 1}}…{{end}}` — hide the date when unset. One conditional.

### D9 · P2 — The "Continue Reading" label truncates the manga's job
- **File**: `detail.jet:38`.
- **Wrong**: `▶ Continue Reading: Ch. 64 · p. 18` is long and mixes meta into the button; on mobile it wraps awkwardly next to the In-Library button.
- **Should be**: Button text `▶ Continue · Ch. 64 p. 18` or `Continue Ch. 64`, with full info as a `title`. Shorter, cleaner, same info on hover.

---

## 5. Reader (`internal/templates/views/reader.jet`)

### R1 · P1 — Top bar is a jumble at narrow widths
- **File**: `reader.jet:6-23`.
- **Wrong**: Back + title + counter + centered chapter title + 4 controls share one row with `max-w-[25%]` truncation on the manga title; at phone width the chapter title (`flex-1 justify-center`) gets ~10% of the row and truncates to nothing while zoom buttons keep fixed width. Verified: bar content overflows before it gets truly cramped.
- **Should be**: Two-tier mobile top bar: row 1 = back + manga title + counter; row 2 (only `<md`) = centered chapter title + controls, OR move fit/zoom/dir controls into a single overflow menu (⋮) on mobile. Simplest robust fix: `flex-wrap` + `order` classes, letting controls wrap under on narrow screens, bars grow taller (they auto-hide anyway).

### R2 · P1 — Buttons in bars have no active/pressed state & inconsistent paddings
- **File**: `reader.jet:18-21, 55-59`.
- **Wrong**: `px-2 py-1` on the small controls vs `px-3 py-1.5` on nav-style buttons; no `active:` scale/press feedback; the `LTR` toggle shows state as *text label only* — you can't tell it's a toggle until you click it.
- **Should be**: Normalize to one size (`px-3 py-1.5 text-xs`); add `active:scale-95 transition` for press feedback; for `btn-dir`, show state as an icon + label (`◀ LTR` / `RTL ▶`) or a subtle `bg-indigo-600/20 border-indigo-500/40` when RTL (manga-relevant state deserves visual weight).

### R3 · P1 — No pinch-zoom on touch, no tap-hint UI
- **Files**: `reader.jet:26-35` (zones), `cmd/goisekai/frontend/lib/reader.js:409-455`.
- **Wrong**: Wheel zoom + drag only. On phones (the primary manga device!) the click zones work, but two-finger pinch does nothing and there's no visual hint that zones exist.
- **Should be**: (a) `touch-action: none` + a 2-pointer gesture handler in reader.js (~20 lines: track two `touchmove` points, scale `zoomBy`); (b) first-run zone hint — on first reader visit (localStorage flag), fade in/out four corner arrows + center circle overlay for 1.5s ("tap edges to turn"). (b) is P2; (a) is a real gap for a manga reader.

### R4 · P2 — Progress line and slider don't echo each other
- **File**: `reader.jet:57, 63` + `reader.js:57-61`.
- **Wrong**: Bottom 3px progress line is great, but it's opacity-only visible when bars hide; when bars are visible the slider thumb position is the only progress cue and the line is redundant/hidden. Two different progress metaphors.
- **Should be**: Keep the line always visible (it's 3px — harmless over the bar) OR style the slider track to match (`bg-gradient` indigo fill portion). Minor, but unifying progress language is the kind of thing that makes UI feel *designed*.

### R5 · P2 — Error panel styling is bare
- **File**: `reader.jet:43-51`.
- **Wrong**: `Failed to load page` in small red text on a black card — works, but the icon-free, unpadded-title design looks like a debug overlay (and `showNotice()` reuses it for "Last page" toasts, where a red panel is wrong).
- **Should be**: Warning icon (⚠ inline SVG), `text-neutral-200` message with `text-red-400` only for true errors, and a distinct 2s auto-toast style for notices (same panel, `text-neutral-300 border border-neutral-700`). ~6 lines, better trust.

### R6 · P2 — Bars use `bg-black/90` — flat against the app's neutral palette
- **File**: `reader.jet:6, 54`.
- **Should be**: `bg-neutral-950/85 backdrop-blur-sm` to match the app shell (and blur looks premium behind manga art). Pure `black/90` reads slightly blue-ish over colorful pages.

### R7 · P2 — Reader page `<title>` stays "goIsekai"
- **File**: `internal/templates/layouts/blank.jet:6` (title hardcoded), `reader.jet` (no override).
- **Wrong**: Browser tab / PiP history shows "goIsekai" for every chapter; other views too (`base.jet:6`). History page becomes unusable for back-nav.
- **Should be**: Set `<title>{{.Manga.Title}} — Ch. {{.CurrentChapter.ChapterNum}} · goIsekai</title>` in reader; `{{.Manga.Title}} · goIsekai` in detail; page names elsewhere. Requires the title block in layouts — trivial. Also update `document.title` in `switchChapter()` (reader.js:356) so fetch-swap nav keeps it fresh.

---

## 6. History (`internal/templates/views/history.jet`)

### H1 · P1 — Date column is a floating orphan on mobile
- **File**: `history.jet:27`.
- **Wrong**: `shrink-0` date at the far right; on a 390px phone the row is thumb-cover 64px + flex text + date → the title truncates hard and the date sits detached at the edge (verified in constrained screenshot).
- **Should be**: Move the date under the title as part of the meta line on small screens: `flex-col sm:flex-row` layout, or render the date as `text-[11px] text-neutral-500 mt-0.5 sm:mt-0` inside the text block, and drop the right column entirely. Cleaner at every width.

### H2 · P1 — No relative time; raw absolute dates
- **File**: `history.jet:27` (server `formatDate`).
- **Wrong**: "Sep 4, 2026" — for a *history* view, recency is the whole point; "2h ago / Yesterday / 3d ago" is the natural language.
- **Should be**: Client-side one-liner (matches inline-script pattern): a 6-line `data-ts` → `timeago` formatter over `#history-dates`, or extend `formatDate` server-side. Also add `title` with the full date.

### H3 · P2 — No "continue where you left off" affordance
- **Wrong**: Rows link to manga detail; the most common intent from history is *resume reading*. Detail → Continue is two taps.
- **Should be**: If the history payload has chapter/page (it likely does server-side — it renders "continue" on detail), add a small `▶ Resume` ghost button on the right (desktop) replacing the date's spot. P2 — data-shape dependent, coordinate with backend.

### H4 · P2 — Grouping: same-manga sessions repeat as separate rows
- (Observed in screenshot: Solo Leveling appears twice — different plugins, but identical-title entries read as dupes.)
- **Should be**: P2; low cost to leave, but a `data-` attribute dedupe or day-group headers ("Today", "Earlier") would make it feel like a real history timeline.

---

## 7. Settings (`internal/templates/views/settings.jet`)

### ST1 · P1 — Form fields mix concerns; no grouping or help text
- **File**: `settings.jet:8-48` (verified in screenshot).
- **Wrong**: App identity (Title), server binding (Host/Port), HTTP client config (User Agent/Accept Language/Referer) and reader behavior (Read Ahead) are one flat column. "Read Ahead" is doubly confusing: it's stored in `localStorage` by the inline script (lines 57-65) but presented identically to server-persisted fields — a user can't know it never touches the ini file.
- **Should be**: Two `border border-neutral-800 rounded-lg p-4` fieldsets: **Server** (Title/Host/Port/Log Level) and **HTTP Client** (UA/Accept/Referer), with a separate **Reader** card for Read Ahead explicitly labeled "(stored in this browser)". Add muted help text (`text-xs text-neutral-500 mt-1`) under Host ("Bind address — use 127.0.0.1 for local only") and Referer (it matters for plugin scraping).

### ST2 · P1 — Save button gives no confirmation
- **File**: `settings.jet:47`.
- **Wrong**: POST → full reload → identical-looking page. Did it save? Unknown.
- **Should be**: Cheapest honest fix: server redirects with `?saved=1`; 4-line inline script shows a small green "Saved ✓" banner near the button, auto-fading. (G9 infrastructure covers this.)

### ST3 · P1 — Read Ahead max mismatch: UI allows 0–10, reader clamps 0–10, but label says "pages to prefetch" while slider default drifts
- **File**: `settings.jet:44-46` + `reader.js:29-32`.
- **Wrong**: `max="10" value="3"` hardcoded in the input, actual stored value injected by JS after load; if localStorage holds garbage ("abc"), input shows NaN silently. Number input also allows typing 999 before form-submit clamping.
- **Should be**: `min max step` are fine; add `oninput` clamp or a small range slider + numeric badge (slider matches the reader-page slider metaphor). Low risk, but present-tense jank.

### ST4 · P2 — Danger zone styling is inconsistent with detail page
- **File**: `settings.jet:50-56`.
- **Wrong**: Image Cache card uses `border border-neutral-800` + `bg-neutral-700 hover:bg-red-600` button — the armed-destructive-hover problem again (D7).
- **Should be**: Same quiet-red destructive recipe as D7; card can gain `border-red-900/40` to signal the zone. Keep confirm() (it's there — good).

### ST5 · P2 — Config path line looks like a leaked debug value
- **File**: `settings.jet:5`.
- **Wrong**: `goisekai.ini` in tiny mono under the title with no label (see G5).
- **Should be**: `Config: <code class="text-xs bg-neutral-800 rounded px-1.5 py-0.5">goisekai.ini</code>` — same data, reads intentional.

---

## 8. Plugins (`internal/templates/views/plugins.jet`)

### PL1 · P1 — Install form: raw file input is ugly and unlabeled
- **File**: `plugins.jet:7` (verified in screenshot: "Choose file  No file chosen" gray strip).
- **Wrong**: The browser-default file input clashes hard with the styled button next to it; no drag-drop affordance; no indication only `.wasm` works beyond the accept attr.
- **Should be**: Hide the native input (`sr-only`), render a styled pseudo-button `border-dashed border-neutral-700 rounded-md px-3 py-2 text-sm text-neutral-400 hover:border-indigo-500` labeled "Choose .wasm…", and reflect the chosen filename via 3-line JS (`input.addEventListener('change', …)`). Optionally `ondragover/ondrop` handlers on the pseudo-button (~8 lines total).

### PL2 · P1 — Row layout breaks on mobile
- **File**: `plugins.jet:17-42`.
- **Wrong**: `justify-between` with left cluster (icon+names) and right cluster (2 pills + toggle button) — at 390px the pills + button overflow/wrap awkwardly, and long plugin IDs (`font-mono truncate`) steal a second line.
- **Should be**: `flex-col sm:flex-row sm:justify-between` — name block on top, pills+button in a bottom row on mobile. The verify sub-card already stacks fine.

### PL3 · P2 — Active/Inactive pills + Activate/Deactivate buttons are redundant state
- **File**: `plugins.jet:31-40`.
- **Wrong**: The pill says "Active" and the button says "Deactivate" — same fact twice, two visual weights.
- **Should be**: Keep the pill (color-coded status at a glance) and reduce the button to a ghost: `border border-neutral-700 text-neutral-400 hover:text-neutral-200` — currently it's a full bordered button with default text color, heavier than the status it mutates. Or drop the pill entirely and let button color carry state. Either way, one source of truth.

### PL4 · P2 — Verify card (when present) lacks visual containment to its plugin
- **File**: `plugins.jet:43-72`.
- **Wrong**: The "Human Verification" panel renders as a *sibling* card below the plugin row with no connector — with multiple plugins needing verify, panels float ambiguous.
- **Should be**: Nest the verify panel inside the plugin card (indent with `mt-3 ml-3 border-l-2 border-amber-500/40 pl-4`), or give it a `bg-amber-500/5 border-amber-500/20` tint tying it to the amber "Not verified" pill.

---

## 9. Logs (`internal/templates/views/logs.jet`) — *bonus page, not in the brief but reviewed*

### LG1 · P2 — Log lines are uncolored walls of mono text
- **File**: `logs.jet:27`.
- **Wrong**: Every line is `text-neutral-300`; DEBUG/INFO/WARN/ERROR all identical (screenshot: solid gray wall). WARN/ERROR are the reason people open logs.
- **Should be**: 4-line inline script or template-side prefix check: `strings.HasPrefix(l, …)` → add `text-amber-400` for WARN, `text-red-400` for ERROR, `text-neutral-500` for DEBUG. Highest value-per-line fix in this file.

### LG2 · P2 — Toolbar selects are `bg-neutral-800` vs the app-wide `bg-neutral-900` recipe
- **File**: `logs.jet:8, 13`.
- **Should be**: Normalize to the shared input recipe (G2 of assessment). Two class swaps.

---

## 10. Cross-cutting / Implementation Notes

**Recurring primitives worth extracting first** (into a partial like `/partials/ui.jet`, or simply as consistent class strings):
1. `.btn-primary` → `bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium transition-colors duration-150 focus-visible:ring-2 ring-indigo-400`
2. `.btn-secondary` → same with `border border-neutral-700 hover:bg-neutral-800`
3. `.btn-danger-quiet` → `border border-red-900/60 text-red-400 hover:bg-red-950/40` (D7/ST4)
4. `.pill` → `rounded-full text-[11px] px-2 py-0.5 font-medium` (+ color variant)
5. `.card-hover` → `transition duration-150 hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40 hover:ring-1 hover:ring-indigo-500`

Jet templates don't do mixins, so this is either copy-paste consistency (current mode, works fine at this codebase size) or a `{{include}}` per button (probably overkill). The doc pins the *canonical strings* so drift stops.

**Build pipeline note**: `cmd/goisekai/frontend/lib/tailwind.css` is CLI-generated; any token migration (G3) or font addition (G4) must rerun the Tailwind CLI with `tailwind.config.js` — verify the build command includes `-c tailwind.config.js`, because the current output provably doesn't (grep: zero custom-token classes present).

**Suggested order of attack** (by felt-impact ÷ effort):
1. G1 focus rings + G2 transitions (one CSS block, whole app)
2. D1 chapter action buttons (worst P0)
3. S3 pagination, L1/L2 card meta, H1/H2 history dates
4. D5/D7 button hierarchy + destructive styling
5. G4 typography identity, G6 nav brand
6. D2 chapter list scale (needs a small handler tweak or JS sort)
7. Everything P2

---

*End of document — generated from template source review + live rendered verification on 2026-09-04.*
