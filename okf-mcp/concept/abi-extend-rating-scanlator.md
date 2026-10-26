---
id: abi-extend-rating-scanlator
title: "ABI extension: Manga.Rating + Chapter.Scanlator"
description: "Added two genuinely-missing fields to the plugin ABI contract: reimanga_net parses rating from the site API but had no Manga.Rating field to emit it into, and mangadex fetches scanlation_group but had no Chapter.Scanlator field. Both gaps were silently dropping plugin-parsed data at the ABI boundary because Go json.Unmarshal ignores unknown fields."
type: port
tags: [goisekai, plugins, abi, wasm, lua, reimanga, mangadex, json]
generated:
  at: "2026-10-26T00:00:00Z"
---

# ABI extension: Manga.Rating + Chapter.Scanlator

## Summary

Added two fields to `pkg/types/types.go` that plugins were trying to emit but had nowhere to put:

1. **`Manga.Rating float64 json:"rating,omitempty"`** — reimanga_net's search API returns `rating` in its JSON (main.go:126), and the plugin already parsed it into a local struct field, but `types.Manga` had no Rating field, so the value was silently dropped when the host unmarshalled the plugin's output.
2. **`Chapter.Scanlator string json:"scanlator,omitempty"`** — mangadex Lua plugin fetches `includes[]=scanlation_group` (main.lua:156), but `types.Chapter` had no Scanlator field, so the fetched group was silently dropped.

## Root cause

Go's `encoding/json` `Unmarshal` **silently ignores unknown fields** by default. A plugin that parses a field the host struct doesn't declare loses that data at the ABI boundary with no error, no log, and no way to detect it without manually comparing the plugin's output JSON to the struct definition.

Both gaps were found by auditing what plugins *emit* vs what `types.Manga`/`types.Chapter` *accept*:

- reimanga_net: parses `rating` into a local struct (main.go:126) but never copies it into `types.Manga` output because the field doesn't exist
- mangadex: fetches `scanlation_group` in the API query (main.lua:156) but never extracts the group name because there's no `Chapter.Scanlator` field to put it in

## The fix

```go
// types.Manga
Rating float64 `json:"rating,omitempty"`

// types.Chapter
Scanlator string `json:"scanlator,omitempty"`
```

Both are **additive + omitempty**:
- Old plugins that never set these fields marshal unchanged (no `"rating": 0` or `"scanlator": ""` in their output)
- New plugins can now set `Rating`/`Scanlator` and the host will decode them
- **No `ContractVersion` bump needed** — adding optional fields is backward-compatible

## Verification

JSON roundtrip probe (`/tmp/abi_probe/main.go`):

- Old plugin (no rating/scanlator): `"rating"` and `"scanlator"` are **absent** from marshalled JSON (omitempty confirmed)
- New plugin (sets both): `"rating":4.35` and `"scanlator":"GroupA"` appear correctly
- Host `Unmarshal` into `types.Manga`/`types.Chapter`: `dm.Rating == 4.35`, `dc.Scanlator == "GroupA"` — round-trip works

`pkg/types` builds clean, `go vet` passes, host binary (`cmd/goisekai`) builds OK.

## What plugins should do next

- **reimanga_net**: add `Rating: it.Rating` to the `types.Manga{}` literal in `search()` (line ~210) and `detailImpl()` (line ~246) once it parses the rating field
- **mangadex**: extract the scanlation group name from the `includes[]=scanlation_group` relationships array and set `Scanlator` on each chapter

## Deliberately NOT added

**`Manga.AltTitles`** — alt titles already flow through a dedicated enrichment path (`alt_titles` table + `AddAltTitles` in `internal/bridge/enrichment_store.go`). Only one Lua plugin (mangabay) sets `alt_titles` on the Manga table and it is dropped at the ABI boundary, but the intended mechanism is enrichment, not the source-plugin ABI. Adding it would duplicate an existing path.

## Root-cause pattern

When a plugin parses a field but the value never reaches the host:
1. Check if `types.Manga`/`types.Chapter` has a matching JSON tag
2. If not, the field is silently dropped by `json.Unmarshal` — add it to the struct (additive + omitempty = safe)
3. If yes, check if the plugin actually *copies* the parsed value into the output struct (a local parse field that's never assigned to the output is a second common gap)
