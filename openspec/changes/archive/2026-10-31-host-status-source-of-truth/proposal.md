# Proposal

## Why

The host currently ships its own dictionary of upstream status spellings
(`internal/pluginutil/status.go`, `DefaultStatusMap`, 29 entries such as
`releasing`, `publishing`, `finished`, `on hiatus`, `discontinued`) so a plugin
can hand over whatever word a site prints and receive a canonical label. That
dictionary is not the only one: the same vocabulary is also kept in
`config.DefaultStatusAlias` plus the `[status]` section of `goisekai.ini`
(`Cancelled`/`Unknown`), in `pluginutil` (`Dropped`/`Upcoming`), and in the Odin
ABI helpers — with different canonical names across them. The live
`goisekai.ini` already disagrees with the code default, which is what that drift
looks like once it escapes.

The host owns the schema, so it should hold exactly one canonical status list
and refuse anything else. A plugin that scrapes `publishing` is the side that
must translate.

## What Changes

- **BREAKING** The host status vocabulary becomes exactly five canonical labels:
  `Ongoing`, `Completed`, `Hiatus`, `Cancelled`, `Upcoming`. `Upcoming` is added
  to the plugin-facing vocabulary, and `Dropped` is folded into `Cancelled`.
- **BREAKING** `normalize_status` (`host.text.normalize_status`) stops being a
  mapper. It takes one argument, the raw value, and returns the canonical label
  when that value matches a canonical label case-insensitively — otherwise the
  empty string. The host never emits a non-canonical status.
- **BREAKING** The plugin-supplied mapping form `normalize_status(map, raw)` and
  `DefaultStatusMap()` are removed. No in-tree plugin uses the two-argument form
  today (0 of 84 call sites).
- `[status]` alias configuration is removed: `config.DefaultStatusAlias`,
  `Config.StatusAlias`, the `[status]` INI section, `addStatusAlias`, and its
  `putAlias` writer. Library statistics stop resolving spellings and compare the
  stored canonical label directly.
- Status is normalized once, at the write boundary (`internal/bridge/sync.go`),
  so `mangas.status` already holds a canonical label and the detail badge, the
  library filters and the statistics read the same five values.
- Genre/tag vocabulary is **not** changed by this proposal; it is the same class
  of problem and gets its own change once this one lands.

## Capabilities

### Modified Capabilities

- `host-text-helpers`: the status-normalization requirement changes from a
  plugin-overridable mapping to a single canonical validator, and the
  plugin-overridable default status map requirement is removed.

## Impact

- Host code: `internal/pluginutil/status.go` (vocabulary only),
  `internal/pluginmanager/{lua,js,wasm,yaegi}` status natives,
  `internal/bridge/sync.go` (write-boundary normalization),
  `internal/database/library_overview.go` (drop alias resolution),
  `internal/config/{genre_alias,config,config_defaults,config_save,config_set}.go`,
  `internal/bridge/{service,service_config,service_stats}.go`.
- Config: the `[status]` section disappears from `goisekai.ini`; an existing
  section is ignored (and dropped on the next save), not treated as an error.
- Plugins: 84 files call `normalize_status`. Each becomes a no-op until it maps
  its site's wording itself, so badges go blank rather than wrong. The host
  change ships first and plugins follow per-site, as agreed.
- Data: rows written before this change still hold raw upstream text
  (`publishing`, `finished`). They count as unknown statistics until re-synced;
  the change adds no rewrite migration.
- Risk: the visible regression window is "status badges missing for plugins that
  have not been migrated yet". Accepted, because the alternative is the host
  silently guessing — which is the thing being removed.
