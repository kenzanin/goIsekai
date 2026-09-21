## Context

See proposal.md for motivation. The current state that shapes the approach:

- A provider is registered by `Manager.ensureLoaded` when its declaring plugin is first invoked, and info scripts are force-loaded by `ensureInfoLoaded` (`internal/pluginmanager/enrichment.go`). That function collects pending ids by ranging a Go map, so the registration order differs between runs. `Registry.order` records registration order, `Catalog("")` returns it verbatim, and `FetchFirst` walks it as the precedence ladder. Precedence is therefore accidental today.
- `Catalog` has a second order path: a kind filter iterates `byKind[kind]`, which is its own append-ordered slice. Both paths must be ordered, not just one.
- `FetchFirst` is documented as stopping at the first source that answers a kind, "so one source owns a kind instead of every source merging into it". The caller (`AppService.FetchEnrichment`) then stores the winning source's items and labels every row with `items[0].Source`.
- Storage is already per source and already dedupes: `alt_titles`, `alt_descriptions`, `manga_categories`, and `manga_related` each carry a `source` column, each has `UNIQUE(manga_row_id, <value>)`, and each adder uses `INSERT OR IGNORE`. Identical values from two sources collapse to one row and the first writer's label survives. The detail view renders every stored row with a `via <source>` badge, so multi-source data needs no new rendering.
- `authors` is the exception: the manga row holds one string, and the fetch currently joins every author item from the winning source into it, comma-separated.
- A declaration is `PluginMeta.EnrichmentProviders` (`pkg/types/types.go`), a list of `{id, name, kinds}`.
- The detail view's fetch form posts only the title; the handler accepts an optional `source` form field that no template sends.
- Title normalization already exists as `normalizeTitle` in `internal/database/duplicates_match.go` (unexported): lowercase, drop non-alphanumerics, collapse whitespace. `internal/pluginutil` is a leaf package with no internal imports, and `host.text.*` helpers are registered from it.

## Goals / Non-Goals

**Goals:**

- Precedence is read from declarations and resolved once, so it cannot depend on map iteration, discovery order, or load order.
- Every enabled source that supports a kind contributes its items for that kind, each keeping its own label, without a database schema change.
- A fetched record belongs to the title that was asked about, and a summary is in a language the source prefers.
- Adding, disabling, or removing a source stays a data operation: edit a script, no host rebuild.

**Non-Goals:**

- Per-kind precedence. One precedence per declared source; a source that is good at one kind and bad at another is fixed by fixing its script, not by a second axis of configuration.
- Selecting a kind or a source in the UI.
- Storing a snapshot of the plugin's original detail, or making reset work offline. See Open Questions: that is a storage change and belongs in its own change.
- Background or scheduled enrichment. Fetch stays manual, which is what makes the extra invocations acceptable.
- Any database schema change or new Go dependency.

## Decisions

**Precedence is declared in the manifest, not in config.**
Alternatives: putting it in `goisekai.ini` (rejected: nothing else about a source is configured there, and a future third-party distribution would have its defaults clobbered by every plugin update) and using the declaration's array position as the precedence (rejected as a primary mechanism because reordering an array to change behavior is invisible in review, but it is why the default must be "least preceding" rather than "unknown"). The manifest is right because in this project the user authors the scripts and `examples/info/<id>/` must carry a sensible default when copied to another install.

**`enabled` is an explicit flag, and precedence never means "off".**
Alternative: precedence `0` meaning disabled. Rejected because it puts a magic value below the lowest real precedence, so a reader cannot distinguish "off" from "highest priority", and every comparison needs a special case.

**Ordering is a property of the registry, resolved at registration time.**
Sort by `(precedence, id)` and keep the sorted slice as the single order both `Catalog` paths and the fetch read. Alternatives: sorting inside the fetch (leaves the catalog unordered, so the order a user sees and the order a fetch uses could disagree) and sorting at each call site in the bridge (duplicates the policy and lets call sites drift). Tie-breaking by id makes the order total, so no result ever depends on arrival order.

**Determinism cannot come from sorting the discovery or load order.**
Info scripts could be sorted before loading, but a source plugin registers its providers at first invocation, which is demand-driven and therefore cannot be ordered by discovery. Imposing order at registration covers both cases with one rule.

**Multi-source fetch: every enabled provider answers every kind it supports.**
Precedence stops being a gate and becomes a ranking. It still decides two observable things: the order items are stored in, and therefore which source's label a value shared by two sources keeps; and which source owns the single-valued `authors` value. Iterating in precedence order and storing as we go gets both for free, because the adders already ignore a duplicate value and `authors` can simply stop at the first non-blank result.
Alternative considered: fetching only from enabled sources but keeping first-winner storage, which would make the whole change invisible to the user. Rejected: it does not address the reported problem of one source's wrong synopsis being the only one shown.

**Storage call sites move from one adder call to one per source.**
No new database code is needed: every adder already takes a `source` argument and ignores duplicates. Authors keep the single-value path and take the first non-blank result in precedence order instead of joining all sources into one string.

**The fetch panel requirement is removed and re-added under a truthful name, not modified.**
A `MODIFIED` block must reproduce every scenario the current spec has, and one of them asserts that the source selector lists only sources for the selected kind, which is the opposite of the new behavior. Removing the requirement and adding "Enrichment fetch control" is the honest form of that change.

**Title normalization is promoted to `internal/pluginutil` and exposed to plugins.**
The comparison happens inside the plugin VM, which cannot call the host's Go helper, so verification has to be expressed in the plugin language. Three scripts each needing it means three copies of the same rule drifting from the duplicate detector's rule. One exported function in the leaf package plus a `host.text.*` registration keeps a single implementation, and the existing duplicate detection keeps using it rather than keeping a private copy.
Alternative considered: writing the rule in Lua in each script. Rejected as three copies for one concept.

**Match verification and language selection live in the source, not the host.**
The host only receives the final items; it never sees the candidate set the source rejected, and a summary item carries no title to compare against. So the source must decide. The host's contribution is the shared normalizer and the contract in the spec.

**A missing preferred-language summary yields no summary.**
Deleting the arbitrary-language fallback is the fix for the reported symptom; keeping it and labelling the language in the UI would leave the wrong synopsis on screen. The cost is that the alt-summary pool shrinks for records whose only description is in an unrequested language.

**Every fetched item is stored, and the user prunes afterwards. There is no selection step.**
A fetch writes every returned item into the matching alternative table and stops there; choosing what fits is done per row afterwards through the promote and remove actions the detail view already has. This is the whole reason the change needs no new storage: the alternative tables already carry a source label and already ignore a duplicate value, and the panel already renders each row with a promote and a remove control.
Alternatives: staging fetched items until the user confirms each one, which needs a new persisted pending state, a new action per kind, and a way to expire an unconfirmed batch, in exchange for saving the user some deletions. Rejected because the per-row pruning it replaces already exists, so the staging step would add a lifecycle to save clicks that the removal control already covers.

**Summary hygiene strips link blocks at the source.**
Stripping at render time would leave the spam in the database and in any future export. The stored value should be the value the user wants.

**Fetched categories are resolved through the genre alias map before storage.**
The host already rewrites plugin genre spellings to canonical names through a config-driven alias map, and it is visible from the detail view, but fetched categories bypassed it and were stored exactly as each source wrote them. Two sources naming the same genre therefore produced two entries. Running the same alias index over fetched categories reuses a map the user already owns and lets the existing uniqueness constraint collapse the cross-source duplicate with no new query.
Alternatives: normalizing when the category is rendered (leaves the duplicate rows in the database, and the stored genre override keys on the exact string, so the two spellings would still toggle independently) and normalizing inside each script (three copies of a map that lives in the user's config and cannot be reached from a script).

**Kitsu resolves by the MangaDex cross-source id when one is available.**
MangaDex returns an `attributes.links` map including a Kitsu id, verified to resolve to the same series, so the second provider can skip title search entirely when the MangaDex script has already looked the title up. Falling back to title search keeps Kitsu usable on its own.

## Risks / Trade-offs

- One plugin invocation per source per kind instead of stopping at the winner → acceptable because the fetch is manual and there are about three sources; the existing per-source error tolerance must be preserved so a slow source cannot fail the whole fetch.
- Merging all sources surfaces noise from a weak source → the match-verification and language rules must land first, otherwise this change just displays more wrong data. A user who dislikes a source disables it in that source's declaration.
- Storing everything without a selection step means more rows to prune, and more again once several sources answer → the rows already carry the source that produced them and each is removable on its own, so the work is bounded and the user can see where an entry came from. If the panel becomes noisy, the next step is grouping or ordering the rows by source, not adding a confirmation step; that would reintroduce the pending state this design avoids.
- The stored `via <source>` label of a value two sources share is the first writer's, so precedence now decides labels, not just order → intended, and the reason precedence must be declared rather than accidental.
- Existing installations declare no precedence or enabled flag → defaults keep every current source working, but their mutual order falls back to the id tie-break, which may differ from today's winner for `authors`. Today's winner is a coin flip, so no reliable behavior is lost.
- Two implementations can still drift if the scripts hand-roll the rule instead of calling the host helper → the spec states the rule and the scripts should call the helper; a drift here degrades verification rather than corrupting data.
- Removing the language fallback can produce no summary where a wrong-language one used to appear → intended; this is the reported bug, and the alt-summary pool is allowed to shrink.
- Normalizing a category replaces the source's own wording with the canonical name, so a recognized tag no longer reads the way the source wrote it → intended, and a wrong mapping is corrected in the user's alias config rather than in code. Unrecognized categories are still stored verbatim, so no information is lost.
- The script fixes are not reachable from Go tests, since they run in a plugin VM → verification needs a live fetch against a known title, so the tasks include a manual check rather than only unit tests.
- Removing the MangaUpdates runtime script reduces coverage until a replacement is installed → it is one reversible data step, and the example copy already exists.

## Migration Plan

No database migration and no config migration. Deployment order matters only for the user-visible result: the script fixes should land before the host starts merging every source, so the additional sources do not amplify unverified data. The host change and the script change are each independently safe to ship.

Rollback: the host change is a revert; disabling a single source in its own declaration is a per-source rollback that needs no rebuild; removing the added source's directory uninstalls it. The one-time removal of the MangaUpdates runtime script is reversible by re-installing it from the example copy.

## Open Questions

**Snapshot of the plugin's original detail (deferred, belongs in its own change).** The user's model is that adding a manga to the library stores the plugin's detail into its own table, the displayed detail is an accumulating copy of it plus fetched additions plus user edits, and reset re-derives the displayed detail from the stored original. The gap is real: today the plugin's original title and description are not stored anywhere, `UpsertManga` overwrites them on every sync unless an override flag is set, and the existing reset restores nothing by itself. It deletes the alternative rows and clears the override flags, but the custom value stays on screen until the next sync re-fetches it, which needs the network and a live plugin. That reset also deletes `read_history`, which is a surprising side effect for a detail action.

Nothing about that model is required for correct enrichment, and it changes `storage`, so it is not part of this change. When it is planned, the minimal shape is a handful of original-value columns on `mangas` populated only on the first insert, with the reset rewriting the displayed value from them: the "display layer" the model describes already exists as the manga's own columns plus the labelled alternative rows, so a second table would duplicate those values and add a second source of truth.

**Whether a disabled source should stay visible in the catalog marked disabled.** Hidden is specified for now. Showing it would be a presentation change that does not affect precedence or fetching, so it can be decided later without touching the approach or the task breakdown.
