## Why

Enrichment picks one source per kind from a precedence list the host builds by iterating a Go map, so which source wins a kind can change on every restart. On top of that, the shipped sources accept an unverified search hit and fall back to a summary in an arbitrary language, so a library manga can be given another series' synopsis in a language nobody asked for. The user cannot see which source was used, and there is no way to express that one source is more trustworthy than another.

## What Changes

- Enrichment source declarations gain an optional numeric precedence and an `enabled` flag. Sources that declare neither keep working, at the lowest precedence, enabled.
- Precedence becomes deterministic. The host orders providers once, tie-breaking by source id, so a restart can never reshuffle which source wins.
- A fetch gathers every enabled source that supports the kind instead of stopping at the first one that answers. Alt titles, alt summaries, categories, and related manga accumulate from all of them and keep their per-source labels. A fetch stores whatever a source returns and never asks the user to approve items one by one; choosing what fits stays a per-row promote or remove afterwards.
- Authors stay single-valued: the highest-precedence source that returned a non-blank author wins, and only that source's value is stored.
- Sources stop returning arbitrary-language summaries, stop accepting a search hit that does not match the requested title, and stop storing raw link and trailer spam inside a summary.
- Fetched categories pass through the genre alias map the host already applies to plugin genres, so the same genre reported by two sources is stored once under one spelling instead of appearing twice.
- A Kitsu enrichment source is added, resolving by the MangaDex cross-source id when one is available rather than by title search.
- The MangaUpdates runtime script is demoted to example-only.
- The per-kind and per-source fetch selectors are dropped from the requirements: with every enabled source fetching, the panel stays a single collapsed control.

## Capabilities

### New Capabilities

None. This change alters existing enrichment behavior and the data shipped with the host; it introduces no new capability.

### Modified Capabilities

- `enrichment`: precedence declaration and deterministic ordering, multi-source fetch and per-source storage, author source selection, match verification and summary quality for fetched records, and the shape of the fetch control.

## Impact

- `internal/enrich`: registry ordering and the fetch strategy.
- `internal/pluginmanager`: deterministic script load order and passing declared precedence into provider registration.
- `internal/bridge`: per-source storage after a multi-source fetch.
- `pkg/types`: the enrichment declaration fields.
- `internal/templates/partials/detail_alt.lua`: fetch control and source labelling.
- `app_data/info/mangadex/main.lua` and `app_data/info/mangaupdates/main.lua`: match verification, language selection, and summary hygiene.
- `app_data/info/kitsu/main.lua` (new) and `examples/info/*`: the added source and the example copies.
- No database schema change, no new Go dependency, no host rebuild needed to change a source.
