# Design: alt-titles-manager

## Context

The current stopgap stores alt titles as a JSON string in `mangas.alt_titles` (one manga populated). Users want multi-provider fetch, per-title curation, main-title promotion, and a fuzzy library search that scales to 2000+ titles. The enricher design keeps ALL upstream API knowledge inside plugins; the host is a dumb aggregator + storage + CRUD surface.

Verified constraint: FTS5 is available in the project's SQLite driver (`modernc.org/sqlite` v1.57.0, probed true) — no new dependency.

## Goals / Non-Goals

- Goals: plugin-declared lookup servers; per-server fetch & merge; promote/remove alt titles; swap main title; FTS5-backed library search with typo-tolerant ranking; JSON API-first endpoints for every UI feature.
- Non-Goals: AI-provider plugin implementation (contract supports it, provider ships later); duplicate-title detection UI (data now enables it, feature later); FTS index over description/genres.

## Decisions

### D1: Proper table replaces JSON column
`alt_titles(id INTEGER PK, manga_row_id → mangas(id) ON DELETE CASCADE, title TEXT, source TEXT, UNIQUE(manga_row_id, title))`. The `mangas.alt_titles` column is dropped in the same migration; the single populated row is migrated to the new table. Rationale: per-row remove and source labels per title are first-class; UNIQUE gives dedup for free.

### D2: Provider discovery from plugin metadata
`PluginMeta` gains `alt_title_servers []{id, name}`. The host aggregates providers by iterating discovered plugins and reading metadata; capability = non-empty server list AND exported `GetAltTitles`. MangaDex JS plugin becomes the first provider: metadata gains `alt_title_servers: [{id:"mangadex", name:"MangaDex"}]`, and its `getAltTitles(arg)` parses `{"title","server"}` (already implemented for title; add server passthrough).

### D3: ABI shape
`GetAltTitles` input `{"title": string, "server": string}`, output `{"source": string, "titles": []string}`. Optional function — `js.go` load validation already skips it (skip-list pattern already shipped for `GetAltTitlesFunc`). The JS name mapping `GetAltTitles → getAltTitles` already exists in `jsFnNames`.

### D4: Swap semantics for main title
`PUT .../title` validates the requested title exists in `alt_titles` for that manga, then in ONE transaction: delete the alt row, set `mangas.title = new`, insert old main title as an alt row (source: keep "manual"? No — use the source label `"user"` to mark user-promoted titles). Rejected titles (not in list) → 400.

### D5: FTS5 index with app-side sync (no triggers)
SQLite FTS5 external-content tables + triggers work, but triggers hide writes from go-jet and complicate debugging. Chosen: plain contentless-sync approach — `library_fts` is a standalone FTS5 table (`title, alt, plugin_id UNINDEXED, manga_row_id UNINDEXED`); the database layer updates it in the same transaction at each write site (manga upsert, title swap, alt add/remove, library toggle). One rebuild helper `rebuildLibraryFTS()` backfills on migration. Trade-off: every write site must remember the sync — mitigated by funneling writes through dedicated DB methods.

### D6: Search = FTS candidates + Go fuzzy ranking
`SearchLibrary(q)`: tokenize query → `library_fts MATCH 'tok*'` (prefix) for candidates → Go-side score each candidate: exact(100) > prefix(80) > substring(60) > subsequence(30, typo-tolerant). Case-fold + diacritic-fold. Sort desc, cap 50. Empty query → empty result. FTS miss with short queries falls back to LIKE scan (still fast at 2000 rows) so 2-char queries still work.

### D7: API-first
All five endpoints live in `api.go` under the existing `/api` group (API-key middleware applies as elsewhere). View/action endpoints are thin wrappers reusing the same bridge methods — no business logic in handlers.

## Risks / Trade-offs

- FTS sync discipline (D5): forgetting a sync point makes search stale. Mitigation: single DB-method funnel + `rebuildLibraryFTS()` callable from a debug endpoint for manual repair.
- Provider rate limits (MangaDex ~5 req/s): fetch is user-triggered, single call per request — acceptable.
- Lunar/JS runtime differences: provider contract only targets JS plugins initially (WASM/Lua can adopt the same ABI later; mapping table pattern per runtime).

## Migration Plan

Single migration (user_version bump): create `alt_titles`; migrate the one JSON row into it; drop `mangas.alt_titles`; create `library_fts`; backfill via `rebuildLibraryFTS()`; regenerate jet types (DDL mirror per project rule: update `cmd/jetgen` and regenerate `.gen`).
