package bridge

import (
	"database/sql"
	"fmt"
	"strings"

	"goisekai/internal/database"
	"goisekai/pkg/types"
)

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
