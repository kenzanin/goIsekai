# Bugs
* [Cover button on detail page silently triggers GetMangaDetail](cover-button-fetch-detail.md) - Why the cover picker button on the manga detail page re-invokes plugin detail fetch, and the code paths to check when it regresses.
* [Cached cover and chapter images vanish: FIFO prune plus MangaDex edge challenge blocks re-fetch](offline-image-cache-pruned-and-mangadex-challenge.md) - Resolved 2026-11-04 in OpenSpec change `image-fetch-resilience`: library-protected recency-aware prune and the plugin-declared User-Agent ladder.
