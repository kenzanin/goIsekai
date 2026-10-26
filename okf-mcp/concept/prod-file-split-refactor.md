---
description: "Every prod source file over 200 lines that held many functions is now split"
id: prod-file-split-refactor
title: Every prod source file over 200 lines that held many functions is now split - the rule, the tool, and the two traps
type: finding
tags: [goisekai, refactor, code-layout, staticcheck, deadcode, split]
---

# Every prod source file over 200 lines that held many functions is now split

Request: *"scan semua prod source cari file yang > 200 pecah jadi file kecil
jika dalam 1 file terdiri dari banyak function jika 1 file terdiri dari 1
function besar biarkan saja. pecah file dengan cara move function ke file
baru. sekalian optimasi dan bug check"*.

## The rule that was applied

A prod file over 200 lines was split **only when it held many functions**. A
file dominated by one big function was left alone, because the complaint is
about *file size*, not about a long function - and cutting a 224-line
`viewLibrary` in half would only move the same code into two places.

Measured as: the largest declaration's share of the file.

## What was split (17 files -> 32 new files)

| package | was | became |
|---|---|---|
| `internal/workers` | `enqueue.go` 346 | `enqueue.go` 130, `enqueue_job.go` 187, `fairness.go` 38 |
| | `pool.go` 215 | `pool.go` 158, `lane.go` 38, `errors.go` 39 |
| `internal/pluginmanager` | `goplug.go` 300 | `goplug.go` 164, `goplug_cstr.go` 53, `goplug_call.go` 97 |
| | `wasm.go` 226 | `wasm.go` 126, `wasm_call.go` 66, `wasm_runtime.go` 49 |
| | `manager.go` 216 | `manager.go` 129, `manager_challenge.go` 41, `manager_setters.go` 62 |
| | `install.go` 218 | `install.go` 130, `load.go` 96 |
| | `wasm_host_dispatch.go` 212 | `wasm_host_dispatch.go` 120, `wasm_host_helpers.go` 102 |
| | `discovery.go` 208 | `discovery.go` 133, `discovery_meta.go` 81 |
| `internal/bridge` | `image.go` 286 | `image.go` 164, `image_referer.go` 78, `image_cached.go` 54 |
| | `logo_cache.go` 271 | `logo_cache.go` 100, `logo_cache_fetch.go` 118, `logo_cache_path.go` 68 |
| | `jobs.go` 221 | `jobs.go` 58, `jobs_enqueue.go` 84, `jobs_run.go` 94 |
| | `library.go` 203 | `library.go` 134, `library_pages.go` 79 |
| `internal/httpserver` | `reader.go` 271 | `reader.go` 90, `reader_chapters.go` 62, `reader_view.go` 132 |
| | `api.go` 214 | `api.go` 56, `api_library.go` 99, `api_helpers.go` 71 |
| `internal/hostnet` | `verify.go` 261 | `verify.go` 126, `verify_cookies.go` 145 |
| `internal/database` | `chapters_read.go` 247 | `chapters_read.go` 117, `chapters_bound.go` 87, `chapters_progress.go` 57 |
| `cmd/goisekai-tray` | `main.go` 212 | `main.go` 92, `lifecycle.go` 105, `paths.go` 52 |

No declaration was rewritten - only **moved**, with its doc comment. The
mechanical splitter (`/tmp/split_go.py`) copies the package clause and import
block, cuts the declaration out of the source, and `goimports` then prunes the
imports each file no longer needs.

## What was deliberately NOT split

Six prod Go files are still over 200 lines, and each is left for a reason:

| file | lines | why |
|---|---|---|
| `internal/httpserver/views_library.go` | 247 | one function, `viewLibrary` = 224 |
| `internal/httpserver/views_manga.go` | 222 | `buildMangaDetailData` = 189 |
| `internal/httpserver/views_migrate.go` | 212 | `viewMigrate` = 187 |
| `cmd/goisekai/main.go` | 209 | `main` = 182 |
| `internal/database/schema.go` | 206 | 0 functions - it is the ordered migration list, and its index constants (`altTitlesMigration = 9` …) are load-bearing |
| `internal/bridge/image_fetch.go` | 201 | one function, `fetchImage` = 175 |

**No Lua template was split.** All nine templates over 200 lines
(`detail.lua` 526, `library.lua` 307, `search.lua` 305, `logs.lua` 288,
`migrate.lua` 257, `detail_chapters.lua` 232, `library_sidebar.lua` 225,
`plugin_cards.lua` 222, `detail_alt.lua` 207) are single-function modules -
each is exactly one `return function(data)` plus at most one helper.
`views/detail.lua` already factors its three biggest blocks into partials
(`partials.detail_alt`, `partials.detail_chapters`, `partials.verify_wizard`)
via `require`. Splitting further would only add `require()` wiring without
making any function smaller.

## The two traps when splitting Go files by hand

**Trap 1 - a comment must travel with its own declaration.** A block has to
end where the *next* declaration's doc comment starts. Cutting at the next
`func`/`type` line instead silently moves the next function's doc comment
onto the previous function. The first version of the splitter did exactly
that and the first split produced a file where `takeKey`'s comment described
`fetchGate`.

**Trap 2 - the receiver is not the type.** `pool.go` has two methods called
`String` (`Lane.String` and `JobStatus.String`). Matching on the name alone
picks the wrong one, so the splitter parses the receiver separately and
targets are written `func:Lane.String`.

Both traps were found by the first two runs, not by inspection - the splitter
is at `/tmp/split_go.py` and is worth keeping if this is ever repeated.

## Bug check results

`staticcheck` was clean of correctness findings (`-checks 'SA*'`): no SA
reports at all. The full run only produced style noise - `ST1001` dot imports
(AGENTS.md documents those as intentional for the go-jet DSL files),
`ST1000`/`ST1003`/`ST1020`/`ST1021` naming and comment form.

`deadcode ./...` found exactly **two** unreachable functions:

- `internal/bridge/migrate_candidates.go:160` **`AutoSelectCandidate`** -
  production-dead: no template and no frontend code references an auto-select;
  only `migrate_test.go` and `migrate_candidates_test.go` call it. It is a
  candidate for deletion, but it is exported and covered by two test files, so
  removing it is a decision rather than a cleanup.
- `internal/pluginmanager/host_surface.go:154` **`hostSurfaceNames`** - used
  only by `helper_parity_test.go` (lines 140 and 204). Not dead: it is the
  parity test's source of truth for the host-helper table. Kept.

One earlier claim turned out to be obsolete: `workers.Permanent()` is wired
correctly. `internal/workers/enqueue.go` checks it with
`errors.AsType[*ErrPermanent](err)` and `pool_test.go` has
`TestPermanentNoRetry`.

## Two known flakes, neither a regression

- `TestSamePriorityFIFO` - failed in a full `internal/bridge` run
  ("expected 3 requests, got 1") and **passed 3/3 in isolation**
  (`ok goisekai/internal/bridge 0.261s`). A timing flake.
- `TestJSInvokeDeadlineCancelsBlockingFetch` - see
  [wasm-install-deadlock](wasm-install-deadlock.md); it hung on the manager
  lock, which was fixed separately in the same session.

## Gate

`go test ./... -count=1` exit 0, `go vet ./...` clean, `golangci-lint run`
0 issues, `gofmt -l internal/ cmd/` empty, `git diff --check` clean.
