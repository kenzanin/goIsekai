---
type: finding
title: "atsumaru released_at bug: epoch-millis field was never read"
description: "atsumaru released_at returned zero time for all chapters - allChapters API returns createdAt as epoch millis, but chapters() never read it. Fixed via strconv.FormatInt (int64) -> dateToISO -> parseTime."
tags: [goisekai, plugins, atsumaru, wasm, tinygo, dates, released_at, epoch-millis]
generated: { by: agent/mcp, at: "2026-10-26T15:54:04Z" }
status: stable
---

# atsumaru released_at bug: epoch-millis field was read with the wrong integer width

## Status

Fixed and verified - committed in `68c3a13`.

## Summary

`examples/plugins/wasm/atsumaru` returned `released_at: "0001-01-01T00:00:00Z"` (the Go zero time) for every chapter. Cause: atsumaru allChapters API returns `createdAt` as epoch milliseconds, and the wasm chapters() loop never read it - it built each types.Chapter with only ID/MangaID/Title/ChapterNum.

## Why the zero time leaks through

types.Chapter.ReleasedAt is `time.Time` with no omitempty (pkg/types/types.go:34). An unset field marshals as `0001-01-01T00:00:00Z` - a valid-looking but meaningless date, not an empty field.

## The fix

- main.go chapters(): read cm["createdAt"].(float64), convert with strconv.FormatInt(int64(ca), 10), feed to dateToISO -> parseTime -> c.ReleasedAt.
- strconv.FormatInt is deliberate: TinyGo wasm32 `int` is 32-bit; epoch millis (13 digits) overflows it. Existing numStr/itoa helpers use `int` and would corrupt the value.
- Added parseTime to text.go and dateToISO to text_host.go (atsumaru had neither).

Host pluginutil.ParseDate already handles 12-14 digit strings as time.UnixMilli.

## The trap that cost a rebuild

After rebuilding + installing + reload, still zero time - because GetChapterList is served from plugin_cache, which still held the pre-fix payload. DELETE FROM plugin_cache WHERE plugin_id='atsumaru' + reload was required. Any wasm change to detail/chapters/pages must clear cache.

## Verification

- Before: all 190 chapters released_at: 0001-01-01T00:00:00Z.
- After: real dates e.g. 2026-10-24T04:21:52Z; 0 zero-dates.
- 1manga (0/7), mangabuddy (0/10), lhtranslation (0/7) already emitted valid dates.
- Build: make clean && make build (TinyGo 0.43.0) - never tools/build-wasm.sh.
