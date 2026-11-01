---
type: Decision
title: "Host owns the chapter date format: epoch seconds or verbatim prose, never a layout guess"
description: "Chapter release dates travel as unix epoch seconds or source prose verbatim; the host stopped guessing date layouts, storing unusable wording in a new released_text column while text.date_to_iso stays deprecated-but-live for the 23 plugins still calling it."
tags: [goisekai, date, released_at, abi, released_text, migration, host-source-of-truth, decision]
generated: { by: agent/mcp, at: "2026-11-01T02:46:21Z" }
status: stable
---

## Decision

The wire format for a chapter's release date is **unix epoch seconds**. A plugin either sends a number it can convert, or sends the source's own prose verbatim. The host no longer tries date layouts or relative phrases on the plugin's behalf.

This is the "host is the source of truth" rule applied to dates: the host defines the format and plugins adapt. It does not guess what a site meant.

## Wire shape (`pkg/types`)

`Chapter.ReleasedAt` stays a `time.Time` so the ~40 downstream packages keep their shape — that was deliberate. A custom `UnmarshalJSON` routes on the **JSON type** of `released_at`:

| Input | Result |
|---|---|
| JSON number | epoch **seconds** → `time.Unix(n, 0).UTC()` |
| RFC 3339 string | parsed into `ReleasedAt` |
| any other string | kept in `ReleasedText`, `ReleasedAt` zero |
| `null` / absent | zero time, empty text |

`ReleasedText string` is tagged `json:"-"`: it is an internal routing output, never something a plugin sends.

**Milliseconds are not supported.** A millis site divides by 1000 inside the plugin. Detecting the unit from digit count is exactly the guessing this change removes.

## Out-of-range guard

A numeric value outside `1970-01-01` .. `2100-01-01` becomes **no date**, with a logged warning — it is not reinterpreted. This is what stops an epoch-millisecond value from rendering as the year 58600.

## Storage

- New column `released_text TEXT` at **migration index 32** (`releasedAtMigration` is 31, `addedAtMigration` earlier).
- `released_at` stays `TIMESTAMP` with `NULL` meaning unknown, so **no existing row is rewritten**.
- Verified on the live DB after deploy: 30,663 rows still hold `released_at`, 0 rows had `released_text` (no plugin sends prose yet).

## Why `time.Time` and not a plain string everywhere

The alternative — carrying the date as a string throughout — was rejected because a failed decode into `time.Time` kills the **whole chapter list**, not just the date. Keeping the typed field localises the failure to the one value.

## What was deliberately NOT done: `text.date_to_iso`

`text.date_to_iso` and `internal/pluginutil/dates.go` (`dateLayouts`, `relativeUnits`, `relativeDate`, `isDigits` unit sniffing) are **untouched and still live**, marked deprecated. Reason: 23 live plugins call `date_to_iso`. Killing the layout guesses now would blank every chapter date until those plugins are migrated to epoch. Retiring it is a separate follow-up.

## Rendering

`internal/templates/partials/detail_chapter_row.lua` reuses the string branch it already had: if `ReleasedAt` formats to something real, use it; otherwise if `ReleasedText` is non-empty, render that verbatim instead of calling `formatDate`.

## Implementation notes worth keeping

- **`.gen` files were hand-edited.** This repo wires **no go-jet generator** (no justfile target, no `go:generate`), so `internal/database/.gen/{model,table}/chapters.go` were edited to mirror what regeneration would emit. There is a `cmd/jetgen` but it is not part of `just check`.
- Committed as `c6ba886`.
- The `just check` gate **does not cover `pkg/types`** — it is a separate Go module (`pkg/types/go.mod` + `replace`), so `./pkg/...` matches no packages. Its tests must be run with `cd pkg/types && go test ./...`.

# Related Concepts
- [Live plugin released_at/chapter-date parsing status across all 41 live plugins](0150-live-plugins-date-info-status.md): That audit records what the 41 live plugins currently send for released_at; this decision sets the wire format those plugins must move to (epoch seconds), which is the follow-up the audit feeds.
