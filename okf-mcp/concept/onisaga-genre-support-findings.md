---
type: Concept
title: Onisaga genre support findings
description: Host genre contract, onisaga page structure, and the open slug-vocabulary mapping for search-genre support.
tags:
  - goisekai
  - onisaga
  - genre
  - plugin
  - finding
generated:
  by: process:omp-assistant
  at: "2026-10-11T00:00:00Z"
---

# Onisaga genre support findings

Grounded in the onisaga search-genre investigation (Oct 2026).
Plugin file: `app_data/plugins/onisaga/main.lua`.

## Host contract (settled)

- Host dispatches `GetGenres` and accepts `SearchFilter.Genres`.
- Host passes genre slugs through untouched — work is plugin-side only:
  onisaga must expose a name+slug genre list and consume the filter.
- User-designated reference for a complete example plugin: **mangadex**
  (`untuk contoh plugin yang lengkap coba lihat plugin mangadex bro itu acuan nya`).

## Onisaga current state (gap)

- Onisaga has no `get_genres`.
- Its `search_manga` does not accept a genre filter.
- Drafted plan (from anisascans/1manga/atsumaru + mangadex examples):
  `get_genres` returns a slug+name list; `search_manga` checks
  `args.genres` — if non-empty, GET the genre page (`/genre/<slug>/`)
  directly and parse the same card markup, otherwise run normal search.
- Open at investigation end: exact `get_genres` return shape the host
  expects (mangadex read in flight), and the slug-vocabulary source.

## Page structure (verified)

- Genre pages are **server-rendered**: `GET /genre/action` → 200 with
  manga anchors present. Plugin-side extraction is straightforward
  (no client-side rendering problem like search results had).
- Bare `/genre` (index) → 404. No genre index to enumerate there.
- Genre-page JSON-LD `ItemList` carries only 10 items while the visible
  page has 34 cards — JSON-LD is NOT a usable source for the full list.
- Genre-page cards share the same shape as search cards
  (img + aria-label anchor), spaced wider because of an `nsfwGate`
  block in the middle.
- `/browse` is the genre-filter entry ("Browse Genres" heading links to
  `/browse`); homepage returns 200 but carries no static genre anchors.
- Vocabulary split: detail pages carry `/genre/{slug}` anchors, but the
  browse page only exposes numeric Livewire IDs.

## Vocabulary crux (open)

- Browse-page Livewire checkbox filter decoded: **6719 unique numeric
  IDs**, deduped from duplicated Livewire snapshots, matching the
  homepage stat — confirmed real, not a parse artifact.
- 6719 is suspiciously large for a genre list; the numeric-ID ↔
  `/genre/{slug}` mapping is unresolved.
- Sitemap as vocabulary source is blocked (see Cloudflare concept):
  direct fetch 403 without fresh cookies; stored `plugin_verify`
  cookies are stale.

## Evidence

- `GET /genre/action` → 200, manga anchors present in HTML.
- `GET /genre` → 404.
- Genre page: 34 visible cards vs 10 JSON-LD items.
- Livewire probe: 6719 unique IDs after dedup.
- Zero edits landed: `app_data/plugins/onisaga/main.lua` and search
  templates unchanged through the whole investigation arc.
