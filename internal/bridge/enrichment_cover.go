package bridge

import (
	"context"
	"fmt"
)

// FetchCovers fetches alternative cover candidates from enrichment sources
// (mangadex, mangaupdates, ...) and stores them for picking on the detail page.
func (s *AppService) FetchCovers(pluginID, mangaID, title string) error {
	return s.FetchEnrichment(pluginID, mangaID, title, s.EnrichmentSources())
}

// SetCover makes url the manga's cover: evicts existing cache entries for the
// old URL, downloads the new one through the image pipeline, and updates the
// stored cover_url. It rides the fetch lane like every other cover download.
func (s *AppService) SetCover(pluginID, mangaID, url string) error {
	if url == "" {
		return fmt.Errorf("bridge: set cover: empty URL")
	}
	return s.runOnFetch(context.Background(), pluginID, func(context.Context) error {
		return s.setCover(pluginID, mangaID, url)
	})
}

func (s *AppService) setCover(pluginID, mangaID, url string) error {
	cached, err := s.db.GetMangaCached(pluginID, mangaID)
	if err != nil {
		return fmt.Errorf("bridge: set cover: %w", err)
	}
	// Evict caches for the current URL so a later refetch of it isn't stale.
	s.imageMu.Lock()
	delete(s.imageCache, cached.CoverURL)
	s.imageMu.Unlock()
	if _, err := s.GetImage(context.Background(), pluginID, url, nil, "", "", PrioHigh); err != nil {
		return fmt.Errorf("bridge: set cover download: %w", err)
	}
	// Force-write the pick: this is the user's explicit choice, so it must
	// land even if custom_cover is already set (re-picking another cover).
	if err := s.db.SetMangaCover(pluginID, mangaID, url); err != nil {
		return fmt.Errorf("bridge: set cover update url: %w", err)
	}
	// Lock the pick: later UpsertManga calls (detail refresh) must not
	// overwrite the user-chosen cover with the plugin default.
	if _, err := s.db.Exec(`UPDATE mangas SET custom_cover = 1 WHERE plugin_id = ? AND source_manga_id = ?`, pluginID, mangaID); err != nil {
		return fmt.Errorf("bridge: set cover lock: %w", err)
	}
	return nil
}
