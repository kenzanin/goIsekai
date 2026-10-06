package httpserver

import (
	"goisekai/internal/database"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// viewLibrary renders the library grid (also the home page).
func (s *Server) viewLibrary(w http.ResponseWriter, r *http.Request) {
	mangas, err := s.service.ListLibrary()
	if err != nil {
		s.logger.Error("library list", "error", err)
	}

	// When a search query is provided, filter the library via FTS.
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var filtered []database.Manga
	if q != "" {
		hits, hitErr := s.service.SearchLibrary(q)
		if hitErr != nil {
			s.logger.Warn("library search", "error", hitErr)
		} else if len(hits) > 0 {
			// Build an index for fast lookup: (pluginID:sourceMangaID) -> manga
			libIdx := make(map[string]database.Manga, len(mangas))
			for _, m := range mangas {
				libIdx[m.PluginID+":"+m.SourceMangaID] = m
			}
			for _, h := range hits {
				if m, ok := libIdx[h.PluginID+":"+h.SourceMangaID]; ok {
					filtered = append(filtered, m)
				}
			}
		}
		mangas = filtered
	}
	// Optional per-plugin filter (?pluginID=). Purely in-memory over the
	// ListLibrary() result — combines with q and pagination below.
	pluginID := strings.TrimSpace(r.URL.Query().Get("pluginID"))
	if pluginID != "" {
		kept := make([]database.Manga, 0, len(mangas))
		for _, m := range mangas {
			if m.PluginID == pluginID {
				kept = append(kept, m)
			}
		}
		mangas = kept
	}
	// Sorting and the status/tag filters run over the whole library before it is
	// sliced, otherwise they would only describe the 24 cards on this page.
	// Progress stats and categories therefore have to be loaded here rather than
	// next to the grid rendering.
	libStats, err := s.service.ListLibraryWithProgress()
	if err != nil {
		s.logger.Warn("library stats", "error", err)
	}
	statsForSort := statsByManga(mangas, libStats)
	categories, catErr := s.service.ListLibraryCategories()
	if catErr != nil {
		s.logger.Warn("library categories", "error", catErr)
	}

	sortKey := strings.TrimSpace(r.URL.Query().Get("sort"))
	if !slices.Contains(librarySorts, sortKey) {
		sortKey = "updated"
	}
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	if !slices.Contains(libraryStatuses, statusFilter) {
		statusFilter = "all"
	}
	tagFilter := strings.TrimSpace(r.URL.Query().Get("tag"))

	mangas = filterLibrary(mangas, statsForSort, categories, statusFilter, tagFilter)
	sortLibrary(mangas, statsForSort, sortKey)

	// Host-side pagination: slice the sorted/filtered library so the grid
	// renders one page at a time.
	const pageSize = 24
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	total := len(mangas)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	ratios := make(map[string]float64)
	metas := s.service.PluginMetas()
	for id, m := range metas {
		ratios[id] = m.ThumbRatio
	}
	mangaPluginMap := make(map[string]string) // mangaID -> pluginID (from DB)
	// Display names + icons: DB rows as base, runtime metas overlay; wasm
	// plugins fall back to their on-disk logo.png (see pluginDisplayMaps).
	pluginNameMap, pluginIconMap := s.pluginDisplayMaps()
	rows, err := s.service.QueryMangaPluginIDs()
	if err == nil {
		for _, r := range rows {
			mangaPluginMap[r.MangaID] = r.PluginID
		}
	}
	statsMap := make(map[string]map[string]any) // mangaID -> {TotalChapters, ReadChapters, PluginName, HasNew}
	for _, st := range libStats {
		pluginID := mangaPluginMap[st.MangaID]
		pluginName := pluginNameMap[pluginID]
		if pluginName == "" {
			pluginName = pluginID
		}
		pluginIcon := ""
		if icon, ok := pluginIconMap[pluginID]; ok {
			pluginIcon = icon
		}
		statsMap[st.MangaID] = map[string]any{
			"TotalChapters": st.TotalChapters,
			"ReadChapters":  st.ReadChapters,
			"PluginName":    pluginName,
			"PluginIcon":    pluginIcon,
			"HasNew":        st.HasNew,
		}
	}
	overview, err := s.service.LibraryOverview()
	if err != nil {
		s.logger.Warn("library overview", "error", err)
	}
	statusLine, readLine, readingTime, mostLine, fewestLine := buildOverviewStrings(overview)
	// Duplicate detection is skipped on search (keeps it cheap on filter);
	// keys are still passed (empty) so the template always has them.
	var (
		duplicateCount  int
		duplicateGroups []database.DuplicateGroup
	)
	if q == "" {
		groups, dupErr := s.service.FindPotentialDuplicates()
		if dupErr != nil {
			s.logger.Warn("library duplicates", "error", dupErr)
		}
		duplicateGroups = groups
		seen := make(map[string]struct{}, len(groups)*2)
		for _, g := range groups {
			for _, m := range g.Members {
				seen[strconv.FormatInt(m.ID, 10)] = struct{}{}
			}
		}
		duplicateCount = len(seen)
	}
	// Per-plugin title counts for the sidebar card.
	var pluginCounts []map[string]any
	if q == "" {
		if pc, pcErr := s.service.CountLibraryByPlugin(); pcErr != nil {
			s.logger.Warn("library plugin counts", "error", pcErr)
		} else {
			pluginCounts = make([]map[string]any, 0, len(pc))
			for _, p := range pc {
				pluginCounts = append(pluginCounts, map[string]any{
					"PluginID": p.PluginID,
					"Name":     pluginNameMap[p.PluginID],
					"Icon":     pluginIconMap[p.PluginID],
					"Count":    p.Count,
				})
			}
		}
	}
	// Display name for the pluginID filter chip (same fallback as statsMap).
	pluginFilterName := pluginNameMap[pluginID]
	if pluginFilterName == "" {
		pluginFilterName = pluginID
	}
	s.renderPage(w, r, "views/library", "library", map[string]any{
		"Mangas":          mangas[start:end],
		"Q":               q,
		"PluginID":        pluginID,
		"PluginName":      pluginFilterName,
		"ResultCount":     total,
		"Ratios":          ratios,
		"LibraryStats":    statsMap,
		"Page":            page,
		"TotalPages":      max((total+pageSize-1)/pageSize, 1),
		"HasNext":         end < total,
		"HasPrev":         page > 1,
		"Sort":            sortKey,
		"Status":          statusFilter,
		"Tag":             tagFilter,
		"CategoryCounts":  s.service.LibraryCategoryCountsOrEmpty(),
		"DuplicateCount":  duplicateCount,
		"DuplicateGroups": duplicateGroups,
		"PluginCounts":    pluginCounts,
		"Stats": map[string]any{
			"TotalTitles": overview.TotalTitles,
			"StatusLine":  statusLine,
			"ReadLine":    readLine,
			"HasUpdates":  overview.HasUpdates,
			"ReadingTime": readingTime,
			"MostLine":    mostLine,
			"FewestLine":  fewestLine,
			"HasFewest":   overview.TotalTitles > 1 && overview.FewestCount > 0,
		},
	})
}
