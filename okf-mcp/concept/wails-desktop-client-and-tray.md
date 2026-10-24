---
type: Concept
title: Wails desktop client + tray wiring
description: Wails v2 webview wrapper around the running goisekai server (cmd/goisekai-wails), Qt6 superseded, tray client_wails config + status colour, WebKitGTK reader scroll / AVIF transcode fixes.
tags:
  - goisekai
  - wails
  - desktop
  - webkit
  - tray
  - avif
generated:
  by: process:reasonix
  at: "2026-10-24T00:00:00Z"
---

# Wails desktop client + tray wiring

Qt6 client (`goisekai-qt6/`) is **deliberately left broken** ("Biarkan Qt6
rusak") and is **gitignored** — its QML files (`LibraryPage.qml`,
`MangaDetailPage.qml`, ...) are NOT tracked; only build scaffolding is.
Don't try to "fix" it or commit those files. The desktop client is now
**Wails**, a thin webview wrapper that loads the running server's web UI.

## Layout

- `cmd/goisekai-wails/` is a **nested Go module**
  (`goisekai/cmd/goisekai-wails`, own `go.mod`/`go.sum`) so the heavy Wails
  deps never enter the server's `go.mod` (server stays CGO-free;
  `grep -c wails go.mod` = 0 at repo root). `main.go` is a ~40-line
  `wails.Run` with `WebViewURL` → `http://127.0.0.1:8080`; override with
  `-server` flag (same shape as the tray's `-server`). Frontend is
  `frontend/dist/index.html` — a launcher that redirects to the server.
- Build recipe `just build-wails`:
  `cd cmd/goisekai-wails && CGO_ENABLED=1 go build -tags "desktop production webkit2_41" -o ../../bin/goisekai-wails .`
  **Plain `go build` produces a stub binary that refuses to run** — Wails
  prints *"Wails applications will not build without the correct build
  tags."* The three tags are mandatory.
- Output `bin/goisekai-wails` (CGO ELF, ~5.6 MB).
- Wails v2.12.0 CLI at `~/go/bin/wails`.

## WebKitGTK gotchas (client is WebKitGTK, not Chromium — this drove most fixes)

1. **Tag must be `webkit2_41`**, not the default. System has
   `webkit2gtk-4.1` (no 4.0); `webkitgtk-6.0` (GTK4) is present but Wails
   v2 only supports webkit2gtk-4.x (GTK4 = Wails v3, not a fix here).
   Omit the tag → pkg-config fails on `webkit2gtk-4.0`.
2. **AVIF decode = scroll-wheel lag in the reader.** `goisekai.ini
   [app] image_format = avif` stores images as AVIF; WebKitGTK decodes AVIF
   on the main thread → choppy strip scroll (user guessed "less RAM"; it
   was the decoder — display/scroll handling is client-side, server just
   ships bytes, so the fix belongs in the display pipeline, not
   `.lua`/`.go` request logic). Fix: `internal/bridge/imgcodec_display.go`
   `ClientDisplayFormat(data)` transcodes AVIF/JXL → JPEG q85 **for
   display only** (disk cache stays AVIF), fail-open on error;
   `internal/httpserver/image.go` calls it before setting Content-Type,
   so `/image` now returns `Content-Type: image/jpeg` (magic `ffd8`).
   Test: `imgcodec_display_test.go`.
3. **Reader strip scroll**: `cmd/goisekai/frontend/lib/reader.js` —
   dropped the JS `scrollTop += e.deltaY` (forced main-thread scroll);
   wheel handler in strip mode does `if (stripMode) return` and lets
   WebKit scroll natively; zones get `pointer-events: none` in strip mode
   via `setZonesEnabled()` on enter/exit. A rAF throttle
   (`stripScrollPending`) remains on the JS paths. **Any `reader.js`
   edit needs `just br`** (brotli siblings are tested by
   `TestBrotliSiblingsMatchSource` / `TestFrontendJSParses`).
4. **Logs "loading lanes…" flash**: `internal/templates/views/logs.lua`
   now server-renders the worker lane cards from `data.WorkerLanes`
   (`workerLaneStates` in `internal/httpserver/api_workers.go`, shared
   with the `/api/workers` handler) instead of JS-only fetch.
5. **White-on-white select dropdowns**: WebKit renders native popups with
   system light colours — fixed in `internal/templates/layouts/base.lua`
   with `:root { color-scheme: dark }` + explicit `select option`
   colours.

## Tray wiring (`cmd/goisekai-tray/`)

- Config key renamed `client_qt6` → **`client_wails`** in `goisekai.ini
  [tray]` (`internal/config/config.go` field `TrayClientWails`;
  `config_set.go` accepts both old and new spellings; see
  `config_test.go`). Value in the local ini:
  `/home/kenzanin/tmp/goIsekai/bin/goisekai-wails`.
- Tray "open client" launches `clientBin` with `-server <url>`
  (`lifecycle.go`); default binary path `bin/goisekai-wails`.
- **Server status colour in the tray menu**: systray has no per-menu-item
  icon API, so the Status menu line carries a **🟢/🔴 emoji** (green =
  running, red = stop/error); the tray logo icon itself is unchanged
  (deliberate user steer: "do not change default icon for systemtray,
  just change the right click content").
- `cmd/goisekai-tray/icon.go` holds the embedded icon bytes.

## Library 5×5 grid + adaptive covers (web UI)

- Web grid locked to `grid grid-cols-5 gap-4`
  (`internal/templates/partials/library_cards.lua`, responsive column
  breakpoints removed) + sidebar always left: `flex flex-row gap-4` and
  `aside w-56 shrink-0` (`partials/library_sidebar.lua`) so un-maximizing
  the Wails window never reflows the grid. Covers stay
  `w-full aspect-[2/3]` → scale with the cell, count stays 25/page
  (`pageSize = 25` in `internal/httpserver/views_library.go`).
- Symptom "grid shows 1×25 big" = persisted **list view mode**
  (`localStorage['gsk:view-mode'] == 'list'` read by `syncViewMode` in
  `alpine-components.js` ~line 910), NOT a layout bug → reset with the
  grid button / `window.resetLibraryView()`.
- Qt6 `LibraryPage.qml` (untracked) had 6 columns / 24 per page; changed
  to `cellWidth: Math.max(120, Math.floor((grid.width - 48) / 5))`,
  `cellHeight: Math.round(cellWidth*1.426)+84`, `pageSize: 25`.

## Deploy gate reminders

- Any `cmd/goisekai/frontend/lib/*` edit → `just br` then
  `just build`, else the package tests fail and the served bundle is the
  stale one (assets served from disk, `Cache-Control: no-cache`).
- Restart: `pkill -x goisekai; sleep 2; (setsid nohup ./goisekai
  -logLevel debug > goisekai.log 2>&1 &); sleep 8` then
  `curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/` → 200.
