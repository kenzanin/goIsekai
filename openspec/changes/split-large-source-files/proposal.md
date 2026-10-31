# Proposal

## Why

Seventeen non-test source files under `internal/` and `cmd/` exceed 200 lines of code (scc scan, comments excluded via `--no-cocomo`; minified vendor files `mermaid.min.js` / `tailwind.css` excluded by nature). The largest — `internal/templates/views/detail.lua` at 526 lines, `views/search.lua` at 340 — each mix several unrelated render sections in one file, which makes navigation, review diffs, and targeted edits slow. The user also asked for unit-test completion as part of this work; splitting files first gives the tests smaller, clearer units to cover.

## What Changes

- Split every non-test source file ≥ 200 lines (excluding vendored/minified assets) into smaller files by **moving existing functions/sections** — no new logic, no behavior change.
- Scope (scc, 2026-10-31, non-test, ≥ 200 LOC):
  - Lua templates: `views/detail.lua` (526), `views/search.lua` (340), `views/logs.lua` (316), `views/library.lua` (311), `views/migrate.lua` (257), `partials/detail_chapters.lua` (232), `partials/library_sidebar.lua` (225), `partials/plugin_cards.lua` (222), `partials/detail_alt.lua` (207)
  - Go: `httpserver/views_library.go` (247), `bridge/image_fetch.go` (234), `httpserver/actions_covers.go` (224), `views_manga.go` (222), `bridge/sync.go` (216), `database/schema.go` (214), `main.go` (213), `setup.go` (212), `views_migrate.go` (212), `api_reader.go` (200)
- Complete unit tests for the newly split Go units, prioritizing files that currently have thin coverage (`views_library.go`, `actions_covers.go`, `api_reader.go`, `image_fetch.go` non-test paths).
- Templates: split means extracting render helpers/sections into sibling partial files required via the existing template engine — identical output.
- Go: split means moving functions into sibling files in the same package — zero import/API churn outside the package.
- Excluded: `schema.go` split only by concern already present (migrations vs DDL) if trivially separable; otherwise documented as intentionally kept (migrations are an ordered literal — splitting risks merge errors).

## Capabilities

### New Capabilities

(none — pure refactor, no externally observable behavior change)

### Modified Capabilities

(none — `skip_specs: true` is set in `.openspec.yaml`; no spec-level behavior changes. Validation passes because the change declares the opt-out.)

## Impact

- Files: ~20 source files touched; each becomes 2–4 smaller files in the same package/directory.
- Behavior: none — verified by existing suites (`just test`, template golden/partial tests) plus new unit tests added in this change.
- Risk: Lua template requires (path changes), Go same-package moves (low), import cycles (none expected — same package only).
- Tooling gates unchanged: `just check && just build` must stay green after each file split.
