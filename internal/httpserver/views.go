package httpserver

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"

	"goisekai/internal/config"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
	"strings"
)

// fsSub returns the embedded frontend subtree (frontend/…) for /static.
func fsSub() (fs.FS, error) {
	return fs.Sub(defaultAssets, "frontend")
}

// defaultAssets holds the assets FS injected by New so view helpers can
// reach it without threading it through every handler.
var defaultAssets embed.FS

// viewLibrary renders the library grid (also the home page).
func (s *Server) viewLibrary(w http.ResponseWriter, r *http.Request) {
	mangas, err := s.service.ListLibrary()
	if err != nil {
		s.logger.Error("library list", "error", err)
	}
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
	pluginNameMap := make(map[string]string) // pluginID -> display name
	for pid, m := range metas {
		pluginNameMap[pid] = pid
		_ = m
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
		statsMap[st.MangaID] = map[string]any{
			"TotalChapters": st.TotalChapters,
			"ReadChapters":  st.ReadChapters,
			"PluginName":    pluginName,
			"HasNew":        st.HasNew,
		}
	}
	s.renderPage(w, "views/library.jet", "library", map[string]any{
		"Mangas":      mangas,
		"Ratios":      ratios,
		"LibraryStats": statsMap,
	})
}

// viewHistory renders the reading history page.
func (s *Server) viewHistory(w http.ResponseWriter, r *http.Request) {
	history, err := s.service.GetReadHistory()
	if err != nil {
		s.logger.Error("history list", "error", err)
	}
	var entries []database.HistoryEntry
	for _, h := range history {
		h.PluginName = h.PluginID
		entries = append(entries, h)
	}
	s.renderPage(w, "views/history.jet", "history", map[string]any{"History": entries})
}

// viewSearch renders the search form and, when q+pluginID are present, results.
func (s *Server) viewSearch(w http.ResponseWriter, r *http.Request) {
	plugins, err := s.service.ListPlugins()
	if err != nil {
		s.logger.Error("plugin list", "error", err)
	}
	q := r.URL.Query().Get("q")
	pluginID := r.URL.Query().Get("pluginID")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	var results []types.Manga
	challenge := false
	if q != "" && pluginID != "" {
		results, err = s.service.SearchManga(pluginID, types.SearchFilter{Query: q, Page: page})
		if err != nil {
			var ch *hostnet.ChallengeError
			if errors.As(err, &ch) {
				challenge = true
				results = nil
				s.logger.Warn("search blocked by challenge", "plugin", pluginID, "q", q)
			} else {
				s.logger.Error("search", "error", err, "plugin", pluginID, "q", q)
			}
		}
	}
	pageSize := s.service.PluginMeta(pluginID).SearchPageSize
	if pageSize <= 0 {
		pageSize = 24
	}
	// Host-side pagination: plugins return ALL matching results (the search
	// contract per plugin metadata search_page_size); slice out the requested
	// page here so the template never renders more than one page.
	total := len(results)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	s.renderPage(w, "views/search.jet", "search", map[string]any{
		"Plugins":    plugins,
		"Q":          q,
		"PluginID":   pluginID,
		"Results":    results[start:end],
		"Page":       page,
		"HasNext":    end < total,
		"ThumbRatio": s.service.PluginMeta(pluginID).ThumbRatio,
		"Challenge":  challenge,
	})
}

// viewMangaDetail renders a manga's info plus its chapter list.
func (s *Server) viewMangaDetail(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	// Opening the detail page clears the library card's [New] badge.
	if err := s.service.ClearMangaNew(pluginID, mangaID); err != nil {
		s.logger.Warn("clear new badge", "plugin", pluginID, "manga", mangaID, "error", err)
	}
	manga, chapters, err := s.service.GetMangaDetails(pluginID, mangaID)
	challenge := false
	if err != nil {
		var ch *hostnet.ChallengeError
		if errors.As(err, &ch) {
			challenge = true
			s.logger.Warn("manga detail blocked by challenge", "plugin", pluginID, "manga", mangaID)
		} else {
			s.logger.Error("manga detail", "error", err, "plugin", pluginID, "manga", mangaID)
			http.Error(w, "failed to load manga: "+err.Error(), http.StatusBadGateway)
			return
		}
	}
	progress, err := s.service.GetChapterProgresses(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("chapter progress", "error", err, "manga", mangaID)
		progress = map[string]database.ChapterProgress{}
	}
	continueTo := computeContinue(chapters, progress)
	// Prefer most-recently-read chapter from history if it exists and isn't fully read
	if lastCont := s.continueFromHistory(pluginID, mangaID, chapters, progress); lastCont != nil {
		continueTo = lastCont
	}
	inLibrary := s.service.IsInLibrary(pluginID, mangaID)
	s.renderPage(w, "views/detail.jet", "search", map[string]any{
		"PluginID":  pluginID,
		"MangaID":   mangaID,
		"Manga":     manga,
		"Chapters":  chapters,
		"Progress":  progress,
		"Continue":  continueTo,
		"InLibrary": inLibrary,
		"Challenge": challenge,
	})
}

// ContinuePoint names where the Continue button should resume.
type ContinuePoint struct {
	ChapterID string
	ChapterN  float64
	Page      int
}

// computeContinue picks the resume target: the first in-progress chapter,
// else the first unread chapter, else nil when everything is finished.
func computeContinue(chapters []types.Chapter, progress map[string]database.ChapterProgress) *ContinuePoint {
	var firstUnread *ContinuePoint
	for _, c := range chapters {
		p, ok := progress[c.ID]
		if ok && p.LastPageRead > 0 {
			if p.TotalPages == 0 || p.LastPageRead < p.TotalPages {
				return &ContinuePoint{ChapterID: c.ID, ChapterN: c.ChapterNum, Page: p.LastPageRead}
			}
			continue // fully read
		}
		// Chapters arrive newest-first; keep the LAST unread seen so the
		// fallback start point is the numerically lowest chapter.
		firstUnread = &ContinuePoint{ChapterID: c.ID, ChapterN: c.ChapterNum, Page: 1}
	}
	return firstUnread
}

// continueFromHistory checks read_history for the most recently read chapter
// and returns it as the resume point if it's not fully read yet.
func (s *Server) continueFromHistory(pluginID, mangaID string, chapters []types.Chapter, progress map[string]database.ChapterProgress) *ContinuePoint {
	mangaRow := pluginID + "|" + mangaID
	lastChID, lastPage, ok := s.service.LastReadChapter(mangaRow)
	if !ok {
		return nil
	}
	for i, c := range chapters {
		if c.ID == lastChID {
			p, hasProgress := progress[c.ID]
			if !hasProgress || p.LastPageRead < p.TotalPages {
				return &ContinuePoint{ChapterID: c.ID, ChapterN: c.ChapterNum, Page: lastPage}
			}
			// Fully read — advance to the next chapter (higher number = earlier in the slice).
			if i > 0 {
				next := chapters[i-1]
				return &ContinuePoint{ChapterID: next.ID, ChapterN: next.ChapterNum, Page: 1}
			}
			return nil
		}
	}
	return nil
}

// PluginView enriches a database plugin with runtime verify metadata and the
// declared thumbnail ratio for the plugins page.
type PluginView struct {
	database.Plugin
	VerifyURL        string
	NeedsHumanVerify bool
	VerifyCookies    string
	VerifyUserAgent  string
	ThumbRatio       float64 // runtime meta; shadows database.Plugin.ThumbRatio (0 for bridge-installed plugins)
}

// viewPlugins renders the plugin manager page.
func (s *Server) viewPlugins(w http.ResponseWriter, _ *http.Request) {
	plugins, err := s.service.ListPlugins()
	if err != nil {
		s.logger.Error("plugin list", "error", err)
	}
	metas := s.service.PluginMetas()
	views := make([]PluginView, 0, len(plugins))
	for _, p := range plugins {
		v := PluginView{Plugin: p}
		if m, ok := metas[p.ID]; ok {
			v.VerifyURL = m.VerifyURL
			v.NeedsHumanVerify = m.NeedsHumanVerify
			v.ThumbRatio = m.ThumbRatio
		}
		if row, ok, err := s.service.GetPluginVerifyState(p.ID); err == nil && ok {
			v.VerifyCookies = row.Cookies
			v.VerifyUserAgent = row.UserAgent
		}
		views = append(views, v)
	}
	s.renderPage(w, "views/plugins.jet", "plugins", map[string]any{"Plugins": views})
}

// viewSettings renders the current goisekai.ini values.
func (s *Server) viewSettings(w http.ResponseWriter, _ *http.Request) {
	path := s.service.GetConfigPath()
	cfg, err := config.Load(path)
	if err != nil {
		s.logger.Error("config load", "error", err, "path", path)
	}
	s.renderPage(w, "views/settings.jet", "settings", map[string]any{
		"Config": cfg,
		"Path":   path,
	})
}

// viewLogs renders the in-memory log buffer with a 2s HTMX poll.
func (s *Server) viewLogs(w http.ResponseWriter, r *http.Request) {
	limit := 500
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
			if limit > 2000 {
				limit = 2000
			}
		}
	}
	logs := s.service.GetLogs()
	filter := r.URL.Query().Get("filter") // "", "app" or "plugins"
	switch filter {
	case "app":
		out := logs[:0]
		for _, l := range logs {
			if !strings.Contains(l, " plugin=") {
				out = append(out, l)
			}
		}
		logs = out
	case "plugins":
		out := logs[:0]
		for _, l := range logs {
			if strings.Contains(l, " plugin=") {
				out = append(out, l)
			}
		}
		logs = out
	}
	if len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}
	s.renderPage(w, "views/logs.jet", "logs", map[string]any{"Logs": logs, "Limit": limit, "Limits": []int{100, 250, 500, 1000, 2000}, "Filter": filter})
}

// jsonMarshalLogs writes lines as a JSON array.
func jsonMarshalLogs(w http.ResponseWriter, lines []string) error {
	enc := json.NewEncoder(w)
	return enc.Encode(lines)
}
