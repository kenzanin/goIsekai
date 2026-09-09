package httpserver

import (
	"fmt"
	"goisekai/internal/database"
	"net/http"
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
	// Host-side pagination: slice the full library (newest-updated first)
	// so the grid renders one page at a time.
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
	// Enriched per-manga stats: read/total chapters + plugin name + hasNew
	libStats, err := s.service.ListLibraryWithProgress()
	if err != nil {
		s.logger.Warn("library stats", "error", err)
	}
	mangaPluginMap := make(map[string]string) // mangaID -> pluginID (from DB)
	// Resolve display names + icons from the DB-persisted plugin rows
	// (populated by SyncPluginMeta after first load), overlaying runtime
	// metas when they are fresher. PluginMetas alone is runtime-only and
	// returns zero values for deferred plugins after a restart, which
	// renders raw IDs in pills and cards.
	dbPlugins, _ := s.service.ListPlugins()
	pluginNameMap := make(map[string]string, len(dbPlugins)) // pluginID -> display name
	pluginIconMap := make(map[string]string, len(dbPlugins)) // pluginID -> icon URL
	for _, p := range dbPlugins {
		pluginNameMap[p.ID] = p.Name
		pluginIconMap[p.ID] = p.IconURL
	}
	for pid, m := range metas {
		if m.Name != "" {
			pluginNameMap[pid] = m.Name
		}
		if m.Logo != "" {
			pluginIconMap[pid] = resolveLogoURL(m.Logo, pid)
		}
	}
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
	// Precompute display strings for the template (Jet logic stays dumb).
	statusParts := make([]string, 0, 3)
	if overview.StatusDone > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d done", overview.StatusDone))
	}
	if overview.StatusOngoing > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d ongoing", overview.StatusOngoing))
	}
	if overview.StatusUnknown > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d unknown", overview.StatusUnknown))
	}
	statusLine := strings.Join(statusParts, " · ")
	if statusLine == "" {
		statusLine = "no data"
	}
	readLine := fmt.Sprintf("%d finished · %d reading", overview.FullyRead, overview.StartedReading)
	readingTime := fmt.Sprintf("%.1f h", float64(overview.PagesRead)*120/3600)
	mostLine := fmt.Sprintf("%d ch", overview.MostCount)
	if overview.MostDup > 1 {
		mostLine = fmt.Sprintf("%d ch · %d titles", overview.MostCount, overview.MostDup)
	} else if len(overview.MostTitle) > 25 {
		mostLine = fmt.Sprintf("%s… · %d ch", overview.MostTitle[:25], overview.MostCount)
	} else if overview.MostTitle != "" {
		mostLine = fmt.Sprintf("%s · %d ch", overview.MostTitle, overview.MostCount)
	}
	fewestLine := fmt.Sprintf("%d ch", overview.FewestCount)
	if overview.FewestDup > 1 {
		fewestLine = fmt.Sprintf("%d ch · %d titles", overview.FewestCount, overview.FewestDup)
	} else if len(overview.FewestTitle) > 25 {
		fewestLine = fmt.Sprintf("%s… · %d ch", overview.FewestTitle[:25], overview.FewestCount)
	} else if overview.FewestTitle != "" {
		fewestLine = fmt.Sprintf("%s · %d ch", overview.FewestTitle, overview.FewestCount)
	}
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
				seen[m.ID] = struct{}{}
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
	s.renderPage(w, r, "views/library", "library", map[string]any{
		"Mangas":          mangas[start:end],
		"Q":               q,
		"Ratios":          ratios,
		"LibraryStats":    statsMap,
		"Page":            page,
		"TotalPages":      max((total+pageSize-1)/pageSize, 1),
		"HasNext":         end < total,
		"HasPrev":         page > 1,
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
