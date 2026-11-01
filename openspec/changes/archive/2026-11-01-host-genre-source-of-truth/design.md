# Design

## Context

Genre normalization already lives host-side at three call sites in
`internal/bridge`: `library.go` rewrites a manga row's genres,
`enrichment_store.go` rewrites enrichment categories before storing them, and
`plugins_genres.go` rewrites the per-plugin genre cache. All three go through one
type, `genreIndex` in `internal/bridge/genres.go`, which is built from a
`map[string][]string` — canonical name to variant spellings — that
`config.GenreAlias` supplies, seeded by `config.DefaultGenreAlias()` (26 entries,
each pairing a canonical name with the variants `genreKey` would already
collapse such as `sci fi`, `scifi`, `yaoi`, `bl`).

So the alias table is largely doing the work `genreKey` already does for free:
`genreKey` lowercases and strips the separators spaces, hyphens, underscores,
apostrophes, dots and slashes, which makes `sci fi` and `sci-fi` the same key.
The table's real residue is the small set of variants that differ in letters,
not separators: `yaoi`/`bl` for `Boys' Love`, `suspense` for `Thriller`,
`daily life`/`sol` for `Slice of Life`, `tragic` for `Tragedy`, `magical
girl`/`mahou shoujo` for `Magical Girls`, and a few more.

The user has decided the host owns one canonical list, unknown names are kept
verbatim, there is no new plugin native, the `[genre]` section is deleted, and
there is no data migration. See `proposal.md` — Why for the motivation.

## Goals / Non-Goals

Goals

- One host-owned canonical genre list, no variant dictionary, no config override.
- Normalization that still fixes case and separator drift on read.
- Unknown genre names preserved, not dropped.
- No new plugin-facing surface, so no plugin code or ABI change.

Non-goals

- No migration or de-dup of rows already in the database.
- No new vocabulary (`Ecchi`, `Shounen`, `Seinen`, `Josei`, `Manhwa` stay out).
- No `normalize_genre` native; normalization stays at the three existing call
  sites.

## Decisions

### D1 — A single string list, not a map

`DefaultGenreAlias()` returns `map[string][]string` because the config layer
treats a genre as "canonical name + variants". With the variants gone, the
value is just a name list, so the source of truth becomes
`[]string` of the 26 names. This mirrors the status change, where
`pluginutil` moved to `Statuses() []string` + `NormalizeStatus(string)`.

Alternative considered: keep the map and delete the variant values, leaving 26
keys with empty slices. Rejected — it keeps the whole alias plumbing
(`GenreAlias` field, `addGenreAlias`, `addAliasLine`, the `[genre]` INI branch,
the `putAlias` writer) alive for a map that no longer carries variants, which is
exactly the dead surface the status change removed.

### D2 — Where the list lives

The 26-name list becomes a host-owned value in the bridge, next to the index
that consumes it, because `internal/bridge/genres.go` is where all three call
sites converge. `DefaultGenreAlias()` in `internal/config/genre_alias.go` is
deleted.

Alternative considered: a `pluginutil.Genres()` helper like
`pluginutil.Statuses()`. Rejected — unlike status, genre matching is
separator-insensitive, which is `genreKey`'s job in the bridge, and there is no
plugin native that would need the vocabulary exported; putting it in `pluginutil`
would make plugins think they must map their genres themselves, the opposite of
this change's intent.

### D3 — Index built over the list, with verbatim fall-through

`newGenreIndex` keeps its `genreKey`-based lookup but builds the table only from
the 26 canonical names (each registered under its own key, as it does today),
so case- and separator-drifted input still resolves to the canonical spelling.
`normalize` keeps its verbatim behavior for an unmatched name and its
deduplication. No variant loop, no map argument.

Consequence, accepted: the ~10 letter-different variants the old table
handled (`yaoi`, `sol`, `suspense`, `tragic`, ...) no longer collapse to a
canonical name; a site that sends `yaoi` now stores `yaoi` beside `Boys' Love`.
This is the visible window the status change accepted, and it is the direct cost
of dropping the dictionary. The separator and case drift — the far commoner
real-world case (`sci fi` vs `SCI-FI`) — is still handled.

### D4 — `genreIndex` still exists, still three call sites

The type and the three call sites are unchanged; only the source of its table
changes. `loadGenreIndex` in `service_config.go` no longer reads config and
constructs the index from the list. This keeps the diff small and leaves the
behavior at each call site (dedup, order, empty-name skip) untouched.

Alternative considered: fold `genreIndex` into a free function. Rejected —
the index is held on `AppService` and rebuilt once per service, not per call;
keeping the type preserves that.

### D5 — `[genre]` becomes ignored, not an error

The config loader drops the `GenreAlias` field and the `[genre]` branch in
`config_set.go`. An INI section `[genre]` from an older version must still load
without error: unknown sections/keys are already ignored by the loader, so a
leftover section parses and is dropped. `putAlias` for `genre` goes away with
the field; status already removed its call, so `putAlias`'s last caller is gone
and the helper itself is deleted.

## Risks / Trade-offs

- [Duplicate genres across sources until plugins migrate] `yaoi` and `Boys'
  Love` can coexist in the filter. → Mitigation: accepted, visible; the genre
  filter already matches on the stored name, and each plugin's wording is fixed
  per site. This is the same accepted window as the status change.
- [A user who customized `[genre]` loses their additions on the next save]
  Their lines parse but are dropped. → Mitigation: the section is user-facing
  only to extend the vocabulary, which this change forbids by design; no
  migration of custom genres is attempted (non-goal).
- [Deleting `DefaultGenreAlias` breaks config tests] `genre_alias_test.go`
  asserts the alias table. → Mitigation: rewrite that test around the 26-name
  list and the verbatim behavior; delete the `[genre]` round-trip assertions.

## Migration Plan

Ship host-only, no plugin change, no data migration:

1. Delete `DefaultGenreAlias`, the alias field/helper and the `[genre]` INI
   branch; add the canonical list and rebuild `newGenreIndex` over it.
2. Point `loadGenreIndex` at the list.
3. Rewrite `internal/config/genre_alias_test.go` and the config `[genre]`
   round-trip tests; keep `genres.go`'s behavior tests but source them from the
   list.
4. `just check && just build`, restart.

Rollback is a revert; stored rows were never rewritten, so no data cleanup.

## Open Questions

None — the verbatim behavior, no-native, `[genre]` deletion and no-migration
decisions were made by the user during proposal.
