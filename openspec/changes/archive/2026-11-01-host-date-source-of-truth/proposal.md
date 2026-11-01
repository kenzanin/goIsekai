# Proposal

## Why

The host tries 15 date layouts in order, resolves relative phrases (`2 days ago`, `yesterday`, `just now`), and infers unix seconds from unix milliseconds by counting digits. It is the same shape as the status and genre problems already removed: the host knows less about a source than the plugin does, and its own comment concedes the limit — *"Slash-only dates never resolve: day-first and month-first are indistinguishable without the site's locale."* A guesser that bails on the ambiguous case produces either a silently wrong timestamp or none at all.

A plugin already knows its source's locale, so it should hand over one machine shape. What a plugin genuinely cannot convert is a phrase that carries no instant — `recently`, `unknown` — and today that phrase is simply destroyed: `date_to_iso` returns `""`, the plugin omits the field, and the chapter row shows nothing.

## What Changes

- The chapter payload's `released_at` accepts **unix epoch seconds** (a JSON number), and an RFC 3339 string while plugins are still being migrated. The host routes on the value's type instead of guessing its shape.
- A string the host cannot read as a moment is **prose, not an error**: it is retained verbatim and the chapter list still decodes. `recently` survives instead of collapsing to "no date".
- A numeric value outside `1970-01-01`..`2100-01-01` is treated as "no date" with a warning, so a millisecond value is never rendered as a year in the fifty-eighth century.
- New `released_text` column, so the wording has somewhere to live.
- The chapter row renders the release date as it does today, and falls back to the retained wording when a chapter has no moment. The template already accepts a string in `c.ReleasedAt`; this wires the value through.
- `text.date_to_iso` is **left untouched and deprecated**. 23 live plugins call it, so changing or deleting it now would blank every chapter date in the app. Its removal ships with the plugin migration.

## Capabilities

### Modified Capabilities

- `plugin-abi`: the chapter payload's release date is epoch seconds or strict RFC 3339; anything else is retained as wording and never guessed.
- `storage`: chapter persistence stores the source's own wording alongside the moment.

### Added Capabilities

None.

## Impact

- `pkg/types/types.go` — `ReleasedText` plus a type-routing `UnmarshalJSON` on `Chapter`; `ReleasedAt` stays `time.Time`, so no downstream consumer changes shape.
- `internal/database` — new column and migration, generated model regenerated, write path carries the wording.
- `internal/bridge` — propagate the wording where chapters are constructed from cache.
- `internal/templates/partials/detail_chapter_row.lua` — fall back to the wording.
- No config change. No existing row is rewritten: chapters already stored keep their `released_at` and simply have no `released_text`.

## Non-goals

- Migrating the 23 plugins to send epoch, and deleting `text.date_to_iso` together with the layout table and relative-phrase arithmetic. That is a separate change; this one only defines and implements the host side it will target.
- Accepting unix milliseconds. Sites that report milliseconds divide by 1000 in the plugin, which keeps the unit unambiguous on the wire.
- Any relative-date arithmetic in the host. Computing `four weeks ago` needs the source's clock, which the plugin has and the host does not.
- Backfilling `released_text` for rows stored before this change.
