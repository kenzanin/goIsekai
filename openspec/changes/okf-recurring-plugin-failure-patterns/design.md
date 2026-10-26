# Design — okf-recurring-plugin-failure-patterns

## Goal

A single OKF concept that lets a reader match a *symptom* (empty search, zero pages,
zero-time released_at, blank badge) to its *known root cause* and *fix*, without
re-deriving it from code.

## Structure

One file, `okf-mcp/concept/plugin-failure-patterns.md`, one section per pattern.
Each section is uniform so it is skimmable:

- **Symptom** — what the user/UI sees.
- **Silent-failure signal** — why it produces no error (the key diagnostic).
- **Root cause** — the specific mechanism.
- **Fix** — the concrete correction.
- **References** — pointers to the deeper OKF concept / source file.

## The six patterns

1. **regex.find_all shape** — host returns `[][]string` rows of capture groups;
   a 1-capture row is rendered as a bare string by `wasmRegexRow`
   (`internal/pluginmanager/wasm_host_helpers.go`), so a 1-group pattern decodes
   to nothing with no error. Fix: parse `[]json.RawMessage` → `[]string` per row
   (see lhtranslation `firstCapAll`). Deeper: `three-lua-plugins-converted-to-tinygo-wasm`,
   `odin-wasm-plugin-port-candidates`.
2. **released_at zero-time** — `Chapter.ReleasedAt time.Time` has no `omitempty`,
   so an unset field emits `0001-01-01T00:00:00Z`; also epoch-millis fields must
   be converted (wasm32 `int` overflows 13-digit millis → `strconv.FormatInt`).
   Fix: read the API field and pass through `text.date_to_iso`; leave `released_at`
   out when unparseable. Deeper: `atsumaru-released-at-epoch-millis`,
   `released-at-backfill-gap`.
3. **Lua regex not portable to RE2** — RE2 `.` does not match newline; Lua's does.
   Fix: use `(?s)` for multi-line matches; `[^"]*` crosses newlines, `[^>]*` does
   not. Deeper: `log.md` (~line 488).
4. **stale plugin_cache** — GetMangaDetail 24h / GetChapterList 168h TTL masks a
   rebuild (symptom: old payload, or a ~13ms detail = cache hit). Fix: purge
   `plugin_cache` after any plugin edit + reload. Deeper: `docs/HANDOVER.md`.
5. **wasm toolchain drift** — TinyGo pinned to Go 1.26.9; a silent std-Go wasip1
   fallback yields a ~3.4MB main.wasm that OOMs the runtime. Fix: build via the
   per-plugin Makefile (TinyGo), never the fallback. Deeper:
   `wasm-backend-sizes-and-tinygo-port`, `wasm-plugin-build-scripts`.
6. **sandbox decodeString** — the sandbox endpoints pass a bare string, not a
   JSON-encoded one; `decodeString` must fall back to the raw arg or `manga_id`
   comes back empty. Deeper: `lhtranslation-decodeString-sandbox-string-format`.

## Cross-cutting note

Nearly every one **fails silently** — unmarshal of the wrong shape, a regex that
matches nothing, an unread field all return empty without raising an error, so the
failure surfaces only as an empty/zero UI. This is the reason the same bugs recur:
they are cheap to introduce and invisible until a human looks at the output.

## Index + qmd

Add the concept to `okf-mcp/index.md`, then re-run `qmd update -c okf-mcp` so BM25
finds it. No code change; `skip_specs: true`.

## Non-goals

- Not fixing any of the six in this change (the regex-shape fix is a separate
  change, `fix-host-regex-find-all-shapes`).
- Not auto-enforcing the patterns (e.g. a lint) — out of scope; could be future work.
