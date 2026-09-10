package httpserver

import (
	"net/http"
	"sort"
	"time"

	"goisekai/internal/database"
)

// viewUpdates renders the updates feed: in-library manga with fresh chapter
// updates (new_since set, i.e. the same signal as the library [New] badge)
// plus titles added to the library in the last 7 days.
func (s *Server) viewUpdates(w http.ResponseWriter, r *http.Request) {
	mangas, err := s.service.ListLibrary()
	if err != nil {
		s.logger.Error("updates list", "error", err)
	}
	libStats, err := s.service.ListLibraryWithProgress()
	if err != nil {
		s.logger.Warn("updates stats", "error", err)
	}

	// Plugin display name/icon resolution, mirroring viewLibrary: DB rows
	// first (survive restarts), runtime metas overlay when fresher.
	dbPlugins, _ := s.service.ListPlugins()
	pluginNameMap := make(map[string]string, len(dbPlugins))
	pluginIconMap := make(map[string]string, len(dbPlugins))
	for _, p := range dbPlugins {
		pluginNameMap[p.ID] = p.Name
		pluginIconMap[p.ID] = p.IconURL
	}
	metas := s.service.PluginMetas()
	for pid, m := range metas {
		if m.Name != "" {
			pluginNameMap[pid] = m.Name
		}
		if m.Logo != "" {
			pluginIconMap[pid] = resolveLogoURL(m.Logo, pid)
		}
	}

	statsMap := make(map[string]database.LibraryMangaStats, len(libStats))
	for _, st := range libStats {
		statsMap[st.MangaID] = st
	}

	buildRow := func(m database.Manga) map[string]any {
		st := statsMap[m.ID]
		name := pluginNameMap[m.PluginID]
		if name == "" {
			name = m.PluginID
		}
		return map[string]any{
			"MangaID":       m.SourceMangaID,
			"PluginID":      m.PluginID,
			"Title":         m.Title,
			"CoverURL":      m.CoverURL,
			"ReadChapters":  st.ReadChapters,
			"TotalChapters": st.TotalChapters,
			"HasNew":        st.HasNew,
			"NewSince":      st.NewSince,
			"CreatedAt":     m.CreatedAt,
			"PluginName":    name,
			"PluginIcon":    pluginIconMap[m.PluginID],
		}
	}

	// Fresh updates: new_since set (cleared when the manga is opened).
	var updMangas []database.Manga
	for _, st := range libStats {
		if st.NewSince == nil {
			continue
		}
		for _, m := range mangas {
			if m.ID == st.MangaID {
				updMangas = append(updMangas, m)
				break
			}
		}
	}
	sort.SliceStable(updMangas, func(i, j int) bool {
		return statsMap[updMangas[i].ID].NewSince.After(*statsMap[updMangas[j].ID].NewSince)
	})
	updates := make([]map[string]any, 0, len(updMangas))
	for _, m := range updMangas {
		updates = append(updates, buildRow(m))
	}

	// Recently added: library titles created in the last 7 days.
	cutoff := time.Now().AddDate(0, 0, -7)
	var recMangas []database.Manga
	for _, m := range mangas {
		if m.CreatedAt.After(cutoff) {
			recMangas = append(recMangas, m)
		}
	}
	sort.SliceStable(recMangas, func(i, j int) bool {
		return recMangas[i].CreatedAt.After(recMangas[j].CreatedAt)
	})
	if len(recMangas) > 24 {
		recMangas = recMangas[:24]
	}
	recent := make([]map[string]any, 0, len(recMangas))
	for _, m := range recMangas {
		recent = append(recent, buildRow(m))
	}

	s.renderPage(w, r, "views/updates", "updates", map[string]any{
		"Updates":      updates,
		"Recent":       recent,
		"UpdatesCount": len(updates),
		"RecentCount":  len(recent),
	})
}
