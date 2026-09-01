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
	for id, m := range s.service.PluginMetas() {
		ratios[id] = m.ThumbRatio
	}
	s.renderPage(w, "views/library.jet", "library", map[string]any{"Mangas": mangas, "Ratios": ratios})
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
	s.renderPage(w, "views/search.jet", "search", map[string]any{
		"Plugins":    plugins,
		"Q":          q,
		"PluginID":   pluginID,
		"Results":    results,
		"Page":       page,
		"HasNext":    len(results) == 24,
		"ThumbRatio": s.service.PluginMeta(pluginID).ThumbRatio,
		"Challenge":  challenge,
	})
}

// viewMangaDetail renders a manga's info plus its chapter list.
func (s *Server) viewMangaDetail(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
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
		if firstUnread == nil {
			firstUnread = &ContinuePoint{ChapterID: c.ID, ChapterN: c.ChapterNum, Page: 1}
		}
	}
	return firstUnread
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
