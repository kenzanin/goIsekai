# Tasks

## 1. Settings form and save handler

- [x] 1.1 Add a `max_cache_gb` number input (`min="0"`, `step="0.1"`) to the maintenance card of the main form in `internal/templates/views/settings.lua`, prefilled from the config, with help text showing the current cache size and stating that 0 turns size-based pruning off and that lowering the limit deletes files
- [x] 1.2 In `handleSaveSettings`, apply `max_cache_gb` only when the key is present, accepting a finite value `>= 0` and ignoring anything else (negative, text, NaN, infinity)
- [x] 1.3 Tests in `internal/httpserver/actions_test.go`: a valid value is saved, 0 is saved, a negative value and a non-number leave the stored value unchanged, NaN and `inf` are ignored, a form without the key leaves it unchanged
- [x] 1.4 Test that the rendered Settings page shows the stored limit in the field, and that the Settings golden render still passes without a rebaseline

## 2. Apply a lowered cap without a restart

- [x] 2.1 Pass `dataDir` into `startConfigWatch` from `main.go`
- [x] 2.2 In the reload callback, read the previous cap before storing the new one; when the new cap is above 0 and lower than the previous, run `pruneImageCache` in a goroutine and log the freed bytes
- [x] 2.3 Add a package-level mutex so the reload prune, the startup prune and the maintenance-batch prune never walk the cache at the same time
- [x] 2.4 Tests in `cmd/goisekai`: lowering the cap below the cache size prunes it, raising it deletes nothing, an unchanged reload deletes nothing, a cap of 0 deletes nothing, two concurrent prunes do not double-delete or race (run with `-race`)

## 3. Verification

- [x] 3.1 `go test ./internal/httpserver/ ./internal/config/ ./cmd/goisekai/ -count=1`, `go test ./cmd/goisekai/ -race -count=1`, `just check` with 0 issues, `just build`
- [x] 3.2 Restart the host, change the limit on the Settings page and confirm `goisekai.ini` carries the new `max_cache_gb`, then lower it below the current cache size and confirm the log shows the prune within about 10 seconds
- [x] 3.3 Record the setting in `knowledge/` next to the cache-loss note, including that the stored precision is one decimal
