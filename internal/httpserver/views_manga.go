package httpserver

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"goisekai/internal/database"
)

// buildMangaDetailData assembles the data map for a manga detail view.
// Used by both the GET handler and action handlers rendering inline.
func (s *Server) buildMangaDetailData(r *http.Request, pluginID, mangaID string) map[string]any {
	// Opening the detail page clears the library card's [New] badge.
	_ = s.service.ClearMangaNew(pluginID, mangaID)
	// Human-verify gate (mirrors viewSearch): when the plugin needs human
	// verification and no cookies are saved yet, skip the fetch entirely — it
	// would burn the CDP solve timeout on a request that cannot succeed — and
	// let the wizard modal render over the page instead. PluginMeta self-loads
	// the plugin so the flag is visible before any plugin call runs.
	verifyRow, _, _ := s.service.GetPluginVerifyState(pluginID)
	pluginMeta := s.service.PluginMeta(pluginID)
	manga, chapters, challenge, cachedData, ok := s.loadMangaForDetail(pluginID, mangaID, pluginMeta, verifyRow)
	if !ok {
		return nil
	}
	// Source plugins rarely supply an author; fall back to the one captured by
	// an enrichment provider, if any.
	if strings.TrimSpace(manga.Author) == "" {
		if author, ok := s.service.StoredAuthor(pluginID, mangaID); ok {
			manga.Author = author
		}
	}
	progress, err := s.service.GetChapterProgresses(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("chapter progress", "error", err, "manga", mangaID)
		progress = map[string]database.ChapterProgress{}
	}
	continueTo := computeContinue(chapters, progress)
	if lastCont := s.continueFromHistory(pluginID, mangaID, chapters, progress); lastCont != nil {
		continueTo = lastCont
	}
	inLibrary := s.service.IsInLibrary(pluginID, mangaID)
	lastSynced, _ := s.service.LibrarySyncState(pluginID, mangaID, time.Now())

	// Display name + icon come from the same source every other page uses:
	// DB rows as the base (they survive restarts and cover plugins whose VM is
	// still deferred — PluginMetas alone is runtime-only and empty for those,
	// which renders raw IDs and blank icons), runtime metas overlaid when
	// fresher, and wasm plugins falling back to their on-disk logo.png because
	// their plugin.json need not declare one. Resolving here instead of inline
	// is what makes a wasm plugin's logo show on the detail page at all.
	names, icons := s.pluginDisplayMaps()
	pluginName := pluginID
	if n := names[pluginID]; n != "" {
		pluginName = n
	}
	pluginIcon := icons[pluginID]
	altTitles, _ := s.service.ListAltTitles(pluginID, mangaID)
	altSummaries, _ := s.service.ListAltSummaries(pluginID, mangaID)
	cats, _ := s.service.ListCategories(pluginID, mangaID)
	rels, _ := s.service.ListRelated(pluginID, mangaID)
	altCovers, _ := s.service.ListAltCovers(pluginID, mangaID)
	coverVersions := make(map[string]string, len(altCovers)+1)
	for _, c := range altCovers {
		if c.URL != "" {
			if v := s.service.CoverCacheVersion(pluginID, c.URL); v != "" {
				coverVersions[c.URL] = v
			}
		}
	}
	if manga.CoverURL != "" {
		if v := s.service.CoverCacheVersion(pluginID, manga.CoverURL); v != "" {
			coverVersions[manga.CoverURL] = v
		}
	}
	s.logger.Debug("enrichment cache", "plugin", pluginID, "manga", mangaID, "categories", len(cats), "related", len(rels))
	overrideGenres, _, _ := s.service.GetMangaGenres(pluginID, mangaID)
	// Human verification wizard data (verifyRow/pluginMeta hoisted to the top
	// for the needs_human_verify fetch gate above).

	const chapterPageSize = 50
	chPage, _ := strconv.Atoi(r.URL.Query().Get("ChPage"))
	if chPage < 1 {
		chPage = 1
	}
	chTotal := len(chapters)
	chStart := min((chPage-1)*chapterPageSize, chTotal)
	chEnd := min(chStart+chapterPageSize, chTotal)

	return map[string]any{
		"PluginID":         pluginID,
		"PluginName":       pluginName,
		"PluginIcon":       pluginIcon,
		"MangaID":          mangaID,
		"Manga":            manga,
		"AltTitles":        altTitles,
		"AltSummaries":     altSummaries,
		"CurrentTitle":     manga.Title,
		"Chapters":         chapters[chStart:chEnd],
		"Progress":         progress,
		"Continue":         continueTo,
		"InLibrary":        inLibrary,
		"Challenge":        challenge,
		"Verify":           verifyRow.Cookies,
		"VerifyURL":        pluginMeta.VerifyURL,
		"VerifyUserAgent":  verifyRow.UserAgent,
		"NeedsHumanVerify": pluginMeta.NeedsHumanVerify,
		"VerifyCookies":    verifyRow.Cookies,
		"CachedData":       cachedData,
		"ChCurrentPage":    chPage,
		"ChTotalPages":     max((chTotal+chapterPageSize-1)/chapterPageSize, 1),
		"ChHasNext":        chEnd < chTotal,
		"ChHasPrev":        chPage > 1,
		"Categories":       cats,
		"Related":          rels,
		"AltCovers":        altCovers,
		"PluginGenres":     manga.RawGenres,
		"OverrideGenres":   overrideGenres,
		"Genres":           manga.Genres,
		"CoverDim":         manga.CoverDim,
		"CoverVersions":    coverVersions,
		"LastSynced":       lastSynced,
	}
}

// viewMangaDetail renders a manga's info plus its chapter list.
func (s *Server) viewMangaDetail(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	data := s.buildMangaDetailData(r, pluginID, mangaID)
	if data == nil {
		http.Error(w, "failed to load manga details", http.StatusBadGateway)
		return
	}
	// Action handlers (library toggle, genre chips, enrichment) redirect back
	// here, and the browser follows that 303 with X-Partial still set: render
	// the partial so the handler can swap it into #content.
	if r.Header.Get("X-Partial") == "true" {
		s.renderPage(w, r, "views/detail", "", data)
		return
	}
	s.renderPage(w, r, "views/detail", "", data)
}
