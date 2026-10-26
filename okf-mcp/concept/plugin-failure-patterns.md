---
id: plugin-failure-patterns
title: Recurring plugin failure patterns in goIsekai
description: >
  Catalog of the six plugin failure modes that resurface across goIsekai
  ports and sessions, why each one is silent (produces no error), and the
  fix. A plugin author matches a symptom to its known root cause instead
  of re-deriving it from code.
type: Concept
tags:
  - plugins
  - debugging
  - wasm
  - lua
  - abi
  - regex
  - released-at
  - cache
generated:
  at: "2026-10-26T00:00:00Z"
---

# Plugin Failure Patterns

Every pattern below fails **silently** — the wrong-shape unmarshal, the
regex that matches nothing, the unread field all return an empty/zero
result **with no error**, so the failure surfaces only as an empty list
or a `0001-01-01` date in the UI. This is why the same bugs recur: they
are cheap to introduce and invisible until a human looks at the output.
The host cannot distinguish "plugin returned empty" from "plugin
unreachable" and defaults to cache (see `library.go` `cachedMangaFallback`).

---

## Pattern 1 — regex.find_all shape

- **Symptom**: search/detail/chapters/pages returns fewer results than expected or empty, but no error.
- **Silent-failure signal**: `regex.find_all` with a one-capture-group pattern decodes to nothing with no error.
- **Root cause**: the host renders a one-capture-group row as a bare string via `wasmRegexRow` (`internal/pluginmanager/wasm_host_helpers.go`, `case 1: return row[0]`), so a plugin written to expect `[][]string` rows gets a string and a multi-row array gets mixed types; one of those decodes fail silently. Host RE2 `[^>]*` does **not** cross newlines, so `data-src="[^"]*\n[^"]*"` matches nothing — yet another silent no-match.
- **Fix**: parse `[]json.RawMessage` rows → `[]string`, index capture `[0]` (see lhtranslation `firstCapAll`); use `[^"]*` (which does cross newlines in RE2) instead of `[^>]*` for attribute values; `(?:...)` grouping when a whole-match row is wanted.
- **References**: `okf-mcp/concept/three-lua-plugins-converted-to-tinygo-wasm`, `okf-mcp/concept/odin-wasm-plugin-port-candidates`, `internal/pluginutil/regex.go` `RegexFindAll`, `internal/pluginmanager/wasm_host_helpers.go` `wasmRegexRow`.

---

## Pattern 2 — released_at zero-time

- **Symptom**: all chapters show `0001-01-01T00:00:00Z` or no `released_at` at all.
- **Silent-failure signal**: `types.Chapter.ReleasedAt time.Time` has **no `omitempty`**, so an unset field emits `0001-01-01T00:00:00Z` — a valid-looking but meaningless date, not an empty field. On 32-bit wasm, `int` overflows 13-digit epoch-millis.
- **Root cause**: the plugin reads the release field with the wrong type or wrong conversion. atsumaru reads `createdAt` as `float64` epoch-millis and must pass `strconv.FormatInt(int64(ca), 10)` → `text.date_to_iso` → `time.Parse` (a Go `int` on wasm32 overflows). lhtranslation/mangabuddy must parse the HTML/`updated_at` string through the same `dateToISO` bridge.
- **Fix**: read the native release field; on wasm32 convert epoch-millis via `strconv.FormatInt` (not `int`); always run it through `text.date_to_iso` then `time.Parse(time.RFC3339, ...)`; never emit `released_at` when the source field is absent.
- **References**: `okf-mcp/concept/atsumaru-released-at-epoch-millis`, `pkg/types/types.go` (`Chapter.ReleasedAt` — no `omitempty`), `internal/pluginutil/dates.go` (`DateToISO`/`DateToISONow` digits 9–11 → unix sec, 12–14 → `UnixMilli`).

---

## Pattern 3 — Lua regex not portable to RE2

- **Symptom**: a regex that works in the Lua plugin returns no matches in the wasm port.
- **Silent-failure signal**: RE2 `.` does **not** match newline; Lua's `.` does. `[^>]*` in RE2 stops at `>`, not at newline; `[^"]*` crosses newlines.
- **Root cause**: copying a Lua regex pattern verbatim into `hostCall("regex.find", …)` (which dispatches to RE2) and assuming `.`/`[^>]` behave identically.
- **Fix**: use `(?s)` for whole-dot matches, or `[^"]*` for attribute values (which crosses newlines in RE2); verify every multi-line scrape with a known-good input.
- **References**: `log.md` (~line 488, Lua vs Go regex), `examples/plugins/wasm/*/text_host.go`.

---

## Pattern 4 — stale plugin_cache

- **Symptom**: the UI shows yesterday's data or `released_at`/`manga_id` is unchanged after a plugin rebuild; detail is suspiciously fast (~13 ms).
- **Silent-failure signal**: `plugin_cache` (sqlite, `UNIQUE(plugin_id, manga_id, function_name)`) caches plugin responses with TTLs (GetMangaDetail 24h / GetChapterList 168h). A cache hit returns the old payload silently.
- **Root cause**: after editing plugin output, the cache was not cleared before reload, or the wasm binary on disk is new but the host served the cached row.
- **Fix**: `DELETE FROM plugin_cache WHERE plugin_id='<id>';` then reload (`POST /api/sandbox/plugins/<id>/reload`). Always clear after a wasm rebuild.
- **References**: `docs/HANDOVER.md` (TTLs), `internal/database/schema.go` (`plugin_cache`), `internal/pluginmanager/api_detail.go:39` cache layer.

---

## Pattern 5 — wasm toolchain drift / silent fallback

- **Symptom**: `main.wasm` is ~3.4 MB and OOMs the wasm runtime, or builds with std-Go `wasip1` instead of TinyGo.
- **Silent-failure signal**: the host falls back to a std-Go wasip1 build (`GOOS=wasip1 GOARCH=wasm CGO_ENABLED=0 go build`) which produces a 3.4 MB module that exhausts the runtime's memory; TinyGo produces ~1.2 MB. No error, just a crash/hang.
- **Root cause**: running `go build` directly instead of the per-plugin Makefile; or the Makefile's TinyGo 0.43.0 path not being used.
- **Fix**: build **only** via the per-plugin `make clean && make build` (TinyGo 0.43.0, `-target wasi -buildmode=c-shared` + `wasm-opt -O4`). Never `go build -o main.wasm` or `tools/build-wasm.sh` (deleted).
- **References**: `okf-mcp/concept/wasm-backend-sizes-and-tinygo-port`, each `examples/plugins/wasm/<id>/Makefile`.

---

## Pattern 6 — sandbox decodeString bare-string

- **Symptom**: sandbox `/api/sandbox/plugins/<id>/chapters/{mangaId}` passes the ID through but the plugin receives `""`, so `manga_id` comes back empty.
- **Silent-failure signal**: the sandbox passes a **bare string** (e.g. `slug`), but the wasm port's `decodeString` tries `json.Unmarshal` first and returns the empty decoded value on failure without falling back to the raw arg.
- **Root cause**: `decodeString(arg)` returns `""` when `json.Unmarshal([]byte(arg), &s)` fails on a bare (non-JSON-quoted) string, before the fallback `return arg` runs.
- **Fix**: `decodeString` must fall back to the raw arg: `if json.Unmarshal(...); err == nil { return s }; return arg`. Lua-callable wasm receives JSON-encoded strings; the sandbox endpoint passes bare strings — the function must handle both.
- **References**: `okf-mcp/concept/lhtranslation-decodeString-sandbox-string-format`, `examples/plugins/wasm/lhtranslation/helpers.go`, `examples/plugins/wasm/atsumaru/text.go`.

---

## Detection cheat-sheet

| Symptom                                  | Check                                                                 |
| ---------------------------------------- | --------------------------------------------------------------------- |
| empty search/detail, no error            | Pattern 1 — regex.find_all row shape                                   |
| `0001-01-01` dates or missing dates      | Pattern 2 — released_at zero-time / epoch conversion                    |
| regex worked in Lua, fails in wasm       | Pattern 3 — RE2 `.` vs Lua `.`, `[^>]` vs `[^"]`                        |
| stale data after rebuild, ~13ms detail   | Pattern 4 — `DELETE FROM plugin_cache WHERE plugin_id='<id>'`           |
| 3.4 MB wasm, OOM                         | Pattern 5 — use `make`, never std-Go wasip1                             |
| empty manga_id from sandbox chapters     | Pattern 6 — `decodeString` falls back to raw arg                       |
