# Design

## Context

`released_at` reaches the host through `pkg/types.Chapter.ReleasedAt`, typed `time.Time` with a JSON tag, decoded in `pluginmanager/api_detail.go`. From there it is `time.Time` all the way down: `database.Chapter.ReleasedAt`, `ReleasedAtString(t) time.Time` for the column, `chapterReleasedAt` for the SQL expression, and `c.ReleasedAt:Format(...)` in `internal/templates/partials/detail_chapter_row.lua`. A JSON value that does not fit `time.Time` fails the whole `[]types.Chapter` decode, which is why the current helper warns callers to omit the field rather than send `""`.

The guessing itself lives in one place: `pluginutil/dates.go` — `dateLayouts` (15 layouts), `relativeUnits`, `parseRelative`, `isDigits`, surfaced to plugins as `text.date_to_iso`. 23 of 41 live plugins call it.

The chapter row already tolerates two shapes: `if type(c.ReleasedAt) == "table" and c.ReleasedAt.Format` and `elseif type(c.ReleasedAt) == "string"`. Prose can therefore use the existing slot without touching the template's contract.

## Goals / Non-Goals

**Goals**

- One machine shape on the wire: unix epoch seconds.
- A value that is not a moment is kept as wording, never guessed and never fatal.
- No shape change for the ~40 packages that already consume `time.Time`.

**Non-Goals**

- Removing `text.date_to_iso` or the layout table; that rides with the plugin migration.
- Unix milliseconds on the wire.
- Relative arithmetic in the host.

## Decisions

### D1 — Epoch seconds, one unit, no auto-detection

Sites that already report numbers (atsumaru: epoch millis) divide by 1000 in the plugin. Inferring the unit from digit count is the guessing this change removes; making it optional would reintroduce it.

### D2 — Route on the value's JSON type, not on its shape

A custom `UnmarshalJSON` on `types.Chapter` decodes into a shadow struct whose `released_at` is `json.RawMessage`, then dispatches: number → epoch → `time.Time`; string → RFC 3339 → `time.Time`; any other string → `ReleasedText`. A second field on the input would force every plugin to learn a new key; one key with a clear type rule does not.

### D3 — `ReleasedText` is internal

`ReleasedText string \`json:"-"\`` — it is an output of routing, not something a plugin sets. This keeps the ABI surface at one name while giving the template and the column a field to read.

### D4 — A range guard, not a unit heuristic

Numeric values outside `1970-01-01`..`2100-01-01` are treated as "no date" and logged as a warning. This is the one place a heuristic is unavoidable, and it is a sanity bound, not a format guess: without it a millisecond value renders as year 58600, which is worse than showing nothing. The warning makes the mistake visible instead of silent.

### D5 — `released_text` column, migration index 32

`released_at` keeps its `TIMESTAMP` type and its `NULL`-means-unknown rule, so no stored row is rewritten. `released_text TEXT` is added at the next free index (31 is `addedAtMigration`). The jet-generated model under `internal/database/.gen/` is regenerated rather than hand-edited.

### D6 — Prose reuses the existing render slot

The chapter row already accepts a string in `c.ReleasedAt`. The bridge exposes the wording in that slot when the moment is absent, so `formatDate` and the `0001` guard stay untouched. No new template branch is needed.

### D7 — `text.date_to_iso` is deprecated, not changed

Making it strict would blank every chapter date in the app until 23 plugins are fixed; deleting it would break them outright. It keeps its current behaviour and gains a deprecation note. Its removal, together with `dateLayouts`, `relativeUnits` and `parseRelative`, is the deliverable of the follow-up plugin change.

## Risks / Trade-offs

- **A plugin sending prose loses its date.** Correct by construction: it had no date. The wording is at least preserved.
- **Millisecond senders silently lose dates** until migrated. Mitigated by the D4 warning; the plugin fix is one division.
- **A custom `UnmarshalJSON` on a widely used type** can break unrelated fields if the shadow struct drifts. Mitigated by a round-trip test that decodes a fully populated chapter.
- **`released_text` can go stale as wording** (`4 weeks ago` stays that forever). Accepted: the alternative — arithmetic in the host — is the behaviour being removed. Sources that can compute a moment should send epoch.

## Migration Plan

1. Add the column (index 32) and regenerate the jet model.
2. Ship the host routing, propagation and render fallback. Existing plugins keep working unchanged, because their RFC 3339 output is still accepted.
3. Follow-up change: plugins send epoch, then `text.date_to_iso` and the layout table are deleted from the host.

## Open Questions

- Should the row label prose (`released recently`) or print it bare in the date slot? Defaulting to bare, matching how the date itself is printed today; revisit if it reads ambiguously.
