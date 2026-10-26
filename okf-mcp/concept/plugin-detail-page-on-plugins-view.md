---
id: plugin-detail-page-on-plugins-view
title: "Plugin detail view on /view/plugins (design)"
description: "Design for a richer per-plugin detail section on the existing /view/plugins cards: extend cards with a collapsible Details panel (no new route), add a version field to plugin.json, and render a feature table (host-expected yes/no + extras) sourced from plugin.json. Includes the built_with badge fix that surfaced the gap."
type: Concept
tags: [goisekai, plugins, ui, plugin-json, abi, wasm, lua, design]
generated:
  at: "2026-10-26T00:00:00Z"
---

# Plugin detail view on /view/plugins (design)

## Context — the built_with fix that surfaced this

The `/view/plugins` cards were missing the build-toolchain badge for three wasm
plugins (atsumaru, lhtranslation, mangabuddy). Root cause was **data, not
rendering**: the host already carried `BuiltWith` end-to-end
(`manager_meta.go` → `views_plugins.go` → `plugin_cards.lua` renders the badge),
but only `1manga/plugin.json` declared the field. Added
`"built_with": "tinygo 0.43.0"` to the three ported plugin.json files (source +
app_data installed copies); reload re-reads the meta and the badge now renders
for all four wasm plugins. All three build with TinyGo 0.43.0 via Makefile.

That gap motivated a richer per-plugin detail view.

## Agreed design (2026-10-26)

Three decisions, chosen by the user:

1. **Placement** — extend the existing `/view/plugins` cards with a collapsible
   "Details" section per card. No new route; everything stays in one place.
2. **Feature-table source** — plugin.json declares a `features` object. The host
   renders a table: host-expected yes/no + extra features the plugin provides.
3. **Version** — add a `version` field to plugin.json. The card already renders a
   `v{Version}` badge (`plugin_cards.lua`) but it is always empty today because no
   plugin.json declares `version`.

## What the Details section shows

- name + logo — already on the card (icon + `meta.Name`).
- version — new `plugin.json` field feeding the existing `v{Version}` badge.
- toolchain/runtime — already present: the `Kind` badge (lua/js/native/yaegi/wasm)
  plus the `built_with` badge.
- **feature table** — rows = features; columns = "expected by host (yes/no)" and
  "provided by plugin".

## Host-expected exports (the yes/no baseline)

These are what the host ABI expects, and the host can detect which a plugin
implements without any plugin declaration:

- required (4): `Search`, `GetMangaDetail`, `GetChapterList`, `GetPageList`
- optional (2): `GetGenres`, `GetEnrichment`
- legacy (1): `Init` — never called by the current host

The extras column carries data-level capabilities the plugin declares (e.g.
released_at, rating, scanlator, pagination, genre filter). **Caveat:** sourcing
extras from plugin.json means each plugin must declare them and they can drift
from the code — the host cannot verify a declared extra is actually emitted.

## Data flow for version + features

`plugin.json` → `manager_meta.go` (parse into the meta struct) →
`views_plugins.go` (`PluginView`) → `plugin_cards.lua` (render). Adding `version`
and `features` follows the same path `built_with` already takes — no new wiring.

## Status

Design agreed; **not yet implemented**. The `built_with` fix is shipped; the
Details section, `version` field and `features` table are pending a go-ahead.
