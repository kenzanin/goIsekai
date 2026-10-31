# Design

## Context

See `proposal.md` — Why. Two facts about the current code shape drive this design:

- `normalize_status` is reached through four separate runtime bindings
  (`internal/pluginmanager/lua_crypto.go`, `js_crypto.go`,
  `wasm_host_helpers.go`, `yaegi_surface.go`), all delegating to
  `pluginutil.NormalizeStatus(m, raw)`. The two-argument mapping form exists in
  every binding but no in-tree plugin uses it: 0 of 84 call sites pass a map.
- Status is persisted from the plugin response at
  `internal/bridge/sync.go` (`persistMangaDetails` → `db.UpsertManga{Status: m.Status}`)
  and read back verbatim by the detail badge
  (`internal/templates/views/detail.lua`) and by library statistics
  (`internal/database/library_overview.go`, `classifyStatus`).

## Goals / Non-Goals

**Goals**

- One canonical status vocabulary in exactly one place.
- The host never emits a status it does not recognize.
- Normalization happens once, at a boundary, rather than in every plugin.

**Non-Goals**

- Migrating the 84 plugin call sites. They follow per-site in separate work.
- Rewriting historical `mangas.status` rows.
- The genre/tag vocabulary, which has the same shape of problem and gets its own
  change.
- Any UI redesign. `Cancelled`/`Upcoming` already render through the existing
  badge and filter code paths.

## Decisions

### D1. Vocabulary lives in one exported slice, not a map

`pluginutil` keeps `Statuses()` returning `[]string{"Ongoing", "Completed",
"Hiatus", "Cancelled", "Upcoming"}`, and `NormalizeStatus(raw string) string`
does an exact ASCII-lowercase match against it.

Rationale: the canonical labels are a **contract**, not configuration. Library
statistics compare against them (`Completed`, `Ongoing`, `Hiatus`), so they must
be literals in code somewhere; keeping them in one slice means there is exactly
one such place. Exact match, not substring: substring matching (the
`classifyStatus` approach) would make `not yet published` match `published` and
invent a status from an unrecognized word, which is the failure being designed
out.

Alternatives considered:

- *Keep a map of spellings → labels* (status quo): rejected, that is the host
  guessing.
- *Keep the map but move it to `goisekai.ini`*: rejected by the requester.
  Configuration made the drift worse — the shipped INI already disagreed with the
  code default (`Cancelled`/`Unknown` vs `Dropped`/`Upcoming`).
- *Longest-alias substring, reusing `classifyStatus`*: rejected, too permissive
  (see above).

### D2. `Upcoming` is added; `Dropped` becomes `Cancelled`

The vocabulary is `Ongoing`, `Completed`, `Hiatus`, `Cancelled`, `Upcoming`.
Adding `Upcoming` gives sites with a "not yet published" state a canonical home
instead of forcing them into `Hiatus` or nothing. `Dropped` is retired into
`Cancelled`, matching the label the INI default and `library_overview` already
use, so the two vocabularies stop disagreeing.

`Upcoming` is deliberately not derived from MangaDex's four-value enum: the
requester's sites do publish "not yet released" as a distinct state, and the host
spec is about what the host accepts, not about mirroring one upstream API.

### D3. Normalize at the write boundary, and also in the native

The native validates (`D2`), and `persistMangaDetails` normalizes once before
`UpsertManga`. Both use the same `NormalizeStatus`.

Rationale: keeping validation in the native gives plugins an immediate,
observable signal; normalizing again at the write boundary means a plugin that
skips the helper, or an old plugin, still cannot put a non-canonical status into
the database. This is the defence that lets the host stop echoing raw text
without depending on 84 plugins being correct first.

### D4. Statistics compare the stored label directly

`classifyStatus` stops taking aliases and compares the stored value
case-insensitively against the three buckets it cares about (`Completed`;
`Ongoing` and `Hiatus`; everything else is unknown), as
`library_overview.go` already does after resolving. `StatusAlias`,
`DefaultStatusAlias`, `loadStatusAlias`, the `[status]` INI section and its
writer are removed.

Rationale: with normalization at the write boundary, the read side has nothing
left to resolve. Keeping alias resolution as well would leave two mechanisms
that can disagree.

### D5. An unreadable `[status]` section is ignored, not an error

Existing `goisekai.ini` files may still carry `[status]`. `LoadOrInit` ignores
the section, and the next save drops it.

Rationale: the upgrade path must not fail on a config file written by the
previous version. Failing to start over a removed section is a worse outcome
than silently dropping dead keys.

## Risks / Trade-offs

- **[Status badges go blank for un-migrated plugins]** → The host no longer
  echoes raw text, so a plugin sending `publishing` yields no badge. Mitigation:
  the badge already renders conditionally on a non-empty status
  (`views_manga_detail`). Ship the host change first, then migrate plugins
  site-by-site; the window is visible but not broken.
- **[Historical rows hold raw text]** → Old rows (`publishing`, `finished`) no
  longer match the vocabulary and count as unknown in statistics until re-synced.
  Mitigation: re-sync is already an existing per-manga operation; no rewrite
  migration is added, so this is deliberate and reversible.
- **[Third-party plugins break]** → Any out-of-tree plugin using the
  `normalize_status(map, raw)` form loses it. Mitigation: the ABI surface table
  (`internal/pluginmanager/host_surface.go`) already carries a note that the
  one-arg and two-arg forms differ per runtime; that entry is updated to record
  the single-argument contract, and the removed form returns the empty string
  rather than erroring.
- **[`Upcoming` not in MangaDex]** → A plugin mapping to MangaDex's enum exactly
  has no target for "not yet published". Mitigation: this is a plugin-side choice
  by design (D2); such a plugin returns no status.

## Migration Plan

1. Host: vocabulary + `NormalizeStatus` single-argument contract, four runtime
   bindings updated.
2. Host: write-boundary normalization in `persistMangaDetails`.
3. Host: drop alias plumbing (`config`, `bridge`, `database`).
4. Verify: existing status tests run against the new contract; a row written with
   raw text is observed to store empty/unknown rather than the raw word.
5. Plugins: per-site migration afterwards, each mapping its own wording.

Rollback: revert the change. The `[status]` section is additive in the INI, so a
rolled-back binary regenerates it on first save; no data migration was performed,
so nothing is lost either way.

## Open Questions

- None that change the specs or the approach. The genre/tag equivalent is
  explicitly deferred to its own change.
