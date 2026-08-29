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

// SetChapterProgress records the last page read and marks the chapter read.
func (d *DB) SetChapterProgress(chapterID string, lastPage int) error {
	_, err := Chapters.UPDATE().
		SET(
			Chapters.LastPageRead.SET(Int(int64(lastPage))),
			Chapters.IsRead.SET(Int(1)),
		).
		WHERE(Chapters.ID.EQ(String(chapterID))).
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
