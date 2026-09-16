package database

import (
	"goisekai/internal/database/.gen/model"
	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

// SetChapterTotalPages records a chapter's page count (best-effort metadata
// from the plugin; ignored if it would lower an already-known count).
func (d *DB) SetChapterTotalPages(chapterID int64, total int) error {
	_, err := Chapters.UPDATE().
		SET(Chapters.TotalPages.SET(Int(int64(total)))).
		WHERE(Chapters.ID.EQ(Int(chapterID))).
		Exec(d.db)
	return err
}

// GetChapterTotalPages returns a chapter's recorded page count (0 when absent).
func (d *DB) GetChapterTotalPages(chapterID int64) (int, error) {
	var out []struct{ TotalPages int64 }
	err := SELECT(Chapters.TotalPages.AS("total_pages")).
		FROM(Chapters).
		WHERE(Chapters.ID.EQ(Int(chapterID))).
		Query(d.db, &out)
	if err != nil || len(out) == 0 {
		return 0, err
	}
	return int(out[0].TotalPages), nil
}

// MarkChapterRead marks a single chapter as read without touching its page.
func (d *DB) MarkChapterRead(chapterRowID int64) error {
	_, err := Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(1))).
		WHERE(Chapters.ID.EQ(Int(chapterRowID))).
		Exec(d.db)
	return err
}

// CountChaptersForManga returns how many chapter rows exist for a manga.
func (d *DB) CountChaptersForManga(mangaIntID int64) (int, error) {
	var out []struct{ N int64 }
	err := SELECT(COUNT(Chapters.ID).AS("n")).
		FROM(Chapters).
		WHERE(Chapters.MangaID.EQ(Int(mangaIntID))).
		Query(d.db, &out)
	if err != nil || len(out) == 0 {
		return 0, err
	}
	return int(out[0].N), nil
}

// ListChaptersCached returns all chapters for a manga from the database cache,
// ordered newest-first (descending chapter_num, then by ID for ties). Returns
// an empty slice (not nil) when no chapters are cached.
func (d *DB) ListChaptersCached(mangaIntID int64) ([]Chapter, error) {
	var models []model.Chapters
	err := Chapters.SELECT(Chapters.AllColumns).
		WHERE(Chapters.MangaID.EQ(Int(mangaIntID))).
		ORDER_BY(Chapters.ChapterNum.DESC(), Chapters.ID.DESC()).
		Query(d.db, &models)
	if err != nil {
		return nil, err
	}
	out := make([]Chapter, len(models))
	for i, m := range models {
		out[i] = chapterFromModel(m)
	}
	return out, nil
}
