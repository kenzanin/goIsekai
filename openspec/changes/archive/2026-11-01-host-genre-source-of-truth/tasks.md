# Tasks

## 1. Canonical genre list in the bridge

- [x] 1.1 `internal/bridge/genres.go`: add `CanonicalGenres() []string` = the 26 MangaDex `genre` group names (`Action`, `Adventure`, `Boys' Love`, `Comedy`, `Crime`, `Demons`, `Drama`, `Fantasy`, `Girls' Love`, `Historical`, `Horror`, `Isekai`, `Magical Girls`, `Martial Arts`, `Mecha`, `Medical`, `Mystery`, `Philosophical`, `Psychological`, `Romance`, `Sci-Fi`, `Slice of Life`, `Sports`, `Superhero`, `Thriller`, `Tragedy`, `Wuxia`); replace `newGenreIndex(aliases map[string][]string)` with `newGenreIndex(names []string)` building the same `genreKey` table from canonical names only (no variant loop)
- [x] 1.2 `internal/bridge/genres.go`: keep `normalize`'s verbatim fall-through and dedup unchanged; drop the `config` import and `indexGenreAliases()` (its callers get the list-based constructor)
- [x] 1.3 `internal/bridge/genres_test.go` (or the test that covers `genreIndex`): rewrite the table to source from `CanonicalGenres()` — `SCI-FI`/`sci fi` → `Sci-Fi`, `romance` → `Romance`, `boys love` → `Boys' Love`, unknown `Ecchi` stays `Ecchi`, unknown-only input returns the input, duplicates collapse; verify `go test ./internal/bridge/ -run Genre -count=1`
- [x] 1.4 `internal/bridge/service.go`: point the `genres *genreIndex` construction at `newGenreIndex(CanonicalGenres())`; `internal/bridge/service_config.go`: `loadGenreIndex` no longer reads config — return `newGenreIndex(CanonicalGenres())`; keep the `genres` field and its use in `library.go:86`, `enrichment_store.go:79`, `plugins_genres.go:33` untouched
- [x] 1.5 Verify: `go build ./internal/...` then `go test ./internal/bridge/ -count=1` green

## 2. Remove the alias config plumbing

- [x] 2.1 `internal/config/genre_alias.go`: delete `DefaultGenreAlias`, `addGenreAlias`, `addAliasLine` and the `aliasTouched` bookkeeping; keep the file only if `addAliasLine` still serves another alias type — if not, delete the file
- [x] 2.2 `internal/config/config.go`: delete the `GenreAlias` field and its doc comment; `internal/config/config_defaults.go`: delete the `GenreAlias: DefaultGenreAlias(),` line; `internal/config/config_set.go`: delete the `strings.EqualFold(section, "genre")` branch so a leftover `[genre]` section loads as unknown keys (ignored, not an error)
- [x] 2.3 `internal/config/config_save.go`: delete `putAlias(f, "genre", c.GenreAlias)`; with the status call already gone, delete the now-unreferenced `putAlias` helper
- [x] 2.4 Verify: `grep -rn "GenreAlias\|DefaultGenreAlias\|addGenreAlias\|addAliasLine\|putAlias" internal/ --glob '!*_test.go'` returns nothing; `go build ./internal/config/`

## 3. Rewrite config + bridge tests

- [x] 3.1 `internal/config/genre_alias_test.go` (or wherever the alias-table tests live): delete assertions on the variant table; replace with a test that a config file containing a `[genre]` section still loads without error, that its contents are ignored, and that saving writes no `[genre]` section
- [x] 3.2 Any config round-trip test that sets `GenreAlias` (e.g. `TestSaveRoundTripsAliases`): drop the genre alias assertion, keep the rest
- [x] 3.3 `internal/bridge/plugins_genres_test.go` / enrichment / library tests that assert canonical names: keep expectations, source them from `CanonicalGenres()`; where a test fed a variant like `scifi` expecting `Sci-Fi`, update it to expect `scifi` kept verbatim
- [x] 3.4 Verify: `go test ./internal/config/ ./internal/bridge/ -count=1` green

## 4. Gate, restart, smoke-test

- [x] 4.1 `just check && just build` green; restart per the deploy recipe and confirm the root page returns HTTP 200
- [x] 4.2 Smoke-test: a manga whose stored genres include separator/case drift (e.g. `sci fi`) renders as `Sci-Fi` on the detail page and in the library filter; an unknown genre (e.g. `Ecchi`) still displays as-is
- [x] 4.3 Confirm `goisekai.ini` no longer writes a `[genre]` section after a config save; a pre-existing `[genre]` section is ignored on load
- [x] 4.4 `git diff` review: no plugin file touched, no native added, no data migration
