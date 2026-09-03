package database

import (
	"math"

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
	if err != nil {
		return err
	}
	if lastPage >= 1 {
		_, err = ReadHistory.INSERT(
			ReadHistory.ChapterID,
			ReadHistory.PageNum,
		).VALUES(
			chapterID,
			int64(lastPage),
		).Exec(d.db)
	}
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

// GetChapterTotalPages returns a chapter's recorded page count (0 when absent).
func (d *DB) GetChapterTotalPages(chapterID string) (int, error) {
	var out []struct{ TotalPages int64 }
	err := SELECT(Chapters.TotalPages.AS("total_pages")).
		FROM(Chapters).
		WHERE(Chapters.ID.EQ(String(chapterID))).
		Query(d.db, &out)
	if err != nil || len(out) == 0 {
		return 0, err
	}
	return int(out[0].TotalPages), nil
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
// order-independent: from > to still marks the min..max span). The two bound
// chapter numbers are fetched first, then a single jet UPDATE applies the span.
func (d *DB) MarkChapterReadRange(mangaRowID string, fromSourceID, toSourceID string) error {
	var bounds []struct {
		ChapterNum float64
	}
	err := SELECT(Chapters.ChapterNum.AS("chapter_num")).
		FROM(Chapters).
		WHERE(Chapters.MangaID.EQ(String(mangaRowID)).AND(
			Chapters.SourceChapterID.IN(String(fromSourceID), String(toSourceID)))).
		Query(d.db, &bounds)
	if err != nil {
		return err
	}
	if len(bounds) == 0 {
		return nil // neither bound exists; nothing to mark
	}
	lo, hi := bounds[0].ChapterNum, bounds[0].ChapterNum
	if len(bounds) > 1 {
		lo = math.Min(bounds[0].ChapterNum, bounds[1].ChapterNum)
		hi = math.Max(bounds[0].ChapterNum, bounds[1].ChapterNum)
	}
	_, err = Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(1))).
		WHERE(Chapters.MangaID.EQ(String(mangaRowID)).AND(
			Chapters.ChapterNum.BETWEEN(Float(lo), Float(hi)))).
		Exec(d.db)
	return err
}

// GetChapterProgressForManga returns per-chapter read progress for every
// stored chapter of a manga.
func (d *DB) GetChapterProgressForManga(mangaRowID string) ([]ChapterProgress, error) {
	var rows []struct {
		SourceChapterID string
		LastPageRead    int64
		TotalPages      int64
		IsRead          int64
	}
	err := SELECT(Chapters.SourceChapterID.AS("source_chapter_id"), Chapters.LastPageRead.AS("last_page_read"), Chapters.TotalPages.AS("total_pages"), Chapters.IsRead.AS("is_read")).
		FROM(Chapters).
		WHERE(Chapters.MangaID.EQ(String(mangaRowID))).
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
		}
		p.Done = p.IsRead || (p.TotalPages > 0 && p.LastPageRead >= p.TotalPages)
		out = append(out, p)
	}
	return out, nil
}

// CountChaptersForManga returns how many chapter rows exist for a manga.
func (d *DB) CountChaptersForManga(mangaRowID string) (int, error) {
	var out []struct{ N int64 }
	err := SELECT(COUNT(Chapters.ID).AS("n")).
		FROM(Chapters).
		WHERE(Chapters.MangaID.EQ(String(mangaRowID))).
		Query(d.db, &out)
	if err != nil || len(out) == 0 {
		return 0, err
	}
	return int(out[0].N), nil
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
