// Package bridge is the final service layer binding plugin results and SQLite
// persistence to the frontend. It delegates search/detail/page lookups to the
// plugin manager, mirrors fetched manga into SQLite so progress can be tracked,
// and proxies image fetches through the sandboxed hostnet proxy.
package bridge

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"goisekai/internal/database"
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
	imageMu    sync.RWMutex
	imageCache map[string][]byte
}

// NewAppService returns an AppService backed by the supplied database, plugin
// manager, and hostnet proxy.
func NewAppService(db *database.DB, mgr *pluginmanager.Manager, proxy *hostnet.Proxy) *AppService {
	return &AppService{
		db:         db,
		mgr:        mgr,
		proxy:      proxy,
		imageCache: make(map[string][]byte),
	}
}

// SearchManga delegates to the plugin's Search function.
func (s *AppService) SearchManga(pluginID string, filter types.SearchFilter) ([]types.Manga, error) {
	result, err := s.mgr.Search(pluginID, filter)
	if err != nil {
		return nil, fmt.Errorf("bridge: search manga: %w", err)
	}
	return result, nil
}

// GetMangaDetails fetches a manga and its chapter list from a plugin, persists
// both to the database as a side effect (so progress can be tracked later),
// and returns the original plugin types unchanged.
func (s *AppService) GetMangaDetails(pluginID, mangaID string) (types.Manga, []types.Chapter, error) {
	manga, err := s.mgr.GetMangaDetail(pluginID, mangaID)
	if err != nil {
		return types.Manga{}, nil, fmt.Errorf("bridge: get manga detail: %w", err)
	}
	chapters, err := s.mgr.GetChapterList(pluginID, mangaID)
	if err != nil {
		return types.Manga{}, nil, fmt.Errorf("bridge: get chapter list: %w", err)
	}
	if err := s.persistMangaDetails(pluginID, manga, chapters); err != nil {
		return types.Manga{}, nil, fmt.Errorf("bridge: persist manga details: %w", err)
	}
	return manga, chapters, nil
}

// GetPageList delegates to the plugin's GetPageList function.
func (s *AppService) GetPageList(pluginID, chapterID string) ([]types.Page, error) {
	result, err := s.mgr.GetPageList(pluginID, chapterID)
	if err != nil {
		return nil, fmt.Errorf("bridge: get page list: %w", err)
	}
	return result, nil
}

// ToggleLibraryItem flips the in-library flag for a manga, addressed by its
// source identifiers (pluginID + source manga id) rather than the internal
// database row id. The bridge reconstructs the row id internally so the
// frontend never needs to know the storage key scheme.
func (s *AppService) ToggleLibraryItem(pluginID, mangaID string) error {
	if err := s.db.ToggleLibrary(mangaRowID(pluginID, mangaID)); err != nil {
		return fmt.Errorf("bridge: toggle library item: %w", err)
	}
	return nil
}

// InstallPlugin copies a plugin wasm into the managed plugins directory,
// hot-loads it, and registers it in the database as active. The plugin id is
// derived from the file basename (minus the .wasm extension); WasmPath points
// at the copy inside the plugins directory so it survives a restart.
func (s *AppService) InstallPlugin(wasmPath string) error {
	dest, err := s.mgr.Install(wasmPath)
	if err != nil {
		return fmt.Errorf("bridge: install plugin: %w", err)
	}
	id := strings.TrimSuffix(filepath.Base(dest), ".wasm")
	if err := s.db.RegisterPlugin(database.Plugin{
		ID:       id,
		Name:     id,
		Version:  "",
		WasmPath: dest,
		IsActive: true,
	}); err != nil {
		return fmt.Errorf("bridge: register plugin: %w", err)
	}
	return nil
}

// RecordRead appends a read-history entry for a chapter's current page,
// addressed by source identifiers.
func (s *AppService) RecordRead(pluginID, mangaID, chapterID string, pageNum int) error {
	if err := s.db.RecordRead(chapterRowID(pluginID, mangaID, chapterID), pageNum); err != nil {
		return fmt.Errorf("bridge: record read: %w", err)
	}
	return nil
}

// SetChapterProgress records a chapter's last page read and marks it read,
// addressed by source identifiers.
func (s *AppService) SetChapterProgress(pluginID, mangaID, chapterID string, lastPage int) error {
	if err := s.db.SetChapterProgress(chapterRowID(pluginID, mangaID, chapterID), lastPage); err != nil {
		return fmt.Errorf("bridge: set chapter progress: %w", err)
	}
	return nil
}

// ListLibrary returns the user's in-library manga, most recently updated first.
func (s *AppService) ListLibrary() ([]database.Manga, error) {
	list, err := s.db.ListLibrary()
	if err != nil {
		return nil, fmt.Errorf("bridge: list library: %w", err)
	}
	return list, nil
}

// TogglePlugin flips the is_active flag for a plugin.
func (s *AppService) TogglePlugin(id string) error {
	return s.db.TogglePluginActive(id)
}

// SyncLibrary re-fetches chapter lists from source plugins for every manga in the library.
func (s *AppService) SyncLibrary() error {
	library, err := s.db.ListLibrary()
	if err != nil {
		return fmt.Errorf("bridge: sync library: %w", err)
	}
	for _, manga := range library {
		m, detailErr := s.mgr.GetMangaDetail(manga.PluginID, manga.SourceMangaID)
		if detailErr != nil {
			logger.Error("sync detail failed", "id", manga.ID, "plugin", manga.PluginID, "error", detailErr)
			continue
		}
		chapters, chapErr := s.mgr.GetChapterList(manga.PluginID, manga.SourceMangaID)
		if chapErr != nil {
			logger.Error("sync chapters failed", "id", manga.ID, "error", chapErr)
			continue
		}
		if persistErr := s.persistMangaDetails(manga.PluginID, m, chapters); persistErr != nil {
			logger.Error("sync persist failed", "id", manga.ID, "error", persistErr)
		}
	}
	return nil
}

// ListPlugins returns all registered plugins.
func (s *AppService) ListPlugins() ([]database.Plugin, error) {
	list, err := s.db.ListPlugins()
	if err != nil {
		return nil, fmt.Errorf("bridge: list plugins: %w", err)
	}
	return list, nil
}

// persistMangaDetails mirrors a fetched manga and its chapters into SQLite.
//
// database.Manga.ID is set to a stable, globally-unique key derived from the
// plugin id and source manga id. UpsertManga takes the caller-provided ID as
// the row id (it returns no generated id), and the row id is a plain TEXT
// primary key unique across every plugin — qualifying it with the plugin id
// keeps distinct sources from colliding while still letting the upsert's
// UNIQUE(plugin_id, source_manga_id) conflict clause do its job. The same key
// is reused as each chapter's manga_id, so the row id is known without a
// read-back query (ListLibrary can't be used for that: it filters in_library=1,
// but a freshly upserted manga is in_library=0).
func (s *AppService) persistMangaDetails(pluginID string, m types.Manga, chapters []types.Chapter) error {
	rowID := mangaRowID(pluginID, m.ID)
	if err := s.db.UpsertManga(database.Manga{
		ID:            rowID,
		PluginID:      pluginID,
		SourceMangaID: m.ID,
		Title:         m.Title,
		CoverURL:      m.CoverURL,
		Description:   m.Description,
		Status:        m.Status,
	}); err != nil {
		return err
	}
	for _, c := range chapters {
		if err := s.db.UpsertChapter(database.Chapter{
			ID:              mangaRowID(rowID, c.ID),
			MangaID:         rowID,
			SourceChapterID: c.ID,
			Title:           c.Title,
			ChapterNum:      c.ChapterNum,
			VolumeNum:       c.VolumeNum,
			FetchedAt:       c.ReleasedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

// mangaRowID builds the stable database primary key for a source row:
// "<pluginID>|<sourceID>". The pipe separator cannot appear in a plugin id
// (it is a trimmed base filename) and is extremely unlikely in a source id,
// keeping the hierarchy unambiguous. Upgrade to a delimiter-free scheme only if
// a source id is ever observed containing '|'.
func mangaRowID(pluginID, sourceID string) string {
	return pluginID + "|" + sourceID
}

// chapterRowID builds the database primary key for a chapter row:
// "<pluginID>|<sourceMangaID>|<sourceChapterID>".
func chapterRowID(pluginID, mangaID, chapterID string) string {
	return mangaRowID(pluginID, mangaID) + "|" + chapterID
}
