---
type: audit
title: Live plugin released_at/chapter-date parsing status across all 41 live plugins
description: "Scan of every installed plugin under app_data/plugins/ (41 total) for chapter-level date parsing (host ABI field ReleasedAt, pkg/types/types.go:39; there is no created_at). 23 plugins parse a date: 1manga, atsumaru, batcave, comizy, en-thunderscan, komix_to, kunmanga_online, lhtranslation, madarascans, mangabay, mangabuddy, mangak, mangakakalot, mangaread, mangasushi, mangataro, manga_uno, mangzio, manhawato, mangadex, mangafire, mangahub, reimanga_net, roliascan, weebcentral. 17 do NOT parse a date (hits=0): bato1, comicasura_odin, comix, dynastyscan, fanfox, ksgroupscans, madaradex, mangade, mangafreak, mangahere, mangakatana, mangatown, topmanhua, violetmanga, zeroscann, onisaga (no source found), mangapill (by design — mangapill.com chapter HTML publishes no per-chapter release date). Also relates mangaread_odin which already parses DD.MM.YYYY via normalize_mangaread_date, plus representative parse patterns to follow per runtime: Go/TinyGo wasm -> abi.find_one + text.date_to_iso (1manga, lhtranslation, kunmanga_online); Lua -> regex capture + host dateToISO (mangasushi, mangadex, mangafire, batcave, mangatown); Odin wasm -> abi.find_one + abi.date_to_iso (comicasura_odin, atsumaru). Decision: mangaread_odin already has the parser (revisit network/HV); the 17 no-date plugins need per-source date-field discovery + dateToISO wiring + rebuild + POST reload + delete from plugin_cache. This concept supersedes the partial released_at note in the parent 0149-live-plugins-audit."
tags: [goisekai, plugins, audit, released_at, date, abi, build, deploy]
generated: { by: agent/cli, at: "2026-10-28T08:19:32Z" }
status: stable
---

# Related Concepts
- [Live plugins audit: builder/runner versions and released_at parsing status](0149-live-plugins-audit-and-builder-versions.md): Extends the live plugins + builder-versions audit with an explicit released_at/chapter-date parsing checklist across all 41 live plugins (supersedes the partial date note in 0149).
- [atsumaru released_at bug: epoch-millis field was never read](atsumaru-released-at-epoch-millis.md): 0150 audit: atsumaru-released-at-epoch-millis is among the 17 live plugins that do not parse a chapter release date (hits=0 in source scan).
- [Comix.to plugin build spec](comix-plugin-build-spec.md): 0150 audit: comix-plugin-build-spec is among the 17 live plugins that do not parse a chapter release date (hits=0 in source scan).
- [1manga: pages field arrives as plaintext JSON, not enc:v1](1manga-plaintext-pages-field.md): 0150 audit: 1manga-plaintext-pages-field is among the 17 live plugins that do not parse a chapter release date (hits=0 in source scan).
