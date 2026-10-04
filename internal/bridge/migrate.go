package bridge

import (
	"context"
	"sync"

	"database/sql"
	"fmt"
	"strings"

	"goisekai/internal/database"
	"goisekai/internal/logger"
	"goisekai/internal/pluginutil"
	"goisekai/pkg/types"
)

// MigrationCandidate is one result from searching another source for the same title.
type MigrationCandidate struct {
	PluginID      string
	PluginName    string
	SourceMangaID string
	Title         string
	IsExactMatch  bool
}

// CollectMigrationCandidates searches every active source (except the entry's own)
// for the entry's display title. Every returned result is a candidate; an empty
// result set with nil error is not a failure. A per-plugin error skips that
// source; only when every searched source fails does the method return an
// error (sources unreachable, distinct from "no candidates").
func (s *AppService) CollectMigrationCandidates(pluginID, mangaID string) ([]MigrationCandidate, []string, error) {
	// title resolution happens below
	// Resolve display title: prefer DB title, fall back to cached manga title.
	displayTitle := ""
	if m, err := s.db.GetMangaCached(pluginID, mangaID); err == nil && m.Title != "" {
		displayTitle = m.Title
	}
	if displayTitle == "" {
		// Try live detail as last resort for title.
		if s.mgr != nil {
			if live, lerr := s.mgr.GetMangaDetail(pluginID, mangaID); lerr == nil && live.Title != "" {
				displayTitle = live.Title
			}
		}
	}
	if strings.TrimSpace(displayTitle) == "" {
		return nil, nil, fmt.Errorf("migration: cannot determine title for %s/%s", pluginID, mangaID)
	}
	return s.collectCandidatesWithTitle(pluginID, displayTitle)
}

func (s *AppService) collectCandidatesWithTitle(currentPluginID, title string) ([]MigrationCandidate, []string, error) {
	normalizedTitle := pluginutil.NormalizeTitle(title)
	var candidates []MigrationCandidate
	var failures []string
	searched := 0
	if s.mgr == nil {
		return nil, nil, fmt.Errorf("migration: plugin manager not available")
	}
	// Active map from DB (is_active only written on insert, DB is source of truth).
	activeMap := make(map[string]bool)
	if dbPlugins, err := s.db.ListPlugins(); err == nil {
		for _, dp := range dbPlugins {
			activeMap[dp.ID] = dp.IsActive
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, p := range s.mgr.LoadedPlugins() {
		if p.ID == currentPluginID {
			continue
		}
		if active, ok := activeMap[p.ID]; ok && !active {
			continue
		}
		searched++
		wg.Add(1)
		go func(plugID, plugName string) {
			defer wg.Done()
			// task 3.1: each source's search rides the fetch lane under its own
			// PluginKey, so one slow source serializes only its own lookups while
			// the other sources keep going.
			searchErr := s.runOnFetch(context.Background(), plugID, func(context.Context) error {
				results, err := s.mgr.Search(plugID, types.SearchFilter{Query: title})
				if err != nil {
					return err
				}
				if len(results) == 0 {
					return nil
				}
				local := make([]MigrationCandidate, 0, len(results))
				for _, r := range results {
					local = append(local, MigrationCandidate{
						PluginID:      plugID,
						PluginName:    plugName,
						SourceMangaID: r.ID,
						Title:         r.Title,
						IsExactMatch:  pluginutil.NormalizeTitle(r.Title) == normalizedTitle,
					})
				}
				mu.Lock()
				candidates = append(candidates, local...)
				mu.Unlock()
				return nil
			})
			if searchErr != nil {
				logger.Warn("migration candidate search failed", "plugin", plugID, "error", searchErr)
				mu.Lock()
				failures = append(failures, plugID)
				mu.Unlock()
			}
		}(p.ID, p.Name)
	}
	wg.Wait()
	if searched > 0 && len(candidates) == 0 && len(failures) == searched {
		return nil, failures, fmt.Errorf("migration: sources could not be reached")
	}
	return candidates, failures, nil
}

// AutoSelectCandidate returns the single exact match when there is exactly one,
// otherwise nil (caller must present the list).
func AutoSelectCandidate(candidates []MigrationCandidate) *MigrationCandidate {
	var exact *MigrationCandidate
	count := 0
	for i := range candidates {
		if candidates[i].IsExactMatch {
			count++
			if exact == nil {
				exact = &candidates[i]
			}
		}
	}
	if count == 1 {
		return exact
	}
	return nil
}

// MigrateMangaSource repoints a library entry at another source. It fetches the
// target's detail and chapter list before opening the transaction, refuses when
// the target already exists, then runs the mutation as one transaction ordered
// per design.md: repoint row, insert target chapters, apply read boundary,
// delete non-target chapters, delete old plugin_cache rows, re-index FTS.
func (s *AppService) MigrateMangaSource(oldPluginID, oldMangaID, newPluginID, newMangaID string) error {
	if s.mgr == nil {
		return fmt.Errorf("migration: plugin manager not available")
	}
	// 1. Fetch target detail + chapters before the transaction (no write lock across network).
	targetManga, err := s.mgr.GetMangaDetail(newPluginID, newMangaID)
	if err != nil {
		return fmt.Errorf("migration: fetch target detail: %w", err)
	}
	targetChapters, err := s.mgr.GetChapterList(newPluginID, newMangaID)
	if err != nil {
		return fmt.Errorf("migration: fetch target chapters: %w", err)
	}
	if targetManga.Title == "" {
		// Host already guards empty cache writes; still validate here.
		targetManga.Title = newMangaID
	}

	// 2. Resolve old row id and refuse if target already exists (including ghost rows).
	oldRowID, err := s.db.ResolveMangaIntID(oldPluginID, oldMangaID)
	if err != nil {
		return fmt.Errorf("migration: resolve old manga: %w", err)
	}
	if oldRowID == 0 {
		return fmt.Errorf("migration: original entry not found")
	}
	if existing, _ := s.db.GetMangaCached(newPluginID, newMangaID); existing.ID != 0 {
		return fmt.Errorf("migration: target %s/%s already exists in mangas", newPluginID, newMangaID)
	}

	// 3. Collect old read chapter source IDs for the boundary step.
	oldReadIDs, err := s.collectReadSourceIDs(oldRowID)
	if err != nil {
		return fmt.Errorf("migration: collect read ids: %w", err)
	}

	// 4. Run the mutation as one transaction, delete last.
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("migration: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := repointMangaTx(tx, oldRowID, newPluginID, newMangaID, targetManga); err != nil {
		return fmt.Errorf("migration: repoint: %w", err)
	}
	if err := upsertChaptersTx(tx, oldRowID, targetChapters); err != nil {
		return fmt.Errorf("migration: insert chapters: %w", err)
	}
	if len(oldReadIDs) > 0 {
		if err := setChaptersUpToTx(tx, oldRowID, oldReadIDs); err != nil {
			return fmt.Errorf("migration: carry read state: %w", err)
		}
	}
	// Delete old chapters not in the target set (cascades chapter_pages + read_history).
	keepIDs := make([]string, len(targetChapters))
	for i, c := range targetChapters {
		keepIDs[i] = c.ID
	}
	if err := deleteChaptersNotInTx(tx, oldRowID, keepIDs); err != nil {
		return fmt.Errorf("migration: prune old chapters: %w", err)
	}
	// Delete old plugin_cache rows.
	if _, err := tx.Exec(`DELETE FROM plugin_cache WHERE plugin_id = ? AND manga_id = ?`, oldPluginID, oldMangaID); err != nil {
		return fmt.Errorf("migration: clean cache: %w", err)
	}
	// Re-index FTS.
	if err := database.SyncFTSTx(tx, fmt.Sprintf("%d", oldRowID)); err != nil {
		return fmt.Errorf("migration: reindex: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration: commit: %w", err)
	}
	return nil
}

func (s *AppService) collectReadSourceIDs(mangaID int64) ([]string, error) {
	return s.db.ReadSourceIDs(mangaID)
}

func repointMangaTx(tx *sql.Tx, mangaID int64, pluginID, sourceMangaID string, m types.Manga) error {
	_, err := tx.Exec(
		`UPDATE mangas SET plugin_id = ?, source_manga_id = ?, title = CASE WHEN custom_title = 1 THEN title ELSE ? END, cover_url = ?, description = CASE WHEN custom_description = 1 THEN description ELSE ? END, status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		pluginID, sourceMangaID, m.Title, m.CoverURL, m.Description, m.Status, mangaID,
	)
	return err
}

func upsertChaptersTx(tx *sql.Tx, mangaID int64, chapters []types.Chapter) error {
	for _, c := range chapters {
		_, err := tx.Exec(
			`INSERT INTO chapters (manga_id, source_chapter_id, title, chapter_num, volume_num) VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(manga_id, source_chapter_id) DO UPDATE SET title = excluded.title, chapter_num = excluded.chapter_num, volume_num = excluded.volume_num`,
			mangaID, c.ID, c.Title, c.ChapterNum, c.VolumeNum,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func setChaptersUpToTx(tx *sql.Tx, mangaID int64, readSourceIDs []string) error {
	if len(readSourceIDs) == 0 {
		return nil
	}
	// Resolve the bound: highest chapter_num among the given source ids.
	place := make([]string, len(readSourceIDs))
	args := make([]any, 0, len(readSourceIDs)+1)
	args = append(args, mangaID)
	for i, id := range readSourceIDs {
		place[i] = "?"
		args = append(args, id)
	}
	query := "SELECT MAX(chapter_num) FROM chapters WHERE manga_id = ? AND source_chapter_id IN (" + strings.Join(place, ",") + ")"
	var bound sql.NullFloat64
	if err := tx.QueryRow(query, args...).Scan(&bound); err != nil {
		return err
	}
	if !bound.Valid {
		return nil
	}
	// Include sub-chapters like 4.1, 6.2 when bound is 6: treat 6.1 as part of 6
	_, err := tx.Exec(`UPDATE chapters SET is_read = 1 WHERE manga_id = ? AND chapter_num < ? + 1`, mangaID, bound.Float64)
	return err
}

func deleteChaptersNotInTx(tx *sql.Tx, mangaID int64, keepIDs []string) error {
	if len(keepIDs) == 0 {
		_, err := tx.Exec(`DELETE FROM chapters WHERE manga_id = ?`, mangaID)
		return err
	}
	place := make([]string, len(keepIDs))
	args := make([]any, 0, len(keepIDs)+1)
	args = append(args, mangaID)
	for i, id := range keepIDs {
		place[i] = "?"
		args = append(args, id)
	}
	_, err := tx.Exec(`DELETE FROM chapters WHERE manga_id = ? AND source_chapter_id NOT IN (`+strings.Join(place, ",")+`)`, args...)
	return err
}
