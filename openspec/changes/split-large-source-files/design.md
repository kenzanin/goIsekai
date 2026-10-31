# Design

## Context

scc scan (non-test, ≥ 200 LOC, 2026-10-31) lists 17 files to split. Two families:

1. **Lua templates** (`internal/templates/views/*.lua`, `partials/*.lua`): each view is one Lua module returning a render function that builds one big HTML string via concatenation. Only `detail.lua` currently requires a partial (`partials.detail_alt`); the engine already supports `require("partials.x")` with a module contract (function taking data, returning HTML string).
2. **Go files** (`internal/httpserver`, `internal/bridge`, `internal/database`, `cmd/goisekai`): same-package function clusters. Splitting is a pure move — Go has no headers to update within a package.

Existing gates: `just check` (fmt + golangci-lint + qml fmt), `just build` (css + br + CGO_ENABLED=0 go build), template/brotli tests (`TestBrotliSiblingsMatchSource`, `TestFrontendJSParses`) — any JS under `frontend/lib/` needs `just br`.

## Goals / Non-Goals

**Goals:**
- Every non-test source file < 200 LOC after the change (vendored min files excluded).
- Zero behavior change: identical HTML output, identical API responses.
- Same-package splits with **section extraction where a file is one large function**: a block of a function moves into a named helper/file-local function taking the values it already closed over as parameters, returning what the original block assigned. Statement order, expressions, and error handling stay identical.
- New unit tests for the split Go units, prioritizing currently thin ones (`views_library.go`, `actions_covers.go`, `api_reader.go`, `image_fetch.go`).

**Non-Goals:**
- No refactoring of logic, no renames of exported symbols, no package restructuring.
- No change to statement order, error semantics, or output; extraction is mechanical, not an opportunity to improve code.
- No splitting of `database/schema.go`'s ordered migrations literal (mechanical split risks dropping/reordering a migration; the DDL is append-only by design). Documented as intentionally kept.
- No vendored assets (`mermaid.min.js`, `tailwind.css`).
- No new abstractions (interfaces, registries) to "make splitting easier".

## Decisions

1. **Go: same-package moves, plus section extraction where the file is one big function.** Measured function counts per target (2026-10-31): `actions_covers.go` 7, `api_reader.go` 6, `sync.go` 10, `views_manga.go` 2, `views_migrate.go` 2 — these split by moving whole functions. But `views_library.go` (247 LOC) and `image_fetch.go` (234 LOC) are each a **single** function, so whole-function moves cannot shrink them: their long bodies are cut at existing section-comment boundaries into file-local helpers (`libraryDupMaps`, `libraryStatsMap`, …), each taking the values it already read and returning what the original block assigned. Same for the Lua views, which are one concatenation block each. Alternative considered — extracting packages — rejected: import churn, export decisions, zero user value.
2. **Lua: extract section renderers into partials with the existing `require` contract.** Each view keeps its top-level `render(data)` entry; big sections become `partials.<view>_<section>` modules that take the same `data` table and return their HTML fragment. `detail.lua` already demonstrates the pattern (`partials.detail_alt`, `partials.cover_modal`), so this is convention, not invention. Output byte-equality is checked by rendering before/after with the same fixture data in a test.
3. **Split boundaries follow existing comments.** The templates already carry section banners (e.g., `-- views/detail.lua`, sidebar/chapters blocks); cut lines there rather than inventing a new taxonomy.
4. **Order: Go first, then Lua.** Go splits are zero-risk moves verified by compile + existing tests; Lua partial extraction needs the byte-equality check. Tests written per split unit as it lands, not batched at the end.
5. **One file split = one commit.** Keeps `just check` green at each step and makes any output diff trivially attributable.

## Risks / Trade-offs

- **Lua string concat is context-sensitive** (a section may close tags opened earlier). Mitigation: byte-equality render test per split view with a rich fixture; boundaries chosen at top-level sections, never mid-element.
- **`just br` forgettability** — only relevant if JS moves; plan avoids `frontend/lib` entirely.
- **Import shuffle in Go can trigger unused-import lint.** `just check` after each commit catches it.
- **Test coverage work may reveal real bugs** — out of scope: file them as findings, don't fix inside this refactor (one concern per change).

## Migration Plan

Not applicable — no data, API, or config changes. Runtime deployments pick up the split transparently from the rebuilt binary.
