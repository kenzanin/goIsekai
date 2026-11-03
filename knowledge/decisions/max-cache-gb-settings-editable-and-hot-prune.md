---
type: Decision
title: "max_cache_gb is editable on the Settings page and a lowered cap prunes without restart"
description: "The image cache cap lives in the main Settings form (maintenance card), is saved through the existing save-settings action with finite >= 0 validation, and the hot-reload callback prunes when the cap is lowered; stored precision is one decimal."
tags: [goisekai, image-cache, settings, config, prune, hot-reload]
generated: { by: agent/cli, at: "2026-11-03T22:40:00+07:00" }
status: generated
---

## The setting

`max_cache_gb` (maintenance section, default 2.0) limits the image cache under
`cache_dir/images`. It is now editable on `/view/settings` in the main form's
maintenance card, beside `update_stale_days`.

- **Stored precision is one decimal**: `config.Save` writes
  `strconv.FormatFloat(v, 'f', 1, 64)`, so a form value like 0.25 is stored
  and read back as 0.3. The field's `step="0.1"` keeps the UI from offering finer values.
- **Validation** (both the save handler and the INI parser): present-key-only;
  accepted only when ParseFloat succeeds and the value is finite and `>= 0`.
  Negative, text, NaN and `inf` leave the stored value unchanged. `0` means
  size-based pruning off (pruneImageCache returns immediately for `<= 0`).

## Lowering the cap takes effect without restart

The safe-subset config watcher (`startConfigWatch`, 5 s poll) reads the
previous cap before storing the new one. When the new cap is above 0 and lower
than the previous one, `pruneImageCache` runs in a goroutine (never on the
save request path) and logs the freed bytes as
`config reload: pruned image cache`. Raising the cap or an unchanged reload
deletes nothing, so frequent config rewrites never walk a large cache directory.

All three prune callers — reload-triggered, startup, and the maintenance
batch — are serialised by the package-level `pruneMu`, so concurrent walks
never race over the same files (tested with `-race`).

## Verification notes

- Save-path tests: `internal/httpserver/actions_test.go`
  (`TestSaveSettingsMaxCacheGB`, uses the test-only `bridge.AppService.SetConfigPath`).
- Prune-on-lower tests: `cmd/goisekai/maintenance_test.go`
  (`TestConfigWatchPruneOnLower`, `TestConcurrentPrunesNoDoubleDelete`).
- Manual check to re-run after deploys: change the limit on the Settings page,
  confirm `goisekai.ini` carries the new `max_cache_gb`, then lower it below
  the current cache size and confirm the prune log line within ~10 s.
