package httpserver

import (
	"net/http"
	"sort"
	"strconv"

	"goisekai/internal/database"
)

// apiLibraryFacetsItem is one plugin or category row in the sidebar payload.
type apiLibraryFacetsItem struct {
	PluginID string `json:"plugin_id,omitempty"`
	Name     string `json:"name"`
	Icon     string `json:"icon,omitempty"`
	Count    int    `json:"count"`
}

// apiLibraryFacets is the desktop client's sidebar payload. The web page gets
// this by rendering library.lua server-side; a QML client has no template
// engine, so it reads the same AppService values as JSON instead. Nothing here
// is a new query — every field comes from a method viewLibrary already calls.
func (s *Server) apiLibraryFacets(w http.ResponseWriter, r *http.Request) {
	overview, err := s.service.LibraryOverview()
	if err != nil {
		// A failed overview must not take the sidebar down; zeros render as
		// empty cards, which is honest about missing data.
		s.logger.Warn("api library facets overview", "error", err)
	}

	names, icons := s.pluginDisplayMaps()
	pluginCounts, err := s.service.CountLibraryByPlugin()
	if err != nil {
		s.logger.Warn("api library facets plugin counts", "error", err)
	}
	plugins := make([]apiLibraryFacetsItem, 0, len(pluginCounts))
	for _, p := range pluginCounts {
		name := names[p.PluginID]
		if name == "" {
			name = p.PluginID
		}
		plugins = append(plugins, apiLibraryFacetsItem{
			PluginID: p.PluginID,
			Name:     name,
			Icon:     icons[p.PluginID],
			Count:    p.Count,
		})
	}

	catCounts := s.service.LibraryCategoryCountsOrEmpty()
	categories := make([]apiLibraryFacetsItem, 0, len(catCounts))
	for name, count := range catCounts {
		categories = append(categories, apiLibraryFacetsItem{Name: name, Count: count})
	}
	// Map iteration is random; without this the sidebar reorders on refresh.
	sort.Slice(categories, func(i, j int) bool {
		if categories[i].Count != categories[j].Count {
			return categories[i].Count > categories[j].Count
		}
		return categories[i].Name < categories[j].Name
	})

	// LibraryCategories is keyed by DB row ID, which the client cannot use.
	// Re-key onto plugin_id:source_manga_id so a card's filter key matches.
	tags := make(map[string][]string)
	byRow, err := s.service.ListLibraryCategories()
	if err != nil {
		s.logger.Warn("api library facets categories", "error", err)
	}
	if len(byRow) > 0 {
		mangas, listErr := s.service.ListLibrary()
		if listErr != nil {
			s.logger.Warn("api library facets list", "error", listErr)
		}
		for _, m := range mangas {
			if cats, ok := byRow[strconv.FormatInt(m.ID, 10)]; ok {
				tags[m.PluginID+":"+m.SourceMangaID] = cats
			}
		}
	}

	duplicateCount := 0
	if groups, dupErr := s.service.FindPotentialDuplicates(); dupErr != nil {
		s.logger.Warn("api library facets duplicates", "error", dupErr)
	} else {
		for _, g := range groups {
			duplicateCount += len(g.Members)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"overview":        overviewPayload(overview),
		"plugins":         plugins,
		"categories":      categories,
		"tags":            tags,
		"duplicate_count": duplicateCount,
	})
}

// overviewPayload flattens database.LibraryOverview into the snake_case shape
// the client reads. The web page formats these into sentences; the desktop
// client renders the numbers directly, so raw counts travel, not strings.
func overviewPayload(o database.LibraryOverview) map[string]any {
	return map[string]any{
		"total_titles":    o.TotalTitles,
		"status_done":     o.StatusDone,
		"status_ongoing":  o.StatusOngoing,
		"status_unknown":  o.StatusUnknown,
		"fully_read":      o.FullyRead,
		"started_reading": o.StartedReading,
		"has_updates":     o.HasUpdates,
		"pages_read":      o.PagesRead,
		"most_title":      o.MostTitle,
		"most_count":      o.MostCount,
		"most_dup":        o.MostDup,
		"fewest_title":    o.FewestTitle,
		"fewest_count":    o.FewestCount,
		"fewest_dup":      o.FewestDup,
	}
}
