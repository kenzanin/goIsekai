---
type: port
title: "ABI extension: Manga.Rating + Chapter.Scanlator"
description: "Added Manga.Rating + Chapter.Scanlator to pkg/types (additive + omitempty). Root cause: Go json.Unmarshal silently drops unknown fields, so plugins parsing rating/scanlator lost the data at the ABI boundary."
tags: [goisekai, plugins, abi, wasm, lua, reimanga, mangadex]
generated: { by: agent/mcp, at: "2026-10-26T15:51:53Z" }
status: stable
---

## Summary

Added two fields to `pkg/types/types.go` that plugins were trying to emit but had nowhere to put:

1. **`Manga.Rating float64 json:"rating,omitempty"`** — reimanga_net's search API returns `rating` in its JSON (main.go:126), and the plugin already parsed it into a local struct field, but `types.Manga` had no Rating field, so the value was silently dropped when the host unmarshalled the plugin's output.
2. **`Chapter.Scanlator string json:"scanlator,omitempty"`** — mangadex Lua plugin fetches `includes[]=scanlation_group` (main.lua:156), but `types.Chapter` had no Scanlator field, so the fetched group was silently dropped.

## Root cause

Go's `encoding/json` `Unmarshal` **silently ignores unknown fields** by default. A plugin that parses a field the host struct doesn't declare loses that data at the ABI boundary with no error, no log, and no way to detect it without manually comparing the plugin's output JSON to the struct definition.

## The fix

```go
// types.Manga
Rating float64 `json:"rating,omitempty"`

// types.Chapter
Scanlator string `json:"scanlator,omitempty"`
```

Both are **additive + omitempty**:
- Old plugins that never set these fields marshal unchanged
- New plugins can set `Rating`/`Scanlator` and the host decodes them
- **No `ContractVersion` bump needed**

## Verification

JSON roundtrip probe (`/tmp/abi_probe/main.go`): old plugin output unchanged (omitempty); new plugin output includes `"rating":4.35` and `"scanlator":"GroupA"`; host `Unmarshal` round-trips both values correctly. `pkg/types` builds clean, `go vet` passes, host binary builds OK.

## What plugins do next

- **reimanga_net**: set `Rating` in the `types.Manga{}` output literal
- **mangadex**: extract scanlation group name from `includes[]=scanlation_group` relationships, set `Scanlator` on each chapter

## Deliberately NOT added

**`Manga.AltTitles`** — alt titles flow through a dedicated enrichment path (`alt_titles` table + `AddAltTitles` in `internal/bridge/enrichment_store.go`). Adding it to the Manga ABI would duplicate that path.

## Root-cause pattern

When a plugin parses a field but it never reaches the host:
1. Check if `types.Manga`/`types.Chapter` has a matching JSON tag
2. If not, the field is silently dropped by `json.Unmarshal` — add it (additive + omitempty = safe)
3. If yes, check if the plugin actually **copies** the parsed value into the output struct
