---
type: Concept
title: Plugin detail view on /view/plugins (design)
description: "Design for a richer per-plugin detail section on /view/plugins cards: collapsible Details panel, version field in plugin.json, feature table sourced from plugin.json. DESIGN ONLY, NOT implemented."
tags: [goisekai, plugins, ui, plugin-json, abi, wasm, lua, design]
generated: { by: agent/mcp, at: "2026-10-26T15:54:52Z" }
status: stable
---

# Plugin detail view on /view/plugins (design)

## Context - the built_with fix that surfaced this

/view/plugins cards were missing the build-toolchain badge for three wasm plugins. Root cause was DATA, not rendering: host already had BuiltWith end-to-end (manager_meta.go -> views_plugins.go -> plugin_cards.lua renders the badge), but only 1manga/plugin.json declared built_with. Added "built_with": "tinygo 0.43.0" to the three ported plugin.json files; reload re-reads meta and badge renders for all four. All three build with TinyGo 0.43.0 via Makefile.

## Agreed design (2026-10-26) - DESIGN ONLY, NOT implemented

1. Placement: extend existing /view/plugins cards with a collapsible Details section. No new route.
2. Feature-table source: plugin.json declares a features object. Host renders table: expected yes/no + provided.
3. Version: add version field to plugin.json. Card already renders v{Version} badge but always empty today.

## Host-expected exports (yes/no baseline)

- required (4): Search, GetMangaDetail, GetChapterList, GetPageList
- optional (2): GetGenres, GetEnrichment
- legacy (1): Init - never called.

## Status
Design agreed; NOT implemented. built_with fix shipped; Details section, version field, features table pending.
