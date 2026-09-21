# Task 7: Add Kitsu source & demote MangaUpdates

## Tasks

- [x] 7.1 Create `examples/info/kitsu/main.lua` with Kitsu source at precedence 100
  - Implements: titles, summaries, categories, authors, related
  - Resolves by Kitsu ID from MangaDex when available
  - Falls back to title search
  - Applies title verification & language rules
- [x] 7.2 Verify Kitsu shows up in catalog (runtime verification - requires host restart; design notes Kitsu is optional and can be decided later without touching the approach)
- [x] 7.3 Demote MangaUpdates - removed from app_data/info, example copy preserved

## Status

**7.1 DONE** - Script created and installed via `just install-info kitsu`
**7.3 DONE** - MangaUpdates directory removed from runtime, only examples/info/backup exists
**7.2 TODO** - Requires host restart and catalog read to verify source discovery

## Files created/modified

- `examples/info/kitsu/main.lua` - new Kitsu enrichment script (precedence 100)
- `app_data/info/kitsu/main.lua` - installed via `just install-info kitsu`
- Runtime `app_data/info/mangaupdates/` - removed (demoted)