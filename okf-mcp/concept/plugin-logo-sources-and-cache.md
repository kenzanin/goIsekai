---
type: Concept
title: Plugin logo sources and cache
description: The three agreed logo sources (file, URL, inline svg/base64), where the cached logo lives (<data_dir>/cache/images/plugins/<pluginID>/logo.*), and that the plugin-page Refresh resets it.
tags:
  - goisekai
  - plugin
  - logo
  - cache
  - decision
generated:
  by: process:reasonix
  at: "2026-10-16T09:50:00Z"
---

# Plugin logo sources and cache

Agreed in-session (2026-10-16), grounded in `internal/httpserver/views_plugins.go`
(`resolveLogoURL`, `pluginDisplayMaps`), `internal/bridge/service_stats.go`
(`PluginDir`, `SyncPluginMeta`), and `internal/pluginmanager/wasm_abi.go`
(`discoverWasm` reads `plugin.json`).

## Three logo sources

A plugin's `Logo` field (`pkg/types/types.go`, `PluginMeta.Logo`) may be:

1. **File** — a bare filename (`logo.png`) beside `main.wasm` / the plugin
   entry file. Served by `/plugin-static/{id}/{file}`
   (`internal/httpserver/plugin_static.go` → `PluginDir`).
2. **URL** — `http(s)://...`. Currently passed through to the browser as-is,
   so it needs a live network fetch on every page and nothing is retained.
3. **Inline** — a `data:` URI (svg text or base64 image). Currently passed
   through to the browser as-is.

## Cache location (user decision, do not create a new folder)

Cache the resolved logo bytes under the **existing image cache tree**:

```
<data_dir>/cache/images/plugins/<pluginID>/logo.png|.svg|.jpg
```

- `<data_dir>/cache` is `CacheDir` (`internal/config/config_defaults.go:52`),
  the same root the image cache uses (`<cache_dir>/images/`).
- `servePluginStatic` (route `/plugin-static/{id}/{file}`) must check this
  cache dir **first** so a URL/inline logo renders instantly at next startup
  without waiting for a live fetch or a loaded plugin VM.
- The cache file is keyed per plugin, so `/plugin-static/{id}/logo.png`
  stays the single stable URL the views emit for every source type.

## Reset

The **Refresh** button on the plugins page (`POST /action/refresh-plugins`,
`handleRefreshPlugins` → `RefreshPlugins`) clears the per-plugin logo cache
so the next view re-resolves the logo from its declared source.

## Wasm bug this fixes

Wasm plugins (`komix_to`, `mangak`, ...) have no logo file — their
`plugin.json` only carries `site_url`, so `meta.Logo` is empty.
`pluginDisplayMaps` falls back to `resolveLogoURL("logo.png", id)` →
`/plugin-static/{id}/logo.png`, which 404s because no file exists and
nothing is cached. The cache dir + refresh reset makes URL/inline logos
persist and render at startup.
