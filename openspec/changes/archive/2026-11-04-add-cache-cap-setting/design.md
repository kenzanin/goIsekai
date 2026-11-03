# Design

## Context

Verified while planning:

- `max_cache_gb` is a full config key: default 2.0, parsed in `config_set.go`, written by `Config.Save` with one decimal (`'f', 1`), and covered by the save round-trip test that walks every `Config` field.
- `startConfigWatch` (`cmd/goisekai/maintenance.go`) polls `goisekai.ini` every 5 seconds and stores the new cap atomically (`maxCacheGB`, float64 bits). It does not prune.
- `pruneImageCache` runs at startup and inside the maintenance batch, which exists only when `backup_interval_hours` is above 0. Nothing prunes in response to a lower cap.
- `handleSaveSettings` loads the config, applies only the keys present in the form (`if _, ok := r.Form["key"]`), validates numerics (`port > 0`, `update_stale_days >= 1`) and ignores a bad value, then saves and redirects.
- `viewSettings` already passes `CacheBytes`; `settings.lua` shows it under "Image Cache" next to a separate "Clear all cached images" form. The main settings form holds the maintenance fields such as `update_stale_days`.

## Goals / Non-Goals

**Goals:**
- Set the cap from the Settings page with the same save path, validation style and hot reload as every other setting.
- Make a lowered cap take effect promptly.

**Non-Goals:**
- No new endpoint, no "prune now" button and no per-plugin or per-manga limits.
- No change to which files the prune deletes (`image-fetch-resilience` owns the ordering).
- No unit selector: the field is gigabytes, as the key is.

## Decisions

### 1. Follow the existing form pattern, no new route

The field is named `max_cache_gb` and lives in the main settings form's maintenance card beside `update_stale_days`; the help text shows the current cache size from `CacheBytes`. `handleSaveSettings` gains one branch in the same shape as its siblings: present in the form, parsed as a float, accepted when finite and `>= 0`, otherwise ignored. NaN and infinities are rejected explicitly because `strconv.ParseFloat` accepts them.

The existing "Image Cache" block with the clear button stays as it is; it is a separate form, so the cap field cannot sit inside it.

Alternative rejected: a dedicated `/action/set-cache-cap` endpoint. It adds a CSRF-guarded route and a test surface for one number that the existing action already handles.

### 2. Settings keep one-decimal precision

`Config.Save` writes `max_cache_gb` with one decimal, so 0.25 is stored as 0.2 or 0.3. The input uses `step="0.1"` so the UI offers only values that survive the round trip. Changing the stored precision would alter the file format for every user, which this change does not need.

### 3. Prune on a lowered cap runs from the reload callback, in a goroutine

In `startConfigWatch`, the callback reads the previous cap before storing the new one. When the new cap is greater than 0 and lower than the previous one, it starts `pruneImageCache` in a goroutine and logs the freed bytes, so the 5-second watcher and the saving request are never blocked. An unchanged or raised cap does nothing, which keeps frequent config rewrites from walking a 14,000-file directory.

A package-level mutex serialises this prune with the startup and maintenance-batch prunes, so two walks never race over the same files. Without it a double delete only produces warnings, but it also doubles the disk walk.

Alternative rejected: pruning in `handleSaveSettings`. The handler would block the response on a directory walk and would bypass the watcher path that also covers a hand-edited `goisekai.ini`.

`startConfigWatch` needs the data directory for the fallback cache location; it gains that parameter from `main.go`, where `dataDir` is already known.

## Risks / Trade-offs

- A lowered cap deletes files up to a few seconds after saving, on the watcher's poll interval. The page cannot show the new size immediately; the next load does.
- Lowering the cap is destructive and silent apart from the log line. The field's help text says files are deleted when the limit is lowered.
- `image-fetch-resilience` changes the `pruneImageCache` signature (a library-protection predicate). Whichever change lands second adapts its call site; the call added here is one line.
- The one-decimal storage means a value like 0.25 reads back as 0.3 after a save. The `step` attribute keeps the UI from offering it.
