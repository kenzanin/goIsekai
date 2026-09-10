package httpserver

import (
	"net/http"
	"sort"
	"strconv"
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

	buildRow := func(m database.Manga, typ, date string) map[string]any {
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
			"Type":          typ,
			"Date":          date,
			"PluginName":    name,
			"PluginIcon":    pluginIconMap[m.PluginID],
		}
	}

	// Merge fresh chapter updates (new_since set) and recently-added titles
	// into one date-sorted feed, then paginate host-side.
	var items []map[string]any
	for _, st := range libStats {
		if st.NewSince == nil {
			continue
		}
		for _, m := range mangas {
			if m.ID == st.MangaID {
				items = append(items, buildRow(m, "update", st.NewSince.UTC().Format(time.RFC3339)))
				break
			}
		}
	}
	cutoff := time.Now().AddDate(0, 0, -7)
	for _, m := range mangas {
		if m.CreatedAt.After(cutoff) {
			items = append(items, buildRow(m, "recent", m.CreatedAt.UTC().Format(time.RFC3339)))
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i]["Date"].(string) > items[j]["Date"].(string)
	})

	const pageSize = 24
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	total := len(items)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)

	s.renderPage(w, r, "views/updates", "updates", map[string]any{
		"Items":      items[start:end],
		"Page":       page,
		"TotalPages": max((total+pageSize-1)/pageSize, 1),
	})
}
