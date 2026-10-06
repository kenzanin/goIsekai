package database

import (
	"fmt"

	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

// ToggleChapterSkip toggles the is_skipped flag for a chapter.
func (d *DB) ToggleChapterSkip(chapterRowID int64) error {
	_, err := d.db.Exec(`UPDATE chapters SET is_skipped = 1 - is_skipped WHERE id = ?`, chapterRowID)
	return err
}

// SetChaptersSkip sets the is_skipped flag for the given source chapters of a manga.
func (d *DB) SetChaptersSkip(mangaIntID int64, sourceIDs []string, skip bool) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	ids := make([]Expression, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		ids = append(ids, String(id))
	}
	_, err := Chapters.UPDATE().
		SET(Chapters.IsSkipped.SET(Int(readFlag(skip)))).
		WHERE(Chapters.MangaID.EQ(Int(mangaIntID)).AND(Chapters.SourceChapterID.IN(ids...))).
		Exec(d.db)
	return err
}

// readFlag converts a read/unread bool to the integer stored in chapters.is_read.
func readFlag(read bool) int64 {
	if read {
		return 1
	}
	return 0
}

// SetChaptersRead marks (read=true) or unmarks (read=false) the given source
// chapters of a manga, leaving per-chapter page progress untouched.
func (d *DB) SetChaptersRead(mangaIntID int64, sourceIDs []string, read bool) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	ids := make([]Expression, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		ids = append(ids, String(id))
	}
	_, err := Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(readFlag(read)))).
		WHERE(Chapters.MangaID.EQ(Int(mangaIntID)).AND(Chapters.SourceChapterID.IN(ids...))).
		Exec(d.db)
	return err
}

// SetChaptersUpTo marks (or unmarks) every chapter of a manga whose chapter_num
// is <= the highest chapter_num among the given source chapters.
func (d *DB) SetChaptersUpTo(mangaIntID int64, sourceIDs []string, read bool) error {
	return d.setChaptersToBound(mangaIntID, sourceIDs, read, false)
}

// SetChaptersDownTo marks (or unmarks) every chapter of a manga whose chapter_num
// is >= the lowest chapter_num among the given source chapters. It is the mirror
// of SetChaptersUpTo for catching up on a newer end of the list.
func (d *DB) SetChaptersDownTo(mangaIntID int64, sourceIDs []string, read bool) error {
	return d.setChaptersToBound(mangaIntID, sourceIDs, read, true)
}

// setChaptersToBound applies one read flag to a contiguous range of a manga,
// anchored on the ticked chapters. Upward takes the highest ticked chapter_num
// and covers everything at or before it; downward takes the lowest and covers
// everything at or after it.
func (d *DB) setChaptersToBound(mangaIntID int64, sourceIDs []string, read, down bool) error {
	dir := "up to"
	if down {
		dir = "down to"
	}
	if len(sourceIDs) == 0 {
		return fmt.Errorf("set chapters %s: no chapters given", dir)
	}
	ids := make([]Expression, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		ids = append(ids, String(id))
	}
	var nums []struct{ ChapterNum float64 }
	err := SELECT(Chapters.ChapterNum.AS("chapter_num")).
		FROM(Chapters).
		WHERE(Chapters.MangaID.EQ(Int(mangaIntID)).AND(Chapters.SourceChapterID.IN(ids...))).
		Query(d.db, &nums)
	if err != nil {
		return err
	}
	if len(nums) == 0 {
		return fmt.Errorf("set chapters %s: no matching chapters", dir)
	}
	bound := nums[0].ChapterNum
	for _, n := range nums[1:] {
		if (down && n.ChapterNum < bound) || (!down && n.ChapterNum > bound) {
			bound = n.ChapterNum
		}
	}
	range_ := Chapters.ChapterNum.LT_EQ(Float(bound))
	if down {
		range_ = Chapters.ChapterNum.GT_EQ(Float(bound))
	}
	_, err = Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(readFlag(read)))).
		WHERE(Chapters.MangaID.EQ(Int(mangaIntID)).AND(range_)).
		Exec(d.db)
	return err
}

// SetMangaChaptersRead marks (or unmarks) every chapter of a manga, leaving
// per-chapter page progress untouched.
func (d *DB) SetMangaChaptersRead(mangaIntID int64, read bool) error {
	_, err := Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(readFlag(read)))).
		WHERE(Chapters.MangaID.EQ(Int(mangaIntID))).
		Exec(d.db)
	return err
}

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

// ResetChapterProgress clears a chapter's read progress: last_page_read back
// to 0 and is_read off. total_pages is left intact (page-count metadata, not
// read state).
func (d *DB) ResetChapterProgress(chapterRowID int64) error {
	return d.resetProgress(Chapters.ID.EQ(Int(chapterRowID)))
}

func (d *DB) resetProgress(where BoolExpression) error {
	_, err := Chapters.UPDATE().
		SET(
			Chapters.LastPageRead.SET(Int(0)),
			Chapters.IsRead.SET(Int(0)),
		).
		WHERE(where).
		Exec(d.db)
	return err
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
