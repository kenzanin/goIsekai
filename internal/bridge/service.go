// Package bridge is the final service layer binding plugin results and SQLite
// persistence to the frontend. It delegates search/detail/page lookups to the
// plugin manager, mirrors fetched manga into SQLite so progress can be tracked,
// and proxies image fetches through the sandboxed hostnet proxy.
package bridge

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/enrich"
	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
	"goisekai/pkg/types"
)

// AppService wires the plugin manager, hostnet proxy, and SQLite database into
// the single entry point the frontend calls.
type AppService struct {
	db         *database.DB
	mgr        *pluginmanager.Manager
	proxy      *hostnet.Proxy
	cfgPath    string
	cacheDir   string
	imageMu    sync.RWMutex
	imageCache map[string][]byte
	imgSem     chan struct{} // caps concurrent image fetches per host process
	imgPaceMu  sync.Mutex
	imgPace    map[string]time.Time // host -> earliest allowed next request (MD@Home pacing)
	enrich     *enrich.Registry
}

// NewAppService returns an AppService backed by the supplied database, plugin
// manager, hostnet proxy, and enrichment registry.
func NewAppService(db *database.DB, mgr *pluginmanager.Manager, proxy *hostnet.Proxy, cfgPath, cacheDir string, enrichReg *enrich.Registry) *AppService {
	return &AppService{
		db:         db,
		mgr:        mgr,
		proxy:      proxy,
		cfgPath:    cfgPath,
		cacheDir:   cacheDir,
		imageCache: make(map[string][]byte),
		enrich:     enrichReg,
	}
}

// Log receives a console message from the frontend and writes it to the Go logger.
func (s *AppService) Log(level string, msg string) {
	switch level {
	case "error":
		logger.Error("[ui] " + msg)
	case "warn":
		logger.Warn("[ui] " + msg)
	default:
		logger.Debug("[ui] " + msg)
	}
}

// GetConfigPath returns the path to goisekai.ini.
func (s *AppService) GetConfigPath() string {
	return s.cfgPath
}

// CDPStatus returns the current CDP engine configuration.
func (s *AppService) CDPStatus() hostnet.CDPConfig {
	return s.proxy.CDPConfig()
}

// CDPCookies returns cookies from all per-plugin jars matching the domain.
func (s *AppService) CDPCookies(domain string) []hostnet.CDPCookie {
	return s.proxy.CDPCookies(domain)
}

// GetChapterList fetches the chapter list for a manga from the plugin.
func (s *AppService) GetChapterList(pluginID, mangaID string) ([]types.Chapter, error) {
	chapters, err := s.mgr.GetChapterList(pluginID, mangaID)
	if err != nil {
		return nil, fmt.Errorf("bridge: get chapter list: %w", err)
	}
	return chapters, nil
}

// TestCDP launches the configured CDP engine against the given URL, waits for
// the challenge to clear, and returns the harvested cookies and User-Agent.
func (s *AppService) TestCDP(targetURL string) ([]hostnet.CDPCookie, string, error) {
	cfg := s.proxy.CDPConfig()
	if cfg.Engine == "" || cfg.Engine == "off" {
		return nil, "", fmt.Errorf("CDP engine is not configured")
	}
	cookies, ua, err := s.proxy.TestCDP(cfg, targetURL)
	if err != nil {
		return nil, "", err
	}
	var out []hostnet.CDPCookie
	for _, c := range cookies {
		out = append(out, hostnet.CDPCookie{
			Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: c.Path, Secure: c.Secure, HTTPOnly: c.HttpOnly,
		})
	}
	return out, ua, nil
}

// ListLibraryWithProgress returns per-manga chapter stats for the library grid.
func (s *AppService) ListLibraryWithProgress() ([]database.LibraryMangaStats, error) {
	return s.db.ListLibraryWithProgress()
}

// LibraryOverview returns aggregated library-wide stats for the stats row.
func (s *AppService) LibraryOverview() (database.LibraryOverview, error) {
	return s.db.LibraryOverview()
}

// FindPotentialDuplicates returns groups of in-library manga that share a
// normalised title or alternative title.
func (s *AppService) FindPotentialDuplicates() ([]database.DuplicateGroup, error) {
	return s.db.FindPotentialDuplicates()
}

// CountLibraryByPlugin returns per-plugin in-library title counts.
func (s *AppService) CountLibraryByPlugin() ([]database.PluginCount, error) {
	return s.db.CountLibraryByPlugin()
}

// GetReadHistory returns the reading history enriched with plugin names.
func (s *AppService) GetReadHistory() ([]database.HistoryEntry, error) {
	entries, err := s.db.GetReadHistory()
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// LastReadChapter returns the most recently read chapter for a manga.
func (s *AppService) LastReadChapter(mangaRowID string) (sourceChapterID string, pageNum int, ok bool) {
	return s.db.LastReadChapter(mangaRowID)
}

// QueryMangaPluginIDs returns (manga_id, plugin_id) pairs for all in-library manga.
func (s *AppService) QueryMangaPluginIDs() ([]database.MangaPluginIDRow, error) {
	return s.db.QueryMangaPluginIDs()
}

// PluginDir returns the directory containing the plugin's main file, or "" if
// not found. Folder-based plugins (lua/js/yaegi) have WasmPath pointing at
// the plugin folder directly.
func (s *AppService) PluginDir(pluginID string) string {
	for _, p := range s.mgr.LoadedPlugins() {
		if p.ID != pluginID || p.WasmPath == "" {
			continue
		}
		if fi, err := os.Stat(p.WasmPath); err == nil && fi.IsDir() {
			return p.WasmPath
		}
		return filepath.Dir(p.WasmPath)
	}
	return ""
}

// SyncPluginMeta persists a loaded plugin's identity metadata (name, logo)
// to the database so deferred plugins show correct data on the next page view.
func (s *AppService) SyncPluginMeta(id string) {
	meta := s.PluginMeta(id)
	if meta.Name == "" && meta.Logo == "" {
		return
	}
	iconURL := meta.Logo
	if iconURL != "" && !strings.HasPrefix(iconURL, "http") && !strings.HasPrefix(iconURL, "data:") {
		iconURL = "/plugin-static/" + id + "/" + iconURL
	}
	if err := s.db.UpdatePluginIdentity(id, meta.Name, iconURL); err != nil {
		logger.Warn("sync plugin meta", "id", id, "error", err)
	}
}

// ListCategories returns enrichment categories for a manga from the database.
func (s *AppService) ListCategories(pluginID, mangaID string) ([]database.EnrichmentRow, error) {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return nil, err
	}
	return s.db.ListEnrichment(rowID, "categories")
}

// ListRelated returns enrichment related/recommended manga for a manga from the database.
func (s *AppService) ListRelated(pluginID, mangaID string) ([]database.EnrichmentRow, error) {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return nil, err
	}
	return s.db.ListEnrichment(rowID, "related")
}

// FetchEnrichment fetches enrichment data from external sources and stores it.
func (s *AppService) FetchEnrichment(pluginID, mangaID, title string, sources []string) error {
	if s.enrich == nil {
		return fmt.Errorf("enrichment provider not configured")
	}
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return err
	}
	logger.Debug("enrich fetch start", "title", title, "sources", sources)
	items := s.enrich.FetchAll(context.Background(), &http.Client{}, title, sources)

	// Store alt titles.
	if titles, ok := items[enrich.KindTitles]; ok && len(titles) > 0 {
		names := make([]string, len(titles))
		for i, t := range titles {
			names[i] = t.Value
		}
		n, err := s.db.AddAltTitles(rowID, names, titles[0].Source)
		if err != nil {
			logger.Warn("store titles", "error", err)
		} else {
			logger.Info("enrich titles stored", "count", len(titles), "inserted", n, "source", titles[0].Source)
		}
	} else {
		logger.Debug("enrich titles: none found")
	}

	// Store alt summaries.
	if summs, ok := items[enrich.KindSummaries]; ok && len(summs) > 0 {
		names := make([]string, len(summs))
		for i, s := range summs {
			names[i] = s.Value
		}
		n, err := s.db.AddAltDescriptions(rowID, names, summs[0].Source)
		if err != nil {
			logger.Warn("store summaries", "error", err)
		} else {
			logger.Info("enrich summaries stored", "count", len(summs), "inserted", n, "source", summs[0].Source)
		}
	} else {
		logger.Debug("enrich summaries: none found")
	}

	// Store categories.
	if cats, ok := items[enrich.KindCategories]; ok && len(cats) > 0 {
		names := make([]string, len(cats))
		for i, c := range cats {
			names[i] = c.Value
		}
		n, err := s.db.AddCategories(rowID, names, cats[0].Source)
		if err != nil {
			logger.Warn("store categories", "error", err)
		} else {
			logger.Info("enrich categories stored", "count", len(cats), "inserted", n, "source", cats[0].Source)
		}
	} else {
		logger.Debug("enrich categories: none found")
	}

	// Store related manga.
	if rels, ok := items[enrich.KindRelated]; ok && len(rels) > 0 {
		rows := make([]database.RelatedRow, len(rels))
		for i, r := range rels {
			rows[i] = database.RelatedRow{Title: r.Value, URL: r.URL, Source: r.Source}
		}
		n, err := s.db.AddRelated(rowID, rows, rels[0].Source)
		if err != nil {
			logger.Warn("store related", "error", err)
		} else {
			logger.Info("enrich related stored", "count", len(rels), "inserted", n, "source", rels[0].Source)
		}
	} else {
		logger.Debug("enrich related: none found")
	}
	return nil
}

// EnrichmentSources returns all registered enrichment source IDs.
func (s *AppService) EnrichmentSources() []string {
	if s.enrich == nil {
		return nil
	}
	entries := s.enrich.Catalog("")
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}

// EnrichmentResult holds all enrichment data for a manga.
type EnrichmentResult struct {
	AltTitles    []database.EnrichmentRow
	AltSummaries []database.EnrichmentRow
	Categories   []database.EnrichmentRow
	Related      []database.EnrichmentRow
}

// EnrichmentCatalogEntry describes one enrichment source for the UI.
type EnrichmentCatalogEntry struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Kinds []string `json:"kinds"`
}

// EnrichmentCatalog returns all registered enrichment sources (built-in +
// plugin-declared). The caller passes an optional kind filter; empty string
// returns all.
func (s *AppService) EnrichmentCatalog(kind string) []EnrichmentCatalogEntry {
	if s.enrich == nil {
		return nil
	}
	entries := s.enrich.Catalog(enrich.Kind(kind))
	out := make([]EnrichmentCatalogEntry, 0, len(entries))
	for _, e := range entries {
		kinds := make([]string, len(e.Kinds))
		for i, k := range e.Kinds {
			kinds[i] = string(k)
		}
		out = append(out, EnrichmentCatalogEntry{
			ID:    e.ID,
			Name:  e.Name,
			Kinds: kinds,
		})
	}
	return out
}

// RemoveCategory deletes a single category from a manga's stored enrichment data.
func (s *AppService) RemoveCategory(pluginID, mangaID, category string) error {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return err
	}
	return s.db.RemoveCategory(rowID, category)
}

// AddCategory adds a category to a manga's stored enrichment data.
func (s *AppService) AddCategory(pluginID, mangaID, category string) error {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return err
	}
	return s.db.AddCategory(rowID, category)
}

// RemoveRelated deletes a single related/recommended manga from storage.
func (s *AppService) RemoveRelated(pluginID, mangaID, title string) error {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return err
	}
	return s.db.RemoveRelated(rowID, title)
}

// GetEnrichment returns all enrichment data for a manga from the database.
func (s *AppService) GetEnrichment(pluginID, mangaID string) (*EnrichmentResult, error) {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return nil, err
	}

	res := &EnrichmentResult{}
	res.AltTitles, err = s.db.ListEnrichment(rowID, "alt_titles")
	if err != nil {
		return nil, err
	}
	res.AltSummaries, err = s.db.ListEnrichment(rowID, "alt_summaries")
	if err != nil {
		return nil, err
	}
	res.Categories, err = s.db.ListEnrichment(rowID, "categories")
	if err != nil {
		return nil, err
	}
	res.Related, err = s.db.ListEnrichment(rowID, "related")
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ResetEnrichment deletes all user enrichment data (alt titles, summaries,
// categories, related) and restores title/synopsis/genres to original plugin values.
func (s *AppService) ResetEnrichment(pluginID, mangaID string) error {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return err
	}
	if err := s.db.ResetEnrichment(rowID); err != nil {
		return fmt.Errorf("reset enrichment: %w", err)
	}
	if err := s.db.SetMangaGenres(rowID, nil); err != nil {
		return fmt.Errorf("reset genres: %w", err)
	}
	return nil
}

// CacheStats returns cache metrics for the /api/stats endpoint.
func (s *AppService) CacheStats() (total, hits int64, err error) {
	return s.db.CacheStats()
}
