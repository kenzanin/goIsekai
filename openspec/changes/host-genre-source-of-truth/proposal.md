# Proposal

## Why

`internal/config/genre_alias.go` ships `DefaultGenreAlias()`, a 26-entry table
that pairs each canonical genre with the alternate spellings plugins are known
to send (`scifi`, `sci fi`, `yaoi`, `bl`, `sol`, `suspense`, `super hero`, ...).
`bridge.newGenreIndex` turns that table into a lookup, and three call sites use
it to rewrite plugin-supplied names before they are displayed or stored:
`library.go` (manga rows), `enrichment_store.go` (categories), and
`plugins_genres.go` (the per-plugin genre cache).

That is the same arrangement the status vocabulary just had removed. The host
is guessing what each upstream site means, and it keeps that guess in a table
the user can also edit through the `[genre]` section of `goisekai.ini` — which
means the vocabulary has a code default and a config override that can drift
apart, exactly as the status aliases did.

There is a second, quieter problem. The `[genre]` section is not only a variant
dictionary: `addGenreAlias` accepts any key as a *new* canonical name, so a user
line `genre_alias.Ecchi = ecchi` silently extends the host vocabulary. The
host's canonical list is therefore whatever the code shipped plus whatever is in
one machine's INI file, and no spec describes it.

The host owns the schema, so it should hold exactly one canonical genre list and
let a plugin deal with its own site's wording.

## What Changes

- The host genre vocabulary becomes exactly the 26 genre names currently in
  `DefaultGenreAlias`, with no variant spellings attached. They stay the
  MangaDex `genre` tag group the current comment already documents, so a
  library scraped from any plugin still lines up with that vocabulary.
- Genre normalization moves to a single host-owned list and stops consulting
  any alias table. `DefaultGenreAlias()`, `Config.GenreAlias`,
  `addGenreAlias`, `addAliasLine`, the `[genre]` INI section and its `putAlias`
  writer are removed. This mirrors the `[status]` removal, so `putAlias` — whose
  only remaining caller is genre — goes away entirely.
- `genreIndex` is reduced to a canonical-spelling fixer: a name that matches a
  canonical genre case-insensitively (ignoring the separators `genreKey` already
  collapses) is rewritten to that canonical spelling; **a name that matches
  nothing is kept verbatim**, never dropped. Genre carries information that
  status does not — dropping an unknown value would delete a tag from a manga
  and from the library filter, so this change does not do it.
- **No new plugin-facing native.** There is no `normalize_genre` today and this
  change does not add one. Normalization stays host-side at the same three call
  sites, so no plugin has to change and no ABI surface grows.
- `[genre]` configuration is removed from the INI writer. An existing section is
  still parsed without error and simply ignored, so upgrading does not fail on
  a previously written config.

## Capabilities

### Added Capabilities

- `genre-vocabulary`: the host owns one canonical genre list, normalizes
  case-insensitively, and preserves unrecognized names verbatim.

## Impact

- Host code: `internal/config/genre_alias.go` (delete the alias table and its
  helpers), `internal/config/{config,config_defaults,config_save,config_set}.go`
  (drop the `GenreAlias` field, its default, its save writer and its `[genre]`
  branch), `internal/bridge/genres.go` (index over the canonical list only),
  `internal/bridge/service_config.go` (`loadGenreIndex` no longer reads config).
- Config: the `[genre]` section disappears from `goisekai.ini` on the next save.
  An existing section is ignored rather than rejected.
- Data: no migration. Stored genre rows already hold whatever the index produced,
  and entries that came from the deleted variant table stay as they were; they
  are simply no longer rewritten on read.
- Tests: `internal/config/genre_alias_test.go` currently exercises the alias
  table and must be rewritten around the canonical list. The `[genre]`
  round-trip assertions in the config tests go with it.
- Risk: a plugin whose site uses a variant spelling (`scifi`, `yaoi`, `sol`)
  now reaches the library with that spelling rather than the canonical one,
  so the same genre can appear under two names across sources until the plugin
  maps it. Accepted, and visible rather than silent — the alternative is the
  host guessing, which is what this change removes.

## Non-goals

- No genre vocabulary expansion: `Ecchi`, `Shounen`, `Seinen`, `Josei`, `Manhwa`
  and similar stay out. They are not MangaDex `genre` group members, and adding
  them is a separate decision about what the library should offer.
- No data migration or de-duplication of existing genre rows.
- No plugin changes and no new native.
