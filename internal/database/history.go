package database

import (
	"time"

	. "github.com/go-jet/jet/v2/sqlite"
	. "goisekai/internal/database/.gen/table"
)

// RecordRead inserts a read-history row for a chapter.
func (d *DB) RecordRead(chapterID string, pageNum int) error {
	_, err := ReadHistory.INSERT(
		ReadHistory.ChapterID,
		ReadHistory.PageNum,
	).VALUES(
		chapterID,
		int64(pageNum),
	).Exec(d.db)
	return err
}

// HistoryEntry is one row of the reading history page.
//
// alias tags follow qrm's two-part named-struct mapping (see LibraryMangaStats).
type HistoryEntry struct {
	MangaID       string    `alias:"mangas.manga_id"`
	SourceMangaID string    `alias:"mangas.source_manga_id"`
	PluginID      string    `alias:"mangas.plugin_id"`
	Title         string    `alias:"mangas.title"`
	CoverURL      string    `alias:"mangas.cover_url"`
	TotalChapters int       `alias:"stats.total_chapters"`
	ReadChapters  int       `alias:"stats.read_chapters"`
	LastReadAt    time.Time `alias:"history.last_read_at"`
	PluginName    string    // filled by bridge, not DB
}

// GetReadHistory returns manga that have been read, ordered by most recently read.
func (d *DB) GetReadHistory() ([]HistoryEntry, error) {
	readCond := Chapters.IsRead.EQ(Int(1)).OR(
		Chapters.TotalPages.GT(Int(0)).AND(Chapters.LastPageRead.GT_EQ(Chapters.TotalPages)))
	lastRead := MAX(ReadHistory.ReadAt)
	lastID := MAX(ReadHistory.ID)
	var out []HistoryEntry
	err := SELECT(
		Mangas.ID.AS("mangas.manga_id"),
		Mangas.SourceMangaID.AS("mangas.source_manga_id"),
		Mangas.PluginID.AS("mangas.plugin_id"),
		Mangas.Title.AS("mangas.title"),
		Mangas.CoverURL.AS("mangas.cover_url"),
		COUNT(DISTINCT(Chapters.ID)).AS("stats.total_chapters"),
		COALESCE(SUM(CASE().WHEN(readCond).THEN(Int(1)).ELSE(Int(0))), Int(0)).AS("stats.read_chapters"),
		lastRead.AS("history.last_read_at"),
	).FROM(ReadHistory.
		INNER_JOIN(Chapters, Chapters.ID.EQ(ReadHistory.ChapterID)).
		INNER_JOIN(Mangas, Mangas.ID.EQ(Chapters.MangaID))).
		GROUP_BY(Mangas.ID).
		ORDER_BY(lastRead.DESC(), lastID.DESC()).
		Query(d.db, &out)
	return out, err
}

// LastReadChapter returns the most recently read chapter for a manga, or nil.
func (d *DB) LastReadChapter(mangaRowID string) (sourceChapterID string, pageNum int, ok bool) {
	var rows []struct {
		SourceChapterID string
		PageNum         int
	}
	err := SELECT(Chapters.SourceChapterID.AS("source_chapter_id"), ReadHistory.PageNum.AS("page_num")).
		FROM(ReadHistory.
			INNER_JOIN(Chapters, Chapters.ID.EQ(ReadHistory.ChapterID))).
		WHERE(Chapters.MangaID.EQ(String(mangaRowID))).
		ORDER_BY(ReadHistory.ReadAt.DESC(), ReadHistory.ID.DESC()).
		LIMIT(1).
		Query(d.db, &rows)
	if err != nil || len(rows) == 0 {
		return "", 0, false
	}
	return rows[0].SourceChapterID, rows[0].PageNum, true
}
