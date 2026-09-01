package database

import (
	. "github.com/go-jet/jet/v2/sqlite"
	. "goisekai/internal/database/.gen/table"
)

// UpsertChapter inserts a chapter or, on a duplicate id, refreshes the
// identifying/metadata columns while preserving is_read, last_page_read and
// download_status.
func (d *DB) UpsertChapter(c Chapter) error {
	_, err := Chapters.INSERT(
		Chapters.ID,
		Chapters.MangaID,
		Chapters.SourceChapterID,
		Chapters.Title,
		Chapters.ChapterNum,
		Chapters.VolumeNum,
		Chapters.IsRead,
		Chapters.LastPageRead,
		Chapters.DownloadStatus,
		Chapters.FetchedAt,
	).VALUES(
		c.ID,
		c.MangaID,
		c.SourceChapterID,
		c.Title,
		c.ChapterNum,
		c.VolumeNum,
		boolToInt(c.IsRead),
		c.LastPageRead,
		c.DownloadStatus,
		RawTimestamp("CURRENT_TIMESTAMP"),
	).ON_CONFLICT(Chapters.ID).DO_UPDATE(
		SET(
			Chapters.MangaID.SET(Chapters.EXCLUDED.MangaID),
			Chapters.SourceChapterID.SET(Chapters.EXCLUDED.SourceChapterID),
			Chapters.Title.SET(Chapters.EXCLUDED.Title),
			Chapters.ChapterNum.SET(Chapters.EXCLUDED.ChapterNum),
			Chapters.VolumeNum.SET(Chapters.EXCLUDED.VolumeNum),
			Chapters.FetchedAt.SET(RawTimestamp("CURRENT_TIMESTAMP")),
		),
	).Exec(d.db)
	return err
}

// SetChapterProgress records the last page read. It intentionally does NOT
// set is_read: a chapter is only "read" (struck through) when it's manually
// marked read or fully read (last_page_read >= total_pages), never merely
// opened. The derived Done flag is computed in GetChapterProgressForManga.
func (d *DB) SetChapterProgress(chapterID string, lastPage int) error {
	_, err := Chapters.UPDATE().
		SET(Chapters.LastPageRead.SET(Int(int64(lastPage)))).
		WHERE(Chapters.ID.EQ(String(chapterID))).
		Exec(d.db)
	return err
}

// SetChapterTotalPages records a chapter's page count (best-effort metadata
// from the plugin; ignored if it would lower an already-known count).
func (d *DB) SetChapterTotalPages(chapterID string, total int) error {
	_, err := Chapters.UPDATE().
		SET(Chapters.TotalPages.SET(Int(int64(total)))).
		WHERE(Chapters.ID.EQ(String(chapterID))).
		Exec(d.db)
	return err
}

// MarkChapterRead marks a single chapter as read without touching its page.
func (d *DB) MarkChapterRead(chapterRowID string) error {
	_, err := Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(1))).
		WHERE(Chapters.ID.EQ(String(chapterRowID))).
		Exec(d.db)
	return err
}

// MarkChapterReadRange marks every chapter of the manga whose chapter_num
// falls between the two referenced source chapter ids as read (inclusive and
// order-independent: from > to still marks the min..max span). Plain SQL: the
// bounds need two correlated MIN/MAX lookups that fight the jet row-mapper.
func (d *DB) MarkChapterReadRange(mangaRowID string, fromSourceID, toSourceID string) error {
	_, err := d.db.Exec(
		`UPDATE chapters SET is_read = 1
		 WHERE manga_id = ?
		   AND chapter_num BETWEEN
		       (SELECT MIN(chapter_num) FROM chapters WHERE manga_id = ? AND source_chapter_id IN (?, ?))
		   AND (SELECT MAX(chapter_num) FROM chapters WHERE manga_id = ? AND source_chapter_id IN (?, ?))`,
		mangaRowID, mangaRowID, fromSourceID, toSourceID, mangaRowID, fromSourceID, toSourceID)
	return err
}

// GetChapterProgressForManga returns per-chapter read progress for every
// stored chapter of a manga. Plain SQL: the projected columns don't map 1:1
// to a generated jet model, and a hand scan is clearer than fighting the
// row mapper for four columns.
func (d *DB) GetChapterProgressForManga(mangaRowID string) ([]ChapterProgress, error) {
	rows, err := d.db.Query(
		`SELECT source_chapter_id, last_page_read, total_pages, is_read
		 FROM chapters WHERE manga_id = ?`, mangaRowID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ChapterProgress
	for rows.Next() {
		var p ChapterProgress
		var isRead int
		if err := rows.Scan(&p.SourceChapterID, &p.LastPageRead, &p.TotalPages, &isRead); err != nil {
			return nil, err
		}
		p.IsRead = isRead != 0
		p.Done = p.IsRead || (p.TotalPages > 0 && p.LastPageRead >= p.TotalPages)
		out = append(out, p)
	}
	return out, rows.Err()
}

// ResetChapterProgress clears a chapter's read progress: last_page_read back
// to 0 and is_read off. total_pages is left intact (page-count metadata, not
// read state).
func (d *DB) ResetChapterProgress(chapterRowID string) error {
	_, err := Chapters.UPDATE().
		SET(
			Chapters.LastPageRead.SET(Int(0)),
			Chapters.IsRead.SET(Int(0)),
		).
		WHERE(Chapters.ID.EQ(String(chapterRowID))).
		Exec(d.db)
	return err
}

// ResetMangaProgress clears read progress for every chapter of a manga.
func (d *DB) ResetMangaProgress(mangaRowID string) error {
	_, err := Chapters.UPDATE().
		SET(
			Chapters.LastPageRead.SET(Int(0)),
			Chapters.IsRead.SET(Int(0)),
		).
		WHERE(Chapters.MangaID.EQ(String(mangaRowID))).
		Exec(d.db)
	return err
}

// SetDownloadStatus records the download status of a chapter.
func (d *DB) SetDownloadStatus(chapterID string, status string) error {
	_, err := Chapters.UPDATE().
		SET(Chapters.DownloadStatus.SET(String(status))).
		WHERE(Chapters.ID.EQ(String(chapterID))).
		Exec(d.db)
	return err
}
