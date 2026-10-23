# Proposal

## Why

The Qt6 desktop client (`goisekai-qt6/`) currently only has a library page. Users browsing the library must open the browser for every manga detail — cover, description, chapter list, progress, alt titles, and continue-reading. The web UI already has a full manga detail page (`internal/templates/views/detail.lua`); porting it to Qt6 gives desktop users an in-app reading flow without leaving the client.

## What Changes

- Add a `MangaDetailPage.qml` to the Qt6 client: a self-contained page (following the `LibraryPage.qml` template) that renders manga detail and chapter list.
- Wire the library grid's card click to open the detail page in-process instead of `Qt.openUrlExternally` to the web UI.
- Port the web UI detail page's features: cover + info header, description, alt titles/covers grid, chapter list with per-row progress + read/unread toggle, continue-reading button, back-to-library navigation.
- Add a back button in the detail page that returns to the library tab with filters intact.
- Add a nav-tab switch case so `Main.qml` renders `MangaDetailPage` when a detail item is selected.

## Capabilities

### New Capabilities

- `qt6-desktop-client`: Desktop Qt6 client behavior — navigation between pages, manga detail rendering, chapter progress display, and back-navigation. Owns the client's page-level requirements as more pages (search, history, plugins) are added.

### Modified Capabilities

- (none — no existing spec-level requirements change; this is additive to the Qt6 client which has no prior spec)

## Impact

- **Code**: new `goisekai-qt6/qml/MangaDetailPage.qml`; `goisekai-qt6/qml/Main.qml` (add detail tab case + wire card click); `goisekai-qt6/qml/LibraryPage.qml` (card click → detail instead of external URL); `goisekai-qt6/CMakeLists.txt` (register new QML file).
- **API**: uses existing `/api/manga/{pluginID}/{mangaID}` and `/api/manga/{pluginID}/{mangaID}/chapters` endpoints — no server changes needed.
- **No breaking changes**: library page behavior unchanged except card-click target; reader remains external (web UI) for now.
