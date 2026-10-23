# goisekai-qt6

C++ Qt Quick (QML) desktop client for a running goIsekai server. Separate from
`cmd/goisekai-qml` (miqt/Go) on purpose: this one uses the real Qt runtime, so
`qmldir` and the `Theme` singleton resolve natively.

## Why this exists

The miqt build unpacked QML to a temp dir and loaded it through
`QQmlApplicationEngine::load()`; nested directories then had no `qmldir`, so
`Theme` was never registered and every `Theme.*` reference failed with
`ReferenceError: Theme is not defined`. The QML itself was fine.

Here the QML is compiled into a QML module (`qt_add_qml_module`) with
`qt_policy(SET QTP0004 NEW)` + `QT_QML_SINGLETON_TYPE` on `Theme.qml`, which
generates the per-directory `qmldir` and the singleton registration.

## Build

```sh
cmake -S . -B build -G Ninja -DCMAKE_BUILD_TYPE=Release
ninja -C build
```

Needs Qt 6.5+ (`qt6-quick`), cmake, ninja, g++.

## Run

```sh
./build/goisekai-qt6 --server http://127.0.0.1:8080
```

The server must be running; the client is a thin HTTP reader over
`/api/library`, `/api/library/facets`, and `/api/library/detail/<pluginID>/<mangaID>`.

## Scope

- `qml/Main.qml` — window shell: tab bar, toolbar (search/sort/status/refresh),
  grid, pagination. Owns all filter state.
- `qml/MangaCard.qml` — one library card with status badge, source overlay and
  progress bar.
- `qml/MangaDetailPage.qml` — self-contained manga detail page (cover, info,
  description, status, genres, chapter list with read/unread indicators).
  Clicking a library card opens this page in-process; no browser needed.
- `qml/Theme.qml` — palette singleton, matches the web UI.

The library page has full content; the detail page is in-process (no external
browser). Every other tab (search, updates, history, plugins, settings, logs,
about) renders a placeholder.
