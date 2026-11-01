---
type: Decision
title: "Host owns one canonical genre vocabulary; the [genre] alias map is gone"
description: "Genre vocabulary is the 26 canonical MangaDex genre-group names exported as CanonicalGenres; unknown names stay verbatim, the [genre] INI alias map and all its config plumbing were deleted, and legacy rows are not migrated."
tags: [goisekai, genre, abi, config, host-source-of-truth, decision, canonical-vocabulary]
generated: { by: agent/mcp, at: "2026-11-01T02:46:22Z" }
status: stable
---

## Decision

The host owns one canonical genre vocabulary: the **26 fixed MangaDex `genre` group names**, exposed as an exported `var CanonicalGenres []string` in `internal/bridge/genres.go`. Plugins map their site's spellings onto those names. The host keeps a name it does not recognise **verbatim** and never invents a translation.

This is the same rule as the status and date changes: the host defines the vocabulary, plugins adapt. There is no config-driven dictionary of site spellings.

## Normalisation

`genreKey` collapses a name for matching by lowercasing and dropping `' '`, `'\t'`, `'-'`, `'_'`, `'\''`, `'\u2019'`, `'.'`, `'/'`. `newGenreIndex(CanonicalGenres)` registers each canonical name under its key. `normalize()` keeps the verbatim fall-through, dedups, and preserves order.

So `Boys' Love`, `boys_love` and `boys-love` all land on the canonical entry, while an unknown name passes through unchanged rather than being silently dropped.

## Config plumbing removed

- `internal/config/genre_alias.go` — **deleted**, along with `genre_alias_test.go`.
- `GenreAlias` field and `aliasTouched` removed from `internal/config/config.go`.
- `DefaultGenreAlias()` removed from `config_defaults.go`.
- The `strings.EqualFold(section, "genre")` branch removed from `config_set.go` (only the `[enhance]` special case remains).
- `putAlias(f, "genre", …)` and the `putAlias` helper removed from `config_save.go`; `sortedKeys` was kept because `[enhance]` still uses it.
- `loadGenreIndex` now returns `newGenreIndex(CanonicalGenres)` — the config read is gone.

The `[genre]` INI section is **ignored on load and never written on save**. A stale `[genre]` block still sitting in `goisekai.ini` simply vanishes on the next save.

## Deviation from the tasks file

`tasks.md` specified a `CanonicalGenres() []string` accessor; the implementation shipped an **exported `var`** instead. Deliberate, and noted in commit `63e9403`: callers only read the list, and a var keeps `newGenreIndex(CanonicalGenres)` readable at the one call site.

## No data migration

Legacy rows written under the old alias map are **not** rewritten. They simply count as unknown names until the plugin re-syncs that manga. Accepted because the alternative was a migration guessing which old alias meant which canonical name — the same guessing being removed everywhere else.

## Tests

`internal/bridge/genres_test.go` was rewritten for canonical-only input (`TestGenreIndexNormalizesToCanonical` with 8 subtests, including "canonical spelling resolves without an alias entry" and "unknown names pass through unchanged"). `genres_cache_test.go` now expects `bl` to stay `bl` rather than becoming `Boys' Love`. `internal/config/alias_sections_test.go` replaced the deleted alias test.

Verified on the live DB after deploy: `Ecchi` rows remain present verbatim.

# Related Concepts
- [Detail-page read toggle, genre tags, and reader-prefs persistence](detail-page-read-genre-readerprefs-fixes.md): Genre tags are rendered on the detail page; that finding covers the detail-page genre-tag behaviour this canonical vocabulary now feeds.
