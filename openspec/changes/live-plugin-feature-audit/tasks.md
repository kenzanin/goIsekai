# Tasks

## 1. Playground instance (prerequisite safe auditing)

- [x] 1.0 Add a `-dataDir` CLI flag (and `GOISEKAI_DATA` env fallback) to `cmd/goisekai/main.go` that overrides `cfg.DataDir` before `setupDirs` runs. Verify `./goisekai -dataDir app_data_playground -port 8081` starts with its own empty DB and PID file while production keeps running.

## 2. Audit run

- [x] 2.1 Write `tools/audit-live-plugins.sh`: takes a base URL (default `http://127.0.0.1:8081`), probes every active plugin's search/detail/chapters/pages via sandbox read-only endpoints with ≤30s timeouts, and writes `live-plugin-audit.json` + `live-plugin-audit.md` (per-plugin rows, per-feature WORKING/MISSING/BROKEN).
- [x] 2.2 Run the audit against the sandbox instance and record the baseline (47 plugins, valid JSON). Findings land in the OKF concept `live-plugin-audit-and-sandbox-instance-pattern`.

## 3. Remediation

- [x] 3.1 Fix broken/missing plugin sources in `examples/plugins/`, sync to the sandbox plugin dir. Completed: mangahome genre parse (`%s` escaping + changed chip markup). Remaining plugins (fenyk, mangade, mangataro, zeroscann, onisaga, komix_to, lhtranslation, reimanga_net) deferred by user decision 2026-11-04 — recorded in OKF concept `live-plugin-feature-audit-remaining-follow-ups`.
- [x] 3.2 Re-probe after the fix and record before/after in the audit output (mangahome detail MISSING → WORKING with genres ["Action","Mature","Shounen"]; no stale cache rows needed clearing).

## 4. Verification and recording

- [x] 4.1 Re-run the audit script against the sandbox and confirm fixed plugins classify WORKING.
- [x] 4.2 `just check` (0 issues) and `just build` (exit 0) pass.
- [x] 4.3 Record the sandbox-instance audit pattern, remediation patterns, and the deferred plugin list as OKF concepts linked to the mangzio-genres and host-date concepts.
