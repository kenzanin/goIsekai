# Tasks

## 1. Canonical vocabulary in the host

- [x] 1.1 `internal/pluginutil/status.go`: replace `DefaultStatusMap` and the
  mapping-based `NormalizeStatus(m, raw)` with a single canonical list
  `Statuses() []string` = `Ongoing`, `Completed`, `Hiatus`, `Cancelled`,
  `Upcoming`, and `NormalizeStatus(raw string) string` doing an exact
  ASCII-lowercase match; a match returns the canonical label, no match returns
  `""`
- [x] 1.2 `internal/pluginutil/status_test.go`: rewrite the table for the new
  contract — every canonical label round-trips, `ONGOING` matches, `publishing`
  and `finished` return `""`, a mapping-table first argument returns `""`
- [x] 1.3 Verify: `go test ./internal/pluginutil/ -count=1` green

## 2. Runtime bindings follow the single-argument contract

- [x] 2.1 `internal/pluginmanager/lua_crypto.go`: `host.text.normalize_status(raw)`
  calls the single-argument native; drop the map-parsing branch
- [x] 2.2 `internal/pluginmanager/js_crypto.go` and `js_natives.go`: drop the
  `.default` map property and the `(map, raw)` form; expose the one-argument
  function only
- [x] 2.3 `internal/pluginmanager/wasm_host_helpers.go` and
  `yaegi_surface.go`: dispatch `text.normalize_status` with one string argument
- [x] 2.4 `internal/pluginmanager/host_surface.go`: update the
  `text.normalize_status` entry — its note about differing one-arg and two-arg
  forms per runtime is obsolete
- [x] 2.5 Verify: `go test ./internal/pluginmanager/ -count=1` green

## 3. Normalize at the write boundary

- [x] 3.1 `internal/bridge/sync.go`: normalize `m.Status` through
  `pluginutil.NormalizeStatus` inside `persistMangaDetails` before
  `UpsertManga`, so a plugin that skips the helper still cannot store a
  non-canonical status
- [x] 3.2 Add a test proving a plugin response carrying `publishing` stores an
  empty status, and `Completed` stores `Completed`
- [x] 3.3 Verify: `go test ./internal/bridge/ -count=1` green

## 4. Remove the alias plumbing

- [x] 4.1 `internal/config/genre_alias.go`: delete `DefaultStatusAlias` and
  `addStatusAlias`
- [x] 4.2 `internal/config/{config,config_defaults,config_save,config_set}.go`:
  drop `StatusAlias` from `Config`, its default, its `putAlias` write, and its
  `[status]` read; an existing `[status]` section is ignored, not an error
- [x] 4.3 `internal/bridge/{service,service_config,service_stats}.go`: delete
  `statusAlias`, `loadStatusAlias`, and the alias argument passed to
  `LibraryOverview`
- [x] 4.4 `internal/database/library_overview.go`: drop the `StatusAlias` type
  and the alias loop in `classifyStatus`; compare the stored label
  case-insensitively against `Completed`, and `Ongoing`/`Hiatus`, counting the
  rest as unknown
- [x] 4.5 Verify: `go test ./internal/config/ ./internal/bridge/ ./internal/database/ -count=1` green

## 5. Gate

- [x] 5.1 `just check && just build` green; restart per the deploy recipe and
  confirm the app serves HTTP 200
- [x] 5.2 Smoke-test: open a library manga whose stored status is already
  canonical and confirm the badge still renders; confirm a manga stored with raw
  text no longer shows a badge rather than showing raw text
- [x] 5.3 Confirm `goisekai.ini` no longer writes a `[status]` section after a
  config save, and that a pre-existing `[status]` section does not prevent
  startup
- [x] 5.4 Confirm the diff touches no plugin file — plugin migration is
  deliberately out of scope

## 6. Follow-up (not this change)

- Archive the change once the project's review requirements are satisfied.
- Genre/tag vocabulary is the same class of problem and gets its own change.
- Per-site plugin status migration proceeds after this change lands.
