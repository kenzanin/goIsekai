package database

import (
	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

// GetChapterProgressForManga returns per-chapter read progress for every
// stored chapter of a manga.
func (d *DB) GetChapterProgressForManga(mangaIntID int64) ([]ChapterProgress, error) {
	var rows []struct {
		SourceChapterID string
		LastPageRead    int64
		TotalPages      int64
		IsRead          int64
		IsSkipped       int64
	}
	err := SELECT(Chapters.SourceChapterID.AS("source_chapter_id"), Chapters.LastPageRead.AS("last_page_read"), Chapters.TotalPages.AS("total_pages"), Chapters.IsRead.AS("is_read"), Chapters.IsSkipped.AS("is_skipped")).
		FROM(Chapters).
		WHERE(Chapters.MangaID.EQ(Int(mangaIntID))).
		Query(d.db, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]ChapterProgress, 0, len(rows))
	for _, r := range rows {
		p := ChapterProgress{
			SourceChapterID: r.SourceChapterID,
			LastPageRead:    int(r.LastPageRead),
			TotalPages:      int(r.TotalPages),
			IsRead:          r.IsRead != 0,
			IsSkipped:       r.IsSkipped != 0,
		}
		p.Done = p.IsRead || (p.TotalPages > 0 && p.LastPageRead >= p.TotalPages)
		out = append(out, p)
	}
	return out, nil
}

// ReadSourceIDs returns the source_chapter_ids of read chapters for a manga.
func (d *DB) ReadSourceIDs(mangaID int64) ([]string, error) {
	rows, err := d.db.Query(`SELECT source_chapter_id FROM chapters WHERE manga_id = ? AND (is_read = 1 OR last_page_read > 0)`, mangaID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
