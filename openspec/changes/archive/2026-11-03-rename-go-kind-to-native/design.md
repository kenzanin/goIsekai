# Design: rename the `.so` kind from `go` to `native`

## Context

The engine registry added in `plugin-engine-registry-quickjs` resolves one runtime per plugin `kind` through a single `map[string]engine`. Five kinds are registered: `lua`, `js`, `go`, `yaegi`, `wasm`. The `go` key does not point at a Go-script runtime — `loadGo` opens a shared library with `openNativeLib` (purego `dlopen`/`dlsym`) — while `yaegi` is the backend that actually interprets Go source. The kind string therefore names a language, not a runtime.

Two constraints make this rename cheap:

- The `plugins` table has no `kind` column. The kind exists only on `loadedPlugin` in memory and is re-derived from the plugin's entry file on every load, so there is no persisted value to migrate.
- The kind never crosses the HTTP boundary as data. It reaches the UI only as a `KIND_BADGE` lookup key in the plugin card partials.

## Goals / Non-Goals

**Goals**

- One kind string that names the runtime: `native` for any shared library loaded through the native loader.
- The registry, the dispatch comments, the hot-load log, and the UI badge all agree on the same five kind strings.
- No migration, no alias, no compatibility shim.

**Non-Goals**

- Splitting the backend into separate Go and C/C++ kinds. The loader is language-agnostic and the ABI is identical; a split would double the registry surface for no observable difference.
- Renaming the Go functions (`loadGo`, `goEngine`, `goFn`). They name the package's internal vocabulary, not the plugin kind, and renaming them is churn with no reader benefit.
- Touching `yaegi`, which already names its runtime accurately.

## Decisions

**D1 — `native`, not `pureGo`.** `pureGo` would be wrong for the C/C++ shared libraries that already run under this backend (`examples/plugins/native/`, `app_data/plugins/roliascan`, `app_data/plugins/madarascans`), and would require a second kind to label them correctly. `native` covers both and matches the README section and the `examples/plugins/native/` path. Alternative considered: keeping `go` and documenting it — rejected, because the collision with `yaegi` is the actual defect and documentation does not fix a wrong key.

**D2 — change the key in place, no alias.** Because the kind is never persisted, accepting `"go"` as a legacy spelling would mean carrying a dead branch forever to serve zero readers. The registry lookup fails loudly for an unknown kind, which is the correct behavior for a value the host itself assigns.

**D3 — rename the badge key, keep the visible label.** `KIND_BADGE.go` becomes `KIND_BADGE.native`; the label stays `Native`, so nothing visible changes. The two partials duplicate the map (`plugin_card.lua`, `plugin_cards.lua`) and must be edited together — a mismatch leaves the badge nil and the card renders without one.

## Risks / Trade-offs

- [A future persistence of `kind` would strand the old spelling] → The kind is derived from the entry file at load time, so any future column would be written after this rename and never see `go`. No action now.
- [The duplicated `KIND_BADGE` map drifts] → Both partials are edited in the same task and verified by a render test.
- [A third-party script filtering on `kind == "go"`] → The kind is not an API response field; no external consumer can observe it.

## Migration Plan

Single release, no data migration, no rollback concern: reverting the commit restores the previous spelling with no state to unwind.

## Open Questions

None.
