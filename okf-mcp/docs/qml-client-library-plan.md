---
id: qml-client-library-plan
title: "QML Desktop Client — Library Page Parity Implementation Plan"
description: "QML desktop client library-page parity implementation plan"
type: reference
tags: [goisekai, qml, desktop, plan]
---

# QML Desktop Client — Library Page Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the Qt Quick client's library page to feature parity with `/view/library`, with sources split across focused files, plus `qmllint`/`qmlformat` gates in the justfile.

**Architecture:** The QML client stays a thin HTTP client over the existing server. Two additive backend changes (a `facets` endpoint and three extra fields on `/api/library`) give the client everything the web sidebar and sort dropdown need, with no new business logic — every value comes from an `AppService` method the web handler already calls. On the client, QML is embedded via `go:embed` and unpacked to a temp dir at startup so `import "components"` resolves across files while the binary stays a single 34MB file.

**Tech Stack:** Go 1.27 (`CGO_ENABLED=1` for this binary only), `github.com/mappu/miqt` v0.15.0 (`qt6`, `qt6/qml`), Qt 6.12 Quick Controls, chi router, go-jet DSL.

**Spec:** `docs/qml-client-library-plan.md` (this document). Decisions confirmed with the user: facets endpoint instead of HTML scraping; client-side filter/sort; 24 cards per page like the web.

## Global Constraints

- The server binary stays **CGO-free**. `cmd/goisekai-qml/main.go` must keep `//go:build cgo` as its first line — without it `CGO_ENABLED=0 go build ./...` fails with `build constraints exclude all Go files`.
- `just build`, `just test`, `just lint`, `just check` must stay green with `CGO_ENABLED=0` and must not compile Qt.
- No new DB queries: facets and the extra fields reuse `LibraryOverview`, `ListLibraryCategories`, `CountLibraryByPlugin`, `FindPotentialDuplicates`, `ListLibraryWithProgress`.
- `just test` must not compile the QML package. Tests for backend changes live in `internal/httpserver/`.
- Every QML file is formatted by `qmlformat` and passes `qmllint` before commit.
- Cover URLs always go through the server: `/image?pluginID=<plugin_id>&url=<urlencoded cover_url>`. Never bind an upstream CDN URL directly — it 403s without the host's Referer and bypasses the cache.

## Review Focus

- **Paths that escape URL escaping.** `source_manga_id` and `cover_url` can contain `/`, `?`, `&`, `+`. A detail link built by string concatenation rather than `url.PathEscape`/`url.QueryEscape` opens the wrong manga or truncates the query. Owned by Task 1 (Go) and Task 3 (`MangaCard.qml`).
- **An empty library versus a failed fetch.** Both produce zero cards. The app must say which, or the user cannot tell a dead server from an empty shelf. Owned by Task 3.
- **Filter state that silently resets.** Changing sort must not clear the search box; changing page must not clear the status chip. Owned by Task 6.
- **Cover aspect ratios.** `thumb_ratio` differs per plugin (0.703 vs 0.75). Hardcoding one crops covers from the other plugins. Owned by Task 2 (`Theme.qml` default) and Task 3.
- **A missing Qt image plugin.** `kimageformats` provides AVIF; without it every cover silently fails to decode and the grid shows placeholders. The "no cover" state must be distinguishable from "still loading". Owned by Task 3.

---

## File Structure

```
internal/httpserver/
  api_library.go          MODIFY  add updated_at, last_read_at, dup_group to items
  api_library_facets.go   CREATE  GET /api/library/facets
  api_library_facets_test.go CREATE
  api.go                  MODIFY  register the route

cmd/goisekai-qml/
  main.go                 MODIFY  embed qml/**, unpack to temp dir, wire engine
  api.go                  CREATE  fetchLibrary, fetchFacets (HTTP + JSON)
  model.go                CREATE  wire types -> QML-ready shapes (cover proxying)
  model_test.go           CREATE  pure-Go tests, no cgo needed in practice
  qml/
    qmldir                CREATE  singleton Theme
    Main.qml              CREATE  window shell, state ownership, wiring
    Theme.qml             CREATE  colours, spacing, ratios
    components/
      LibraryToolbar.qml  CREATE  search, sort, status chips, view toggle
      LibrarySidebar.qml  CREATE  stats cards, plugin list, tag list, duplicates
      MangaGrid.qml       CREATE  GridView, empty state, pagination host
      MangaCard.qml       CREATE  one cover card
      Pagination.qml      CREATE  prev/next + page numbers

justfile                  MODIFY  fmt-qml, lint-qml, build-qml stays
```

---

### Task 1: Facets endpoint and the fields the web sidebar already computes

**Files:**
- Create: `internal/httpserver/api_library_facets.go`
- Create: `internal/httpserver/api_library_facets_test.go`
- Modify: `internal/httpserver/api.go` (add one route line)
- Modify: `internal/httpserver/api_library.go` (three extra fields)

**Interfaces:**
- Produces:
  - `GET /api/library/facets` → JSON, shape below.
  - `/api/library` items gain `"updated_at": string` (RFC3339, `""` when zero), `"last_read_at": string` (RFC3339, `""` when nil), `"dup_group": int` (`-1` when the manga is not in a duplicate group).
  - Go handler method `func (s *Server) apiLibraryFacets(w http.ResponseWriter, r *http.Request)`.

Response shape (all keys always present; empty slices not null):

```json
{
  "overview": {
    "total_titles": 532, "status_done": 0, "status_ongoing": 0, "status_unknown": 532,
    "fully_read": 12, "started_reading": 30, "has_updates": 4, "pages_read": 8421,
    "most_title": "", "most_count": 0, "most_dup": 0,
    "fewest_title": "", "fewest_count": 0, "fewest_dup": 0
  },
  "plugins":    [{"plugin_id": "mangahere", "name": "MangaHere", "icon": "/plugin-static/mangahere/logo.png", "count": 42}],
  "categories": [{"name": "Action", "count": 88}],
  "tags":       {"mangahere:isekai_riiman": ["Action", "Comedy"]},
  "duplicate_count": 0
}
```

`tags` is keyed `pluginID:sourceMangaID` and only lists manga that have at least one category — it is the lookup the client needs for client-side tag filtering, which `/api/library` cannot answer.

- [ ] **Step 1: Write the failing test**

Uses the repo's existing helpers: `testServerFullDB` (`api_alttitles_test.go:25`), `seedManga` (`api_alttitles_test.go:75`), and the `httptest.NewRecorder` + `s.Router.ServeHTTP` pattern every handler test uses. There is no `getJSON` helper — decode by hand.

```go
func TestAPILibraryFacetsShape(t *testing.T) {
	s, db := testServerFullDB(t, "", false)
	mangaID := seedManga(t, db, "p1", "m1", "Title One")
	if err := db.AddCategory(mangaID, "Action"); err != nil {
		t.Fatalf("add category: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/library/facets", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, k := range []string{"overview", "plugins", "categories", "tags", "duplicate_count"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("missing %q", k)
		}
	}
	tags, ok := body["tags"].(map[string]any)
	if !ok {
		t.Fatal("tags is not an object")
	}
	if got, ok := tags["p1:m1"].([]any); !ok || len(got) == 0 {
		t.Fatalf("tags not keyed pluginID:sourceMangaID with a category list: %#v", tags)
	}
}

func TestAPILibraryItemsCarrySortFields(t *testing.T) {
	s, db := testServerFullDB(t, "", false)
	seedManga(t, db, "p1", "m1", "Title One")

	req := httptest.NewRequest("GET", "/api/library", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	for _, k := range []string{"updated_at", "last_read_at", "dup_group"} {
		if _, ok := items[0][k]; !ok {
			t.Fatalf("missing %q", k)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `CGO_ENABLED=0 go test ./internal/httpserver/ -run 'TestAPILibraryFacets|TestAPILibraryItemsCarry' -count=1`
Expected: FAIL — `missing overview` / `missing updated_at`.

- [ ] **Step 3: Implement `apiLibraryFacets` and extend the item struct**

In `api_library.go`, add to `apiLibraryItem` (declaration is at the top of the file):

```go
UpdatedAt  string `json:"updated_at"`
LastReadAt string `json:"last_read_at"`
DupGroup   int    `json:"dup_group"`
```

Populate: `UpdatedAt` from `m.UpdatedAt` formatted RFC3339 (empty when `IsZero()`); `LastReadAt` from `st.LastReadAt` (empty when nil); `DupGroup` defaults to `-1` and is overwritten from a `map[mangaRowID]int` built out of `FindPotentialDuplicates()`'s group order. Build that map once before the item loop, not inside it.

In `api_library_facets.go`, call `LibraryOverview`, `PluginMetas` (for display name), `LibraryCategoryCountsOrEmpty`, `ListLibraryCategories`, `CountLibraryByPlugin`, `FindPotentialDuplicates`, and marshal. Reuse the existing plugin-name/icon helpers the web handler uses (`pluginNameMap`/`pluginIconMap` live in `views_library.go`; if they are locals there, extract them into a small shared helper rather than duplicating the logo-fallback logic).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `CGO_ENABLED=0 go test ./internal/httpserver/ -count=1`
Expected: `ok goisekai/internal/httpserver`

- [ ] **Step 5: Verify against the live server**

```sh
CGO_ENABLED=0 go build -o goisekai ./cmd/goisekai
pkill -x goisekai; sleep 2
(setsid nohup ./goisekai -logLevel debug > goisekai.log 2>&1 &); sleep 9
curl -s -o /dev/null -w 'facets=%{http_code}\n' http://localhost:8080/api/library/facets
curl -s http://localhost:8080/api/library/facets | head -c 400
```

Expected: `facets=200`, and `overview.total_titles` equals the count in `/api/library`.

- [ ] **Step 6: Commit**

```sh
git add internal/httpserver/api_library.go internal/httpserver/api_library_facets.go internal/httpserver/api_library_facets_test.go internal/httpserver/api.go
git commit -m "feat(api): expose library facets and sort fields for the desktop client"
```

---

### Task 2: Split the QML sources, embed them, and add the format/lint gates

**Files:**
- Create: `cmd/goisekai-qml/qml/qmldir`
- Create: `cmd/goisekai-qml/qml/Main.qml`
- Create: `cmd/goisekai-qml/qml/Theme.qml`
- Create: `cmd/goisekai-qml/qml/components/LibraryToolbar.qml` (stub)
- Create: `cmd/goisekai-qml/qml/components/LibrarySidebar.qml` (stub)
- Create: `cmd/goisekai-qml/qml/components/MangaGrid.qml` (stub)
- Create: `cmd/goisekai-qml/qml/components/MangaCard.qml` (stub)
- Create: `cmd/goisekai-qml/qml/components/Pagination.qml` (stub)
- Create: `cmd/goisekai-qml/api.go`, `cmd/goisekai-qml/model.go`
- Modify: `cmd/goisekai-qml/main.go` (embed dir, temp dir, `engine.Load`)
- Modify: `justfile` (`fmt-qml`, `lint-qml`, wire into `check`)
- Delete: `cmd/goisekai-qml/main.qml` (moved into `qml/`)

**Interfaces:**
- Produces: `Theme.bg`, `Theme.panel`, `Theme.card`, `Theme.line`, `Theme.accent` (`#ffa332`), `Theme.text`, `Theme.textDim`, `Theme.textFaint`, `Theme.radius`, `Theme.gap`, `Theme.defaultRatio` (`0.703`).
- Produces Go: `type libraryItem struct { Title, Cover, Href, PluginID string; Read, Total, DupGroup int; HasNew bool; UpdatedAt, LastReadAt string }`; `func fetchLibrary(base, apiKey string) ([]libraryItem, error)`; `func fetchFacets(base, apiKey string) (facetsBundle, error)`; `func coverProxyURL(base, pluginID, coverURL string) string`; `func mangaHref(base, pluginID, sourceMangaID string) string`.
- Consumes: Task 1's `GET /api/library/facets` and the extended `/api/library`.

- [ ] **Step 1: Confirm the split-file mechanism works before building on it**

Already proven in a scratch probe: `engine.Load(QUrl_FromLocalFile(dir + "/Main.qml"))` with `import "components"` yields `rootObjects=1`, `failed=false`. The failure mode to avoid is `LoadData`, which has no base URL and therefore cannot resolve `import "components"` — hence the temp-dir unpack below.

- [ ] **Step 2: Add the justfile gates**

`qmlformat` has no `--check` flag on Qt 6.12, so the format gate compares against stdout. `qmllint` on this Qt build only reports syntax errors (verified: `widht: 10` passes, an unbalanced brace exits 255) — the recipe comment must say so, or the next person will trust it too much.

```make
qml_dir := "cmd/goisekai-qml/qml"

# Format the QML sources in place.
fmt-qml:
    qmlformat -i $(find {{qml_dir}} -name '*.qml')

# Lint and format-check the QML sources (read-only).
# qmllint on Qt 6.12 reports syntax errors only - a misspelled property slips
# through, so a passing lint is not proof the UI renders.
lint-qml:
    #!/usr/bin/env sh
    set -eu
    qmllint $(find {{qml_dir}} -name '*.qml')
    for f in $(find {{qml_dir}} -name '*.qml'); do
        qmlformat "$f" | diff -u "$f" - > /dev/null || { echo "not formatted: $f"; exit 1; }
    done
```

Add `lint-qml` to the `check` recipe's dependency list.

- [ ] **Step 3: Create `Theme.qml` and `qmldir`**

```qml
pragma Singleton
import QtQuick

QtObject {
    readonly property color bg: "#171717"
    readonly property color panel: "#1f1f1f"
    readonly property color card: "#242424"
    readonly property color line: "#3a3a3a"
    readonly property color accent: "#ffa332"
    readonly property color text: "#e5e5e5"
    readonly property color textDim: "#9ca3af"
    readonly property color textFaint: "#6b7280"
    readonly property int radius: 8
    readonly property int gap: 16
    readonly property real defaultRatio: 0.703
}
```

`qml/qmldir`:

```
module goisekai.qml
singleton Theme 1.0 Theme.qml
```

- [ ] **Step 4: Write `model.go` with the URL-building helpers and their test**

```go
func coverProxyURL(base, pluginID, coverURL string) string {
	if coverURL == "" {
		return ""
	}
	return base + "/image?pluginID=" + url.QueryEscape(pluginID) + "&url=" + url.QueryEscape(coverURL)
}

func mangaHref(base, pluginID, sourceMangaID string) string {
	return base + "/view/manga/" + url.PathEscape(pluginID) + "/" + url.PathEscape(sourceMangaID)
}
```

```go
func TestMangaHrefEscapesPathSegments(t *testing.T) {
	got := mangaHref("http://h", "batcave", "https://batcave.biz/33758-batman-2025.html")
	if strings.Contains(got, "batcave//33758") || strings.Contains(got, "?") {
		t.Fatalf("unescaped segment: %s", got)
	}
}

func TestCoverProxyURLRoundTripsQuery(t *testing.T) {
	got := coverProxyURL("http://h", "mangahere", "https://x/c.jpg?token=a&ttl=1")
	u, err := url.Parse(got)
	if err != nil { t.Fatal(err) }
	if inner := u.Query().Get("url"); inner != "https://x/c.jpg?token=a&ttl=1" {
		t.Fatalf("query mangled: %q", inner)
	}
}
```

- [ ] **Step 5: Rewire `main.go` to unpack the embedded tree and load from disk**

Keep `//go:build cgo` as line 1. Replace the single-file embed with:

```go
//go:embed qml
var qmlFS embed.FS
```

At startup: `os.MkdirTemp("", "goisekai-qml-*")`, `defer os.RemoveAll(dir)`, walk `qmlFS` writing each file preserving relative paths, then `engine.Load(qt.QUrl_FromLocalFile(filepath.Join(dir, "Main.qml")))`. Register `engine.OnObjectCreationFailed` to print the QML error to stderr and exit non-zero — a window that opens empty is otherwise indistinguishable from a bug.

- [ ] **Step 6: Create the five component stubs and `Main.qml`**

Stubs are minimal but valid (a `Rectangle`/`Item` with the properties `Main.qml` sets), so the tree compiles and the gates run before the real content lands. `Main.qml` holds `ApplicationWindow` and instantiates toolbar + sidebar + grid.

- [ ] **Step 7: Run the gates**

```sh
just fmt-qml
just lint-qml
just build-qml
CGO_ENABLED=0 go build ./...   # must still exit 0
```

Expected: all exit 0. Then run the app for 15s and confirm `rootObjects` is non-zero and stderr is empty.

- [ ] **Step 8: Commit**

```sh
git add cmd/goisekai-qml justfile
git commit -m "refactor(qml): split the client into per-component QML sources with lint gates"
```

---

### Task 3: `MangaCard` and `MangaGrid` with pagination

**Files:**
- Modify: `cmd/goisekai-qml/qml/components/MangaCard.qml`
- Modify: `cmd/goisekai-qml/qml/components/MangaGrid.qml`
- Modify: `cmd/goisekai-qml/qml/components/Pagination.qml`

**Interfaces:**
- `MangaCard` consumes model roles: `title`, `cover`, `read`, `total`, `hasNew`, `ratio`, `href`. Emits signal `activated(string href)`.
- `MangaGrid` produces: `property var rows` (filtered+sorted array), `property int pageSize: 24`, `property int page`; emits `pageChanged`.
- `Pagination` consumes `page`, `pageCount`; emits `requested(int page)`.

- [ ] **Step 1: `MangaCard.qml`**

Card layout as in the current `main.qml` delegate, with three changes: the cover aspect uses `model.ratio || Theme.defaultRatio` instead of a hardcoded 0.703; a distinct "still loading" state (`cover.status === Image.Null || Image.Loading`, showing nothing) versus an error state (showing "no cover"), so a missing AVIF plugin is diagnosable; and the click emits `activated(model.href)` instead of calling `Qt.openUrlExternally` directly, keeping navigation in one place.

- [ ] **Step 2: `MangaGrid.qml`**

`GridView` over a `ListModel`, `cellWidth`/`cellHeight` derived from a `zoom` property (default 168), `boundsBehavior: Flickable.StopAtBounds`, keyboard `↑↓←→` + `Return`. Slice the incoming `rows` to `pageSize` starting at `(page-1)*pageSize`. Two empty states, worded differently: `rows.length === 0` → "No manga matches …"; `rows.length > 0` but the slice is empty → "page out of range, back to 1".

- [ ] **Step 3: `Pagination.qml`**

Prev/next buttons plus numeric buttons with ellipsis for long ranges, mirroring `partials/pagination.lua`'s behaviour: always show first, last, current ±1. Disabled states must be visually distinct (`enabled: false` + dimmed colour), not just non-clickable.

- [ ] **Step 4: Verify visually and via the request log**

```sh
just build-qml
pkill -x goisekai-qml; sleep 2
before=$(grep -c 'path=/image' goisekai.log)
(setsid nohup ./goisekai-qml > /tmp/qml.log 2>&1 &); sleep 15
after=$(grep -c 'path=/image' goisekai.log)
echo "covers requested: $((after-before))"   # expect ~24, not 0 and not 532
```

Expected: ~24 (one page), proving both pagination and lazy loading work.

- [ ] **Step 5: Commit**

---

### Task 4: `LibraryToolbar`

**Files:**
- Modify: `cmd/goisekai-qml/qml/components/LibraryToolbar.qml`

**Interfaces:**
- Produces properties `query`, `sortKey`, `statusFilter`, `viewMode` (`"grid"`/`"list"`), signal `changed()`.
- Sort keys must match the server's vocabulary exactly: `updated`, `title`, `read`, `progress`, `recent_read`, `duplicates`.
- Status values: `all`, `reading`, `unread`, `done`, `new`.

- [ ] **Step 1: Build the toolbar**

Search field (`Ctrl+F` focuses, `Escape` clears), sort `ComboBox`, four status chips, grid/list toggle. Editing any control emits `changed()` and never touches the others' state.

- [ ] **Step 2: Verify the status semantics match the server's `filterLibrary`**

Client-side predicates must be identical to `internal/httpserver/library_sort.go:120`: `reading` → `read > 0 && total > 0 && read < total`; `unread` → `read === 0`; `done` → `total > 0 && read >= total`; `new` → `hasNew`. A mismatch means the same filter shows different counts in web and desktop.

- [ ] **Step 3: Commit**

---

### Task 5: `LibrarySidebar`

**Files:**
- Modify: `cmd/goisekai-qml/qml/components/LibrarySidebar.qml`

**Interfaces:**
- Consumes Task 2's `facetsBundle`: `overview`, `plugins[]`, `categories[]`, `tags{}`, `duplicateCount`.
- Produces signals `pluginSelected(string pluginID)`, `tagSelected(string tag)`, `duplicatesSelected()`.

- [ ] **Step 1: Build the sidebar**

Three sections: stat cards from `overview` (total titles, fully read, started, has updates, pages read); plugin list with counts as selectable rows (icons via the server's `/plugin-static/...` URL); tag list with counts. Clicking a row toggles that filter; clicking the active row clears it.

- [ ] **Step 2: Guard the tag filter against missing data**

`tags` only contains manga that have categories. Filtering by a tag must fall back to "no matches" rather than throwing when a manga key is absent — use `(tags[key] || [])`.

- [ ] **Step 3: Commit**

---

### Task 6: Wire state together in `Main.qml`

**Files:**
- Modify: `cmd/goisekai-qml/qml/Main.qml`
- Modify: `cmd/goisekai-qml/main.go` (pass both payloads as context properties)

**Interfaces:**
- Context properties: `libraryJson` (array), `facetsJson` (object), `statusText` (string).

- [ ] **Step 1: Centralise the filter/sort pipeline**

`Main.qml` owns one `rebuild()` that applies, in order: status predicate → tag predicate → plugin predicate → duplicates predicate → text query → sort → paginate. Each control only sets its own property and calls `rebuild()`; none resets another's value. Reset `page` to 1 only when the result count shrinks below the current page's start index, not on every filter change.

- [ ] **Step 2: Add refresh**

`F5` and a toolbar button re-run both fetches. Keep this in Go, not QML: expose a context property `refreshRequested` (int) that `Main.qml` binds a `Connections`-style watcher to is **not** possible without a `QObject` — the simplest verified path is for QML to call `XMLHttpRequest` itself against `/api/library` and `/api/library/facets`, parse the JSON, and hand the result to the same `rebuild()` used at startup. That keeps one code path for both first load and refresh. Do **not** invent a `QJSValue` callback through miqt; `NewQVariant14` is the only string channel this plan relies on and it is already proven to work.

The API key, when set, must travel on those requests too. Either pass it into the QML context property and set the header, or (better) leave refresh to Go and have `main.go` re-run `fetchLibrary` on a `QShortcut` and push the payload through a second context property that `Main.qml` watches with `onXChanged`. Verify whichever you pick actually re-renders; a context property set after `LoadData` does **not** notify a plain `property var` in QML.

- [ ] **Step 3: Verify filter interaction**

Manual, in the running app: type a query, change sort → query survives; select a status chip, change page → chip survives; select a plugin, type a query → both apply; clear all → 532 items in 23 pages.

- [ ] **Step 4: Full gate run**

```sh
just fmt-qml && just lint-qml && just build-qml
CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./cmd/... && CGO_ENABLED=0 go test ./internal/... ./pkg/...
```

Expected: every command exits 0.

- [ ] **Step 5: Commit and push**

---

## Explicitly Out of Scope

- **Write actions** (sync, toggle library, edit title): `/action/*` is CSRF-protected and needs a token flow. Deferred until the library read path is proven.
- **Reader view**, search page, plugins/settings/history pages: this plan is the library page only, per the agreed scope.
- **Native detail page**: clicking a card still opens `/view/manga/...` in the browser.
- **Static Qt build**: Qt stays dynamically linked; the binary does not need to run on a machine without Qt 6 + `kimageformats`.
