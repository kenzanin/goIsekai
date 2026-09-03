# Tasks: API-First

## 1. Envelope helpers + API key plumbing
- [x] 1.1 Add `writeJSON(w, status, v)` and `writeErr(w, status, msg)` helpers in `internal/httpserver/api.go`
- [x] 1.2 Add `-apiKey` CLI flag in `cmd/goisekai` and `api_key` ini key; thread value into Server struct
- [x] 1.3 Implement chi middleware `requireAPIKey` (constant-time compare; pass-through when empty) and mount on `/api` subgroup in `routes.go` (`GET /image` stays outside the group)
- [x] 1.4 Startup WARN log when binding a non-loopback address with empty `apiKey`

## 2. JSON read endpoints
- [x] 2.1 `GET /api/library` — same service calls as `viewLibrary`, bare JSON array (title, plugin_id, source_manga_id, cover_url, stats, new_since)
- [x] 2.2 `GET /api/search` — pluginID/q/page params via `param()`-style parsing; slice + `has_next` identical to view pagination; 400 on missing params
- [x] 2.3 `GET /api/manga/{pluginID}/{mangaID}` — detail payload: metadata, status, description, chapters (newest-first), per-chapter progress, continue point; 404 when not resolvable
- [x] 2.4 `GET /api/history`
- [x] 2.5 `GET /api/image/{pluginID}/{mangaID}/{chapterID}?url=` — raw image bytes payload (WebP conversion + two-level validation with single refetch; 502 envelope on final failure) — history entries JSON (mirror `viewHistory` query incl. sort order)

## 3. JSON action endpoints
- [x] 3.1 `POST /api/library/toggle/{pluginID}/{mangaID}` → `{"in_library": bool}` (no 303)
- [x] 3.2 `POST /api/chapters/read/{pluginID}/{mangaID}/{chapterID}` (optional body `{"read": bool}`) → `{"is_read": bool}`
- [x] 3.3 `POST /api/progress/{pluginID}/{mangaID}/{chapterID}` body `{"page": N}` → `{"page": N, "total_pages": N}`

## 4. Verification
- [ ] 4.1 Handler tests in `internal/httpserver`: envelope shape (200 bare payload / error `{"error":...}` with 400/401/404/502), key middleware accept/reject/pass-through, image exemption
- [ ] 4.2 Parity check: same service state reflected by view + API (toggle via API → visible in `/view/library` HTML)
- [ ] 4.3 `go build ./... && go vet ./... && go test ./internal/httpserver/ ./internal/bridge/ ./internal/database/` green
- [ ] 4.4 Live smoke via curl: search, library, detail, toggle, reader-data with and without `X-API-Key` configured
