# atsumaru released_at bug: the epoch-millis field was never read

## Summary

`examples/plugins/wasm/atsumaru` returned `released_at: "0001-01-01T00:00:00Z"` (the Go
zero time) for every chapter, while `1manga`, `lhtranslation` and `mangabuddy` returned
valid dates. The cause was narrow: the atsumaru `allChapters` API returns `createdAt`
as epoch milliseconds, and the wasm `chapters()` loop simply never read it — it built
each `types.Chapter` with only `ID`/`MangaID`/`Title`/`ChapterNum`.

## Why the zero time leaks through

`types.Chapter.ReleasedAt` is a plain `time.Time` with no `omitempty` (pkg/types/types.go:34).
An unset field therefore marshals as `0001-01-01T00:00:00Z`, not omitted — so "forgot
to set it" looks identical to "the site has no date" in the JSON. The other three plugins
guard with `if iso := dateToISO(...); iso != "" { c.ReleasedAt = parseTime(iso) }`, so
they leave the zero time in place only when the host cannot parse a date.

## The fix (atsumaru only)

- `main.go chapters()`: read `cm["createdAt"].(float64)`, convert with
  `strconv.FormatInt(int64(ca), 10)`, feed it to `dateToISO` → `parseTime` → `c.ReleasedAt`.
- `strconv.FormatInt` is deliberate: TinyGo wasm32 `int` is 32-bit, and epoch millis
  (13 digits, e.g. `1781295156201`) overflows it. The existing `numStr`/`itoa` helpers
  use `int` and would corrupt the value.
- Added `parseTime` to `text.go` and `dateToISO` to `text_host.go` (atsumaru had neither;
  the other plugins already had them).

The host's `pluginutil.ParseDate` already handles 12–14 digit strings as
`time.UnixMilli`, so the string form of the millis parses without any layout change.

## The trap that cost a rebuild

After rebuilding + installing the wasm and hitting `/reload`, the endpoint still returned
the zero time — because `GetChapterList` is served from `plugin_cache`, which still held
the pre-fix payload. `DELETE FROM plugin_cache WHERE plugin_id='atsumaru'` + reload was
required before the new dates appeared. Any wasm change to a detail/chapters/pages
answer must clear the cache or the fix looks like it did not take.

## Verification

- Before: all 190 chapters `released_at: 0001-01-01T00:00:00Z`.
- After: real dates, e.g. `2026-10-24T04:21:52Z`, `2026-09-05T15:21:37Z`; 0 zero-dates.
- `1manga` (0/7), `mangabuddy` (0/10), `lhtranslation` (0/7) already emitted valid dates;
  no change needed there.
- Build: `make clean && make build` (TinyGo 0.43.0) — never `tools/build-wasm.sh`, which
  falls back to std-Go wasip1 and produces a 3.4 MB module that can OOM the wasm runtime.
