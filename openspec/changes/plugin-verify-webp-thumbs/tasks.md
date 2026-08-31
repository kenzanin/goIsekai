# Tasks

## Group 1 — DB + verify storage

- [ ] 1.1 Schema migration: `plugin_verify` table (plugin_id PK, verify_url, cookies TEXT, user_agent TEXT, updated_at) + `plugins.thumb_ratio REAL DEFAULT 0`; mirror both in `cmd/jetgen/main.go`; regen `.gen`
- [ ] 1.2 DB methods: `UpsertPluginVerify` / `GetPluginVerify` in a new `internal/database/verify.go`; `thumb_ratio` read/write in plugin upsert/list paths
- [ ] 1.3 DB tests for verify roundtrip + thumb_ratio default

## Group 2 — ABI + pluginmanager

- [ ] 2.1 `types.PluginMeta`/Init JSON: optional `verify_url`, `needs_human_verify`, `thumb_ratio` fields
- [ ] 2.2 pluginmanager: persist new fields at RegisterPlugin; expose via ListPlugins
- [ ] 2.3 `examples/mangadex-plugin`: emit `thumb_ratio: 0.703` (256/364); rebuild + reinstall wasm

## Group 3 — hostnet verify inject + challenge detect

- [ ] 3.1 `hostnet.SetVerifyCookies(pluginID, cookieHeader, ua)`: parse (tolerant), seed per-plugin client jar + UA override
- [ ] 3.2 Challenge detection: 403/503 + `challenge-platform`/`Just a moment` (8 KB bounded read) → typed `ErrChallenge{VerifyURL}` from SearchManga/GetMangaDetails bridge surface
- [ ] 3.3 hostnet tests: cookie inject persists across requests; challenge error propagates

## Group 4 — WebP cache

- [ ] 4.1 Add `gen2brain/webp` dep; wire encode at disk-cache write in `internal/bridge/image.go` (skip GIF/WebP; fail-open to original bytes on error)
- [ ] 4.2 Serve path: correct Content-Type for `.webp` cache hits
- [ ] 4.3 Test: JPEG bytes in → `.webp` file on disk smaller than input; GIF passthrough untouched

## Group 5 — UI

- [ ] 5.1 plugins.jet: Human Verification panel (Open verification page link, textarea, optional UA field, Save action, verified status) + collapsible "How to copy cookies" steps
- [ ] 5.2 action endpoint `POST /action/save-verify/{pluginID}` (303 back to plugins)
- [ ] 5.3 search.jet + detail.jet: challenge banner when ErrChallenge
- [ ] 5.4 library.jet + search.jet: aspect-ratio styling from plugin thumb_ratio (fallback 2:3); regen tailwind.css if new classes
- [ ] 5.5 views.go: pass thumb_ratio + verify state + challenge error into templates

## Group 6 — gates

- [ ] 6.1 Full: build CGO_ENABLED=0, `go test ./internal/... -count=1`, gofmt, template tests, live curl verify (plugins panel renders, save-verify 303, search banner path, webp content-type, aspect-ratio style present)
