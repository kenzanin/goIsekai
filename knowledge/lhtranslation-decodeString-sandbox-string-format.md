---
type: finding
title: "lhtranslation decodeString - sandbox passes a bare string, not JSON-encoded"
description: "The sandbox /chapters/{id} endpoint passes a bare manga id (not a JSON-encoded string), so decodeString must fall back to the raw arg; without the fallback chapters returned an empty manga_id."
tags: [goisekai, plugins, lhtranslation, wasm, tinygo, sandbox, decodeString]
generated: { by: human/kenzanin, at: "2026-10-28T07:10:59Z" }
id: "lhtranslation-decodeString-sandbox-string-format"
---

# lhtranslation: decodeString sandbox string format

## Problem

`examples/plugins/wasm/lhtranslation` chapters returned an empty
`manga_id` for every chapter, so the reader could not resolve the manga
from a chapter row.

## Root cause

There are two ways a manga id reaches a wasm plugin:

- The **Lua/bridge** path passes a **JSON-encoded** string (`"slug"` — with
  surrounding quotes), which is what `json.Unmarshal` into a Go `string`
  expects.
- The **sandbox endpoint** `/api/sandbox/plugins/lhtranslation/chapters/{id}`
  passes a **bare** string (`slug` — no quotes).

A `decodeString` helper written only for the JSON path returns `""` for the
bare form: `json.Unmarshal([]byte("slug"), &s)` fails on the unquoted input,
and if the helper returns the (empty) unmarshalled value instead of falling
back to the raw arg, the id silently becomes empty.

## Fix

`helpers.go` `decodeString` now falls back to the raw arg:

```go
func decodeString(arg string) string {
	var s string
	if err := json.Unmarshal([]byte(arg), &s); err != nil {
		return strings.TrimSpace(arg)
	}
	return s
}
```

This keeps the JSON-encoded path working (bridge/Lua) while also accepting
the bare-string sandbox path. The same helper shape is reused by atsumaru
and mangabuddy, which face the identical two callers.

## Verification

- `/api/sandbox/plugins/lhtranslation/chapters/<slug>` now returns a
  populated `manga_id` per chapter.
- Search / detail / pages were already working; only chapters was affected.

## Note

This is a *contract* quirk worth remembering for any future wasm plugin:
never assume the arg is JSON-encoded. Always route ids through a
`decodeString` that tolerates both forms.
