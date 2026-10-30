package httpserver

import (
	"strconv"

	"goisekai/internal/database"
)

// libraryDupMaps colours the duplicate groups and ranks their members.
//
// dupColors maps a manga row-ID to the colour of its duplicate group. Every
// member of a group shares one colour, so the same story is recognisable
// across the grid at a glance. dupRank gives the group's position, which the
// "duplicates" sort uses to keep a group's members adjacent. The count is the
// number of coloured manga, i.e. how many belong to a duplicate group.
//
// The colour is indexed by group position, so the group order has to be
// deterministic (FindPotentialDuplicates breaks ties on Key) or the bands
// would change colour between page loads.
func libraryDupMaps(duplicateGroups []database.DuplicateGroup) (dupColors map[string]string, dupRank map[string]int, count int) {
	dupColors = make(map[string]string, len(duplicateGroups)*2)
	dupRank = make(map[string]int, len(duplicateGroups)*2)
	for gi, g := range duplicateGroups {
		color := dupPalette[gi%len(dupPalette)]
		for _, m := range g.Members {
			id := strconv.FormatInt(m.ID, 10)
			dupColors[id] = color
			dupRank[id] = gi
		}
	}
	return dupColors, dupRank, len(dupColors)
}

// libraryStatsMap builds the per-card stats map the library view renders
// (mangaID -> {TotalChapters, ReadChapters, PluginName, PluginIcon, HasNew})
// and returns it alongside the mangaID -> pluginID lookup it reads.
//
// Display names + icons: DB rows as base, runtime metas overlay; wasm plugins
// fall back to their on-disk logo.png (see pluginDisplayMaps).
func (s *Server) libraryStatsMap(
	libStats []database.LibraryMangaStats,
	pluginNameMap, pluginIconMap map[string]string,
) (map[string]map[string]any, map[string]string) {
	mangaPluginMap := make(map[string]string) // mangaID -> pluginID (from DB)
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
	return statsMap, mangaPluginMap
}

// libraryPluginCounts returns per-plugin title counts for the sidebar card.
// It only runs on an unfiltered library (no search query): with a query the
// card has nothing meaningful to say about the visible subset.
func (s *Server) libraryPluginCounts(
	q string,
	pluginNameMap, pluginIconMap map[string]string,
) []map[string]any {
	if q != "" {
		return nil
	}
	pc, pcErr := s.service.CountLibraryByPlugin()
	if pcErr != nil {
		s.logger.Warn("library plugin counts", "error", pcErr)
		return nil
	}
	pluginCounts := make([]map[string]any, 0, len(pc))
	for _, p := range pc {
		pluginCounts = append(pluginCounts, map[string]any{
			"PluginID": p.PluginID,
			"Name":     pluginNameMap[p.PluginID],
			"Icon":     pluginIconMap[p.PluginID],
			"Count":    p.Count,
		})
	}
	return pluginCounts
}
