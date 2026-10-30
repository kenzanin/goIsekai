package httpserver

import (
	"goisekai/internal/database"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// dupPalette holds the left-band colours for duplicate groups, assigned by
// group position and cycling once exhausted. Every entry must be bright enough
// to read against the neutral-900 card on a dark page — that rules out blacks,
// greys, and the indigo accent the theme already uses for selection (a group
// banded in the accent colour would look like a highlighted card).
var dupPalette = []string{
	"#ef4444", "#f97316", "#f59e0b", "#eab308",
	"#84cc16", "#22c55e", "#10b981", "#14b8a6",
	"#06b6d4", "#0ea5e9", "#3b82f6", "#a855f7",
	"#d946ef", "#ec4899", "#f43f5e", "#a3e635",
}

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
	// sliced, otherwise they would only describe the 25 cards on this page.
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
	// ?view=duplicate filters the grid to only manga that belong to a
	// duplicate group; the sidebar duplicate card renders just the count
	// when this filter is active (thumbnails are redundant here).
	viewFilter := strings.TrimSpace(r.URL.Query().Get("view"))
	if viewFilter != "duplicate" {
		viewFilter = ""
	}

	// Duplicate detection is always computed so the grid can be filtered to
	// it, regardless of search — but it is skipped when q is set AND the
	// view filter is not "duplicate" (keeps search cheap).
	var (
		duplicateCount  int
		duplicateGroups []database.DuplicateGroup
	)
	if q == "" || viewFilter == "duplicate" {
		groups, dupErr := s.service.FindPotentialDuplicates()
		if dupErr != nil {
			s.logger.Warn("library duplicates", "error", dupErr)
		}
		duplicateGroups = groups
	}

	// Apply the status/tag filter over the whole library, then optionally
	// narrow to duplicate-group members when ?view=duplicate is set.
	mangas = filterLibrary(mangas, statsForSort, categories, statusFilter, tagFilter)

	// dupColors maps a manga row-ID to the colour of its duplicate group. Every
	// member of a group shares one colour, so the same story is recognisable
	// across the grid at a glance. dupRank gives the group's position, which the
	// "duplicates" sort uses to keep a group's members adjacent.
	//
	// The colour is indexed by group position, so the group order has to be
	// deterministic (FindPotentialDuplicates breaks ties on Key) or the bands
	// would change colour between page loads.
	dupColors, dupRank, dupCount := libraryDupMaps(duplicateGroups)
	duplicateCount = dupCount

	if viewFilter == "duplicate" {
		kept := make([]database.Manga, 0, len(mangas))
		for _, m := range mangas {
			if _, ok := dupColors[strconv.FormatInt(m.ID, 10)]; ok {
				kept = append(kept, m)
			}
		}
		mangas = kept
	}
	sortLibrary(mangas, statsForSort, sortKey, dupRank)

	// Host-side pagination: slice the sorted/filtered library so the grid
	// renders one page at a time.
	const pageSize = 25
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
	pluginNameMap, pluginIconMap := s.pluginDisplayMaps()
	statsMap, _ := s.libraryStatsMap(libStats, pluginNameMap, pluginIconMap)
	overview, err := s.service.LibraryOverview()
	if err != nil {
		s.logger.Warn("library overview", "error", err)
	}
	statusLine, readLine, readingTime, mostLine, fewestLine := buildOverviewStrings(overview)
	// Per-plugin title counts for the sidebar card.
	pluginCounts := s.libraryPluginCounts(q, pluginNameMap, pluginIconMap)
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
		"View":            viewFilter,
		"DupColors":       dupColors,
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
