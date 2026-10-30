package httpserver

import (
	"context"

	"goisekai/internal/logger"
	"goisekai/internal/workers"
	"goisekai/pkg/types"
)

// migrationCandidate is one possible migration target for the picker's card.
type migrationCandidate struct {
	PluginID      string
	PluginName    string
	SourceMangaID string
	Title         string
	CoverURL      string
	IsExactMatch  bool
	// ChapterCount is -1 when the source could not be asked, so the card can
	// say "unknown" instead of claiming zero chapters.
	ChapterCount int
}

// collectMigrationPage runs the target search for one page of the migration
// picker and returns the candidates already shaped for the template, plus the
// per-plugin failures and the "every source failed" sentinel.
//
// It returns a non-nil error only when the page cannot be rendered at all (the
// fan-out itself failed); a source that merely could not be reached ends up in
// failures instead.
func (s *Server) collectMigrationPage(
	pluginID, mangaID, q string,
	targets []string,
	page, sourceChapterCount int,
) (candidates []any, failures []string, searchErr error, hasNext bool, fatal error) {
	var cands []migrationCandidate

	if q != "" {
		// The fan-out lives in the bridge: it owns the parallel search over the
		// fetch lane and the alt-title match keys, so the picker recognises a
		// source that publishes the series under a different name. An empty query
		// searches nothing rather than hammering every source.
		found, failed, err := s.service.CollectMigrationCandidates(pluginID, mangaID, targets, q)
		if err != nil {
			return nil, nil, nil, false, err
		}
		failures = failed
		for _, c := range found {
			cands = append(cands, migrationCandidate{
				PluginID:      c.PluginID,
				PluginName:    c.PluginName,
				SourceMangaID: c.SourceMangaID,
				Title:         c.Title,
				CoverURL:      c.CoverURL,
				IsExactMatch:  c.IsExactMatch,
				// A search result carries no chapter count; it is asked for per
				// card below, so a source that cannot be reached stays -1.
				ChapterCount: -1,
			})
		}
		if len(cands) == 0 && len(failures) > 0 && len(failures) == len(targets) {
			// Every target failed: distinct from a genuine no-results.
			searchErr = errAllSourcesFailed
		}
	}

	// Host-side pagination, 30 per page (same as search).
	const pageSize = 30
	total := len(cands)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	hasNext = end < total
	cands = cands[start:end]

	// A search result carries no chapter count, so the counts have to be asked
	// for - one call per card - to make "will this migration lose chapters?"
	// answerable before the button is pressed rather than after. Only the
	// visible page is asked about, and the plugin cache holds a chapter list for
	// 168h, so revisiting the picker is cheap. A source that cannot be reached
	// keeps -1 and its card says "unknown" rather than lying about having none.
	futures := make(map[int]*workers.Future)
	for i := range cands {
		fut, err := s.service.EnqueueGetChapterList(context.Background(), cands[i].PluginID, cands[i].SourceMangaID, func(ctx context.Context, chapters []types.Chapter, err error) {
			if err != nil {
				logger.Warn("migrate chapter count failed", "plugin", cands[i].PluginID, "manga", cands[i].SourceMangaID, "error", err)
				return
			}
			cands[i].ChapterCount = len(chapters)
		})
		if err != nil {
			logger.Warn("migrate chapter enqueue failed", "plugin", cands[i].PluginID, "manga", cands[i].SourceMangaID, "error", err)
			continue
		}
		futures[i] = fut
	}
	// Wait for all chapter fetches to complete
	for i, fut := range futures {
		if err := fut.Await(context.Background()); err != nil {
			logger.Warn("migrate chapter future await failed", "plugin", cands[i].PluginID, "manga", cands[i].SourceMangaID, "error", err)
		}
	}

	for _, c := range cands {
		candidates = append(candidates, map[string]any{
			"PluginID": c.PluginID, "PluginName": c.PluginName, "SourceMangaID": c.SourceMangaID, "Title": c.Title, "CoverURL": c.CoverURL, "IsExactMatch": c.IsExactMatch,
			"ChapterCount": c.ChapterCount, "SourceChapterCount": sourceChapterCount,
			// Fewer chapters than the entry being moved is the trap this
			// comparison exists to catch: the migration discards the surplus.
			"Fewer": c.ChapterCount >= 0 && sourceChapterCount > 0 && c.ChapterCount < sourceChapterCount,
		})
	}
	return candidates, failures, searchErr, hasNext, nil
}
