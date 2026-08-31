# Design

## Context

Post-migration goIsekai: chi HTTP server + Jet templates + HTMX; plugins run in wazero and call `host_http_request`; all plugin HTTP goes through `internal/hostnet` tls-client with a per-plugin cookie jar; images cached two-tier (memory + `app_data/cache/images/...`); DB is SQLite via jet with migrations in `internal/database/schema.go` mirrored into `cmd/jetgen/main.go`.

## Goals / Non-Goals

Goals: one-time manual verification whose cookies keep the host client working; smaller cache; uncropped thumbnails.
Non-Goals: fully automatic challenge solving (v2 reverse-proxy idea parked); migrating existing cache entries; per-request proxy support.

## Decisions

### D1: Verification state in its own `plugin_verify` table (not columns on `plugins`)
Cookies/UA are credentials with a lifecycle (paste, overwrite, expire) separate from plugin metadata. PK = plugin id; upsert on save. Keeps `plugins` table stable for jetgen.

### D2: Cookie injection at the hostnet client, not per-request headers
`hostnet` already keys clients per plugin. Add a `SetVerifyCookies(pluginID, cookies, ua)` that (a) seeds the client's cookie jar for the site domain and (b) stores the UA override applied on subsequent requests. Per-request header injection would miss redirect/retries and fights the "headers fully replace defaults" tls-client gotcha (#1974, #1978).

### D3: Tolerant cookie parser
Accept: full `Cookie:` header (`a=1; b=2`), single `name=value`, bare `cf_clearance` value (host names it `cf_clearance`). Parse to pairs; store serialized; seed jar at client creation AND on save (existing client instance gets updated too).

### D4: Challenge detection in hostnet response path
A response is a challenge when status 403/503 AND body contains `challenge-platform` / `Just a moment` (bounded read of first 8 KB). Surface as a typed error `ErrChallenge{VerifyURL}` from SearchManga/GetMangaDetails bridge calls; views render a banner when it appears. No body buffering of large images — detection only on plugin JSON API calls (search/detail/pages), which are small.

### D5: WebP via gen2brain/webp, write-path only
`gen2brain/webp` is pure Go (libwebp transpiled via wasm2go; tries shared lib via purego first, falls back — CGO-free either way). Convert at cache-write: decode (jpeg/png) → `webp.Encode(w, img, Options{Quality: 85})` → write `.webp`. Skip GIF (animation) and existing WebP. Cache key path unchanged (extension swap only). Serve sniffs bytes via `http.DetectContentType` — already does; verify it returns image/webp (Go's sniffer does since 1.18).

### D6: thumb_ratio from plugin Init, 0 = default 2:3
Optional JSON field; `pluginmanager` stores it in `plugins.thumb_ratio` at RegisterPlugin (default 0 → template uses 2/3 fallback). Template: `style="aspect-ratio: {{ratio}}"` on the cover slot + `object-cover`, drop fixed `h-64`.

### D7: ABI stays contract_version 1
All new plugin→host fields are optional JSON; old host ignores unknown fields, old plugins omit them. No version bump.

## Risks / Trade-offs

- purego dynamic libwebp attempt on exotic hosts: harmless fallback, but build with `-tags nodynamic`? No — default behavior is fine cross-platform; if a musl host lacks the .so it just falls back to transpiled Go.
- WebP encode of huge pages costs CPU (~10-30 ms/page) — acceptable at read speed; skip conversion on encode error (store original bytes; ponytail: fail-open).
- cf_clearance expiry: banner re-appears; user re-pastes. Accepted UX.

## Migration Plan

Schema: `CREATE TABLE IF NOT EXISTS plugin_verify (...)` + `ALTER TABLE plugins ADD COLUMN thumb_ratio REAL DEFAULT 0`. Both mirrored in jetgen. Old binaries + new DB: additive, safe. Rollback: drop table/column.

## Open Questions

None blocking. (v2 in-page verification reverse-proxy remains parked.)
