## 1. Shared title normalization

- [x] 1.1 Move the title normalization rule out of `internal/database/duplicates_match.go` into `internal/pluginutil` as an exported function, and make the duplicate detector call it instead of keeping a private copy. Verify `CGO_ENABLED=0 go build ./...` succeeds and the duplicate-detection tests still pass (`go test ./internal/database/ -run Duplicate -count=1`).
- [x] 1.2 Register the function as a `host.text.*` helper next to the existing text helpers for both the Lua and JS runtimes, so a plugin can normalize a title without reimplementing the rule. Verify with a case added to `internal/pluginmanager/host_helpers_test.go` that asserts case, punctuation, and repeated spacing all collapse to the same string.

## 2. Source script fixes

- [ ] 2.1 In `examples/info/mangadex/main.lua`, delete the arbitrary-language fallback from the summary/title picker so only the preferred languages are ever returned, and return nothing when none is present. Verify by fetching enrichment for a title whose record has no preferred-language description and confirming no summary is stored.
- [ ] 2.2 In the same script, verify the search result against the searched title using the shared normalizer before using it: accept only when the candidate's own title or one of its alternative titles matches, and return no items when nothing matches. Verify a nonsense title returns zero items and a real title returns its own record.
- [ ] 2.3 In the same script, strip promotional link blocks and trailer URLs from a summary before returning it. Verify a title known to carry a "Links:" block stores a summary containing no bare URL.
- [ ] 2.4 Apply the same three fixes to `examples/info/mangaupdates/main.lua`. Verify with the same three checks against a title that source knows.
- [ ] 2.5 Re-install the fixed scripts into the runtime info directory with `just install-info mangadex` and `just install-info mangaupdates`, then confirm the runtime copies match the examples byte for byte.
- [ ] 2.6 Run `just lint-lua` and confirm the edited scripts produce no warnings.

## 3. Deterministic precedence

- [ ] 3.1 Add an optional precedence and an optional enabled flag to the enrichment declaration type in `pkg/types`, documented as "lower runs first" and "absent means enabled, least preceding". Verify a decode test shows absent fields fall back to those defaults and declared fields survive a `PluginMeta` round trip.
- [ ] 3.2 Extend the provider interface in `internal/enrich` with the precedence and enabled state, and make the registry keep a single order sorted by precedence then source identifier, used by both the unfiltered and the kind-filtered catalog as well as by the fetch. Verify a registry test that registers the same providers in shuffled orders and asserts identical catalog output, filtered and unfiltered, across runs.
- [ ] 3.3 Make the info scripts load in a sorted order so that two scripts declaring the same source identifier resolve to a stable winner, since the first claim of an identifier currently wins and load order is randomized. Verify by declaring the same source identifier in two scripts and asserting the same winner across repeated runs.
- [ ] 3.4 Have the plugin manager pass the declared precedence and enabled flag into provider registration and skip disabled declarations entirely. Verify a test asserting a disabled declaration produces no catalog entry and is never fetched.
- [ ] 3.5 Carry precedence and enabled through the bridge's catalog entry so the catalog a caller reads exposes the order it will fetch in, and exclude disabled sources there too. Verify a bridge test asserting the order follows precedence and that a disabled source appears in neither the full nor the kind-filtered catalog.

## 4. Multi-source fetch

- [ ] 4.1 Replace the first-winner fetch in `internal/enrich` with one that gathers a kind's items from every enabled provider supporting it, keeping each item tagged with its own source and preserving the existing tolerance for a provider that errors or times out. Verify a registry test in which two providers answer the same kind, both results are returned, and a third failing provider does not suppress them.
- [ ] 4.2 Change `AppService.FetchEnrichment` to store each source's items with its own source label, one store call per source, instead of storing only the winning source's items. Verify a bridge test in which a two-source fetch yields categories labelled with both sources, and that a value reported by both sources is stored once under the higher-precedence label.
- [ ] 4.3 Change the author store path so only the highest-precedence non-blank author result is stored, rather than joining every source's authors into one string. Verify a bridge test in which two sources report different authors stores only the higher-precedence one.
- [ ] 4.4 Keep the explicit single-source path working for callers that name one source. Verify a fetch limited to one source stores only that source's items and leaves the other source's data untouched.

## 5. Category normalization

- [ ] 5.1 Resolve fetched categories through the alias index the host already uses for plugin genres before storing them, reusing that index rather than building a second one, and keep a category the map does not know exactly as the source sent it. Verify a test in which two sources report spellings the map treats as one genre stores a single canonical row.
- [ ] 5.2 Confirm the category toggle stores the spelling the stored category uses, so toggling a normalized category on and off is stable and does not create a second genre entry. Verify by toggling a category the alias map rewrote and confirming the stored genre override matches the stored category.

## 6. Fetch control

- [ ] 6.1 Confirm the detail view's enrichment control already matches the new requirement: a single collapsed action that posts only the manga title, with no kind or source selection, and per-row source labels on the stored items. Expected outcome is no template change; if any selector or missing label is found, add or align it. Verify by submitting the control from the manga detail view and observing items from more than one source, each labelled.

## 7. Sources

- [ ] 7.1 Add `examples/info/kitsu/main.lua` declaring a `kitsu` source at a lower precedence than MangaDex, implementing titles, summaries, categories, authors, and related, resolving by the Kitsu identifier MangaDex reports when one is available and falling back to title search otherwise, and applying the same match-verification and language rules as the other scripts. Verify by fetching enrichment for a title MangaDex reports a Kitsu identifier for and confirming Kitsu data is stored under the `kitsu` label alongside MangaDex's.
- [ ] 7.2 Confirm the new source is discovered without any host change or rebuild. Verify `just install-info kitsu` followed by a catalog read lists `kitsu` with its kinds, and that it never appears in the manga source list.
- [ ] 7.3 Demote MangaUpdates from the runtime info directory, leaving the example copy as the shareable artifact. Verify a catalog read no longer lists `mangaupdates` and an enrichment fetch still succeeds. Record that this is reversible with `just install-info mangaupdates`.

## 8. Verification

- [ ] 8.1 Run `just test` and confirm the package suites pass, then run `just check` and confirm formatting, modernization, and linting are clean.
- [ ] 8.2 Restart the host twice with debug logging and trigger the same enrichment fetch each time. Verify the fetch log lists the sources in the same, declared, precedence order on both runs, and that the stored items keep their per-source labels.
- [ ] 8.3 Fetch enrichment for a title that previously produced a wrong-language synopsis and confirm the stored summary is now in a preferred language and free of link blocks, while a title no source matches stores nothing.
- [ ] 8.4 Fetch a title both sources know and confirm the category list shows one entry per genre rather than one per source spelling, and that an alt title, alt summary, and related item from each source is individually removable.
