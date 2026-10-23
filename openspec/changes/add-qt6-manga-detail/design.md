# Design

## Context

The Qt6 client (`goisekai-qt6/`) uses Qt Quick (QML) with native Qt 6.12. `Main.qml` is a shell (window + nav bar + tab switch); `LibraryPage.qml` is a self-contained page owning its own state, data load, and UI. The web UI detail page (`internal/templates/views/detail.lua`) renders: cover + info header, description, alt titles/covers, chapter list with per-row progress, continue-reading button. The server exposes `GET /api/manga/{pluginID}/{mangaID}` for detail data.

See proposal.md for motivation.

## Goals / Non-Goals

**Goals:**
- Add `MangaDetailPage.qml` following the `LibraryPage.qml` self-contained pattern
- Wire library card click → in-process detail page (no external browser)
- Render cover, title, description, status, genres, chapter list with progress
- Back-to-library navigation preserving filter state

**Non-Goals:**
- Reader stays external (web UI) — no in-app reader
- Alt covers grid and enrichment (related manga) — deferred to later iteration
- No server API changes needed

## Decisions

1. **Page follows LibraryPage pattern**: self-contained QML file owning its own state + data load, instantiated by Main.qml when a detail item is selected. This keeps Main.qml thin and every page independent — the template for future pages.
2. **Detail state lives in Main.qml**: `currentTab: "detail"` + `detailKey: "pluginID:mangaID"` set by LibraryPage's card click. Main.qml passes `detailKey` to `MangaDetailPage.qml`, which fetches and renders. Back button resets `currentTab: "library"`.
3. **API fetch via XMLHttpRequest**: same pattern as LibraryPage — no C++ bridge needed, keeps everything in QML.
4. **Chapters rendered in a Flickable + ColumnLayout**: chapter list is typically < 200 rows; no virtualization needed at this scale. Paginated via API params.

## Risks / Trade-offs

- [Detail fetch is slower than library load] → Show skeleton/loading state, not blank page
- [Filter state could reset on back] → Main.qml owns filter state (not LibraryPage), so it persists across tab switches
