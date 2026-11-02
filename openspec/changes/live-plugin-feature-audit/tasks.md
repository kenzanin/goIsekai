# Tasks

## 1. Playground instance (prerequisite for safe auditing)

- [ ] 1.0 Add a `-dataDir` CLI flag (and `GOISEKAI_DATA` env fallback) to `cmd/goisekai/main.go` that overrides `cfg.DataDir` before `setupDirs` runs — it already fans DB, plugins, info, cache and PID out of that one path. Verify with `./goisekai -dataDir app_data_playground -port 8081` starting on 8081 with its own empty DB and PID file while production keeps running.

## 2. Audit run

- [ ] 2.1 Write `tools/audit-live-plugins.sh`: takes a base URL (default `http://127.0.0.1:8081`), for every active plugin in `GET /api/plugins`, sequentially probes search/detail/chapters/pages via existing read-only endpoints (including `/sandbox/plugins/*` raw probes) with ≤15s curl timeouts, and writes `live-plugin-audit.json` + `live-plugin-audit.md` (per-plugin rows, per-feature WORKING/MISSING/BROKEN, tag 403-challenge plugins separately).
- [ ] 2.2 Run the audit against the sandbox instance (`-dataDir app_data_playground`) and record the resulting table (this is the baseline; findings land in the OKF note for this change).

## 3. Remediation

- [ ] 3.1 For each plugin classified BROKEN or MISSING, apply the minimal source fix following the established patterns (balanced-object parse for chunk payloads, date_to_iso for dates, optional getGenres from catalog markup, per-page headers for gated CDNs), syncing `examples/plugins/` → the sandbox plugin dir; promote to production `app_data/plugins/` only after the sandbox re-probe passes.
- [ ] 3.2 After each fix, clear caches that recorded absence (e.g. `plugins.genres = NULL` in the SANDBOX db when adding getGenres) and re-probe live in the sandbox, recording before/after in the audit table.
- [ ] 3.3 For plugin fixes that need a binary rebuild (wasm/native), rebuild via the runtime's Makefile target before re-probe; otherwise edits to lua/js are live after a sandbox reload.

## 4. Verification and gate

- [ ] 4.1 Re-run the audit script against the sandbox and confirm previously broken plugins now classify as expected (WORKING where fixed, BROKEN-403-only where a source genuinely gates).
- [ ] 4.2 Run `just check` (0 issues) and `just build` (exit 0); restart the server and confirm it serves root 200 (production path re-verified after fixes are promoted).
- [ ] 4.3 Record the audit findings, the per-plugin fixes, the sandbox-instance pattern (`-dataDir` usage), and the remediation patterns as an OKF concept under `knowledge/` (okf_create), linked to the mangzio-genres and host-date concepts.
