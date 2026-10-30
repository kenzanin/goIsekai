package httpserver

import (
	"errors"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
	"goisekai/pkg/types"
)

// loadMangaForDetail resolves the manga row + chapter list a detail page
// renders, including every fallback that keeps the page alive when the cache
// and the plugin disagree.
//
// ok is false when there is neither data nor a challenge wizard to show; the
// caller answers 502 in that case. challenge and cachedData are handed back
// because they drive the verification wizard and the "served from cache" note.
func (s *Server) loadMangaForDetail(
	pluginID, mangaID string,
	pluginMeta pluginmanager.LoadedPlugin,
	verifyRow database.PluginVerifyRow,
) (manga types.Manga, chapters []types.Chapter, challenge, cachedData, ok bool) {
	var err error
	if pluginMeta.NeedsHumanVerify {
		// Blocked: wizard renders via Challenge; reload after cookies saved
		// re-runs the real fetch.
		challenge = true
		// Use cached data for blocked plugins so the page doesn't blank out.
		// The wizard renders over the cached data so the user can verify.
		cachedData = true
		manga, chapters, err = s.service.CachedMangaAndChapters(pluginID, mangaID)
		if err != nil {
			s.logger.Debug("manga detail: no cached data for blocked plugin", "plugin", pluginID, "manga", mangaID, "error", err)
			cachedData = false
		} else if manga.ID == "" {
			s.logger.Debug("manga detail: empty manga from cache for blocked plugin", "plugin", pluginID, "manga", mangaID)
			cachedData = false
		}
		// If cookies exist but may be stale, try live fetch to verify.
		// If it fails, we keep the cached data.
		if verifyRow.Cookies != "" && manga.ID != "" {
			liveManga, liveChapters, liveErr := s.service.GetMangaDetails(pluginID, mangaID)
			if liveErr == nil && liveManga.ID != "" {
				manga, chapters = liveManga, liveChapters
			}
		}
	} else {
		// Not blocked: serve cached data immediately, no live fetch that
		// would block on the interactive lane. The cache is refreshed on
		// background enqueues (library sync, search, etc.).
		manga, chapters, err = s.service.CachedMangaAndChapters(pluginID, mangaID)
		// Chapters are persisted only for library manga (persistMangaDetails
		// skips them otherwise, and PruneOrphans deletes non-library rows),
		// while the manga row itself IS cached by any open. So the cache can
		// legitimately answer with a row and an empty chapter list — exactly
		// what the add-to-library redirect sees. The contract in
		// persistMangaDetails is that a non-library row's chapters are
		// "re-fetched from the plugin on open", so fetch live in that case
		// instead of rendering "No chapters".
		if err == nil && manga.ID != "" && len(chapters) == 0 {
			s.logger.Debug("manga detail: cached copy has no chapters, fetching live", "plugin", pluginID, "manga", mangaID)
			liveManga, liveChapters, liveErr := s.service.GetMangaDetails(pluginID, mangaID)
			if liveErr == nil && len(liveChapters) > 0 {
				manga, chapters = liveManga, liveChapters
			} else if liveErr != nil {
				// Keep the cached copy; a plugin outage must not blank a
				// page the cache can already render (sans chapters).
				s.logger.Warn("manga detail: live chapter fetch failed, keeping cached copy", "plugin", pluginID, "manga", mangaID, "error", liveErr)
				cachedData = true
				if _, isChallenge := errors.AsType[*hostnet.ChallengeError](liveErr); isChallenge {
					challenge = true
					s.logger.Warn("manga detail blocked by challenge", "plugin", pluginID, "manga", mangaID)
				}
			}
		} else if err != nil {
			// Nothing persisted yet (e.g. a fresh search result the host
			// never saved): fall back to the live fetch instead of blanking
			// the page with a 502.
			s.logger.Debug("manga detail: no cached copy, fetching live", "plugin", pluginID, "manga", mangaID, "error", err)
			manga, chapters, err = s.service.GetMangaDetails(pluginID, mangaID)
			if err != nil {
				if _, isChallenge := errors.AsType[*hostnet.ChallengeError](err); isChallenge {
					challenge = true
					s.logger.Warn("manga detail blocked by challenge", "plugin", pluginID, "manga", mangaID)
				} else {
					s.logger.Error("manga detail", "error", err, "plugin", pluginID, "manga", mangaID)
					return manga, chapters, challenge, cachedData, false
				}
			}
		}
	}
	// If no data was loaded and no challenge wizard is shown, report failure
	// (handler will 502). This covers nonexistent plugin or manga.
	if manga.ID == "" && !challenge {
		return manga, chapters, challenge, cachedData, false
	}
	return manga, chapters, challenge, cachedData, true
}
