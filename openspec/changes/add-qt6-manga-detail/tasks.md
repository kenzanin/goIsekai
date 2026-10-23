# Tasks

## 1. Manga Detail Page

- [ ] 1.1 Create `goisekai-qt6/qml/MangaDetailPage.qml`: self-contained page with `detailKey` property, XMLHttpRequest fetch to `/api/manga/{pluginID}/{mangaID}`, loading/error/success states, cover + title + description + status + genres layout, chapter list with read/unread badges, back button emitting `backRequested` signal. Verify: `ninja -C goisekai-qt6/build` compiles without errors.
- [ ] 1.2 Register `qml/MangaDetailPage.qml` in `goisekai-qt6/CMakeLists.txt` QML_FILES. Verify: `cmake -S goisekai-qt6 -B goisekai-qt6/build && ninja -C goisekai-qt6/build` succeeds.

## 2. Wire Navigation

- [ ] 2.1 In `goisekai-qt6/qml/LibraryPage.qml`: change card click from `Qt.openUrlExternally(it.href)` to emit a new `detailRequested(string key)` signal carrying `pluginID:mangaID`. Verify: `ninja` compiles.
- [ ] 2.2 In `goisekai-qt6/qml/Main.qml`: add `currentTab: "detail"` case — instantiate `MangaDetailPage` with `detailKey` from LibraryPage's signal; back button resets to `currentTab: "library"`. Verify: `ninja` compiles.

## 3. Build & Verify

- [ ] 3.1 Full build + startup: `ninja -C goisekai-qt6/build` then `QT_QPA_PLATFORM=offscreen timeout 6 ./goisekai-qt6/build/goisekai-qt6` → exit 124, empty log. Verify: exit 124 + 0 log lines.
- [ ] 3.2 Commit all changes: `git add -A && git commit`. Verify: `git status --short` clean.
