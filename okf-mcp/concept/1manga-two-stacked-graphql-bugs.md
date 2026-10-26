---
id: 1manga-two-stacked-graphql-bugs
title: 1manga search returned 0 results - two stacked GraphQL syntax bugs
description: x: "mn03" was emitted as a GraphQL string instead of the mn03 enum (API 400), and the query ended with image}}} instead of image}} (syntax error); each bug masked the other so fixing one alone never worked.
type: finding
tags: [goisekai, plugins, 1manga, wasm, graphql, tinygo, search]
generated:
  at: "2026-10-26T00:00:00Z"
---

# 1manga: two stacked GraphQL syntax bugs

## Summary

The 1manga wasm plugin (`examples/plugins/wasm/1manga`) search was silently
returning 0 results. Two independent bugs were stacked on each other:

1. **`x: sourceID` quoted as a string.** The source id `mn03` was emitted as a
   GraphQL *string* (`x: "mn03"`) instead of an *enum* value (`x: mn03`).
   The MangaHub API validates this and returns HTTP 400:
   `Argument "x" has invalid value "mn03". Expected type "MangaSource", found "mn03".`
2. **Extra closing brace.** The original search query ended with `image}}}`
   (three `}`) instead of `image}}` (two). That alone is a GraphQL syntax
   error: `(1:99) Expected Name, found <EOF>`.

## Why it took so long to find

The two bugs masked each other. Fixing #2 (the brace) first did nothing
because #1 (the quoted `x`) still produced a 400. Fixing #1 first was
invisible because #2 produced a *syntax* error before the API ever got to
validate the `x` argument. Only fixing both — and reading the API's actual
400 body instead of guessing from the status code — got search working.

The API response body was the key diagnostic that had been missing: the
plugin only logged the HTTP status, not the `{"errors":[{"message":...}]}`
payload. Surfacing the body in the log (`GraphQL status 400, body: ...`)
was what turned a black-box "non-2xx" into a readable answer.

## The fix (in `main.go`)

All four GraphQL call sites build the source id unquoted:

```go
// search
sb.WriteString(`{search(x: `)
sb.WriteString(escapeGQL(sourceID))   // → "mn03" with no surrounding quotes
sb.WriteString(`, q: "`)
...
sb.WriteString(`) {rows {title, slug, image}}}`)  // exactly two closing braces
```

`detail` / `chapters` / `pages` switched from the literal `x: sourceID`
(which the Go compiler never flagged because `sourceID` was being
concatenated as text) to `x: ` + `sourceID`, matching the reference Lua
plugin which interpolates `SOURCE_ID` the same way.

An empty search query now sends `q: " "` (a single space) instead of
`q: ""` so a genre-only / "browse popular" search still matches rows.

## Build rule

Build **only** via the plugin's `Makefile` (`make clean && make build`),
never by hand. The Makefile pins TinyGo 0.43.0 + `wasm-opt -O4` and the
result is ~1.2 MB. A manual std-Go wasip1 build is ~3.4 MB and can OOM the
host's linear-memory budget (`malloc failed for input`), which is a
*third*, separate failure mode that also produces an empty search list.

## Live verification

- `/api/sandbox/plugins/1manga/search?q=isekai` → real result set
  (e.g. `mondaijitachi-ga-isekai-kara-kuru-sou-desu-yo`).
- `detail` and `chapters` for a valid slug both populate.
- `/view/search?pluginID=1manga&genre=&q=isekai` renders cards (HTTP 200).
- `make lint` (go vet + gofmt) and `go test .` pass.

## Notes

- Reference: `examples/plugins/lua/1manga/main.lua` (the Lua original) —
  its `SOURCE_ID` interpolation and brace structure are the source of truth
  for the Go port.
- The `fetchAccessKey` / `graphqlQuery` retry-and-drop-key logic in the Go
  wasm matches the Lua; the divergence was purely in how the query string
  was assembled.
- Debug logging added during diagnosis was stripped from the final
  `hostcall.go` / `main.go` — the shipped `httpPost` does not log the body.
