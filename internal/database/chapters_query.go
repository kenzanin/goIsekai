package database

import (
	"fmt"

	"goisekai/internal/database/.gen/model"
	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

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

// readFlag converts a read/unread bool to the integer stored in chapters.is_read.
func readFlag(read bool) int64 {
	if read {
		return 1
	}
	return 0
}

// SetChaptersRead marks (read=true) or unmarks (read=false) the given source
// chapters of a manga, leaving per-chapter page progress untouched.
func (d *DB) SetChaptersRead(mangaRowID string, sourceIDs []string, read bool) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	ids := make([]Expression, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		ids = append(ids, String(id))
	}
	_, err := Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(readFlag(read)))).
		WHERE(Chapters.MangaID.EQ(String(mangaRowID)).AND(Chapters.SourceChapterID.IN(ids...))).
		Exec(d.db)
	return err
}

// SetChaptersUpTo marks (or unmarks) every chapter of a manga whose chapter_num
// is <= the highest chapter_num among the given source chapters.
func (d *DB) SetChaptersUpTo(mangaRowID string, sourceIDs []string, read bool) error {
	if len(sourceIDs) == 0 {
		return fmt.Errorf("set chapters up to: no chapters given")
	}
	ids := make([]Expression, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		ids = append(ids, String(id))
	}
	var nums []struct{ ChapterNum float64 }
	err := SELECT(Chapters.ChapterNum.AS("chapter_num")).
		FROM(Chapters).
		WHERE(Chapters.MangaID.EQ(String(mangaRowID)).AND(Chapters.SourceChapterID.IN(ids...))).
		Query(d.db, &nums)
	if err != nil {
		return err
	}
	if len(nums) == 0 {
		return fmt.Errorf("set chapters up to: no matching chapters")
	}
	bound := nums[0].ChapterNum
	for _, n := range nums[1:] {
		if n.ChapterNum > bound {
			bound = n.ChapterNum
		}
	}
	_, err = Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(readFlag(read)))).
		WHERE(Chapters.MangaID.EQ(String(mangaRowID)).AND(Chapters.ChapterNum.LT_EQ(Float(bound)))).
		Exec(d.db)
	return err
}

// SetMangaChaptersRead marks (or unmarks) every chapter of a manga, leaving
// per-chapter page progress untouched.
func (d *DB) SetMangaChaptersRead(mangaRowID string, read bool) error {
	_, err := Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(readFlag(read)))).
		WHERE(Chapters.MangaID.EQ(String(mangaRowID))).
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
	return d.resetProgress(Chapters.ID.EQ(String(chapterRowID)))
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

// ListChaptersCached returns all chapters for a manga from the database cache,
// ordered newest-first (descending chapter_num, then by ID for ties). Returns
// an empty slice (not nil) when no chapters are cached.
func (d *DB) ListChaptersCached(mangaRowID string) ([]Chapter, error) {
	var models []model.Chapters
	err := Chapters.SELECT(Chapters.AllColumns).
		WHERE(Chapters.MangaID.EQ(String(mangaRowID))).
		ORDER_BY(Chapters.ChapterNum.DESC(), Chapters.ID.DESC()).
		Query(d.db, &models)
	if err != nil {
		return nil, err
	}
	out := make([]Chapter, len(models))
	for i, m := range models {
		out[i] = Chapter{
			ID:              derefStr(m.ID),
			MangaID:         m.MangaID,
			SourceChapterID: m.SourceChapterID,
			Title:           m.Title,
			ChapterNum:      m.ChapterNum,
			VolumeNum:       derefFloat(m.VolumeNum),
			IsRead:          derefBool(m.IsRead),
			LastPageRead:    int(derefFloatPtr(m.LastPageRead)),
			TotalPages:      int(derefFloatPtr(m.TotalPages)),
			DownloadStatus:  derefStr(m.DownloadStatus),
			FetchedAt:       derefTime(m.FetchedAt),
		}
	}
	return out, nil
}

// derefFloatPtr converts *int64 to float64 for int64 fields that we treat as floats.
func derefFloatPtr(p *int64) float64 {
	if p != nil {
		return float64(*p)
	}
	return 0
}
