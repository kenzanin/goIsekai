package database

import (
	"time"

	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

// ReleasedAtString renders a chapter's release date in the storage format of
// the released_at column. It returns "" for a zero time, which means "the
// plugin knows no date".
func ReleasedAtString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

// chapterReleasedAt is the SQL expression for a chapter's release date: the
// site's own upload date as reported by the plugin, or NULL when the plugin
// knows none. NULL keeps "no date known" distinguishable from a real date.
func chapterReleasedAt(t time.Time) TimestampExpression {
	s := ReleasedAtString(t)
	if s == "" {
		return TimestampExp(NULL)
	}
	return TimestampExp(String(s))
}

// UpsertChapter inserts a chapter or, on a duplicate (manga_id, source_chapter_id),
// refreshes the identifying/metadata columns while preserving is_read, last_page_read
// and download_status. Returns the integer chapter ID.
//
// fetched_at is stamped with the sync time (CURRENT_TIMESTAMP); the site's own
// upload date goes to released_at, which is a different thing entirely.
func (d *DB) UpsertChapter(c Chapter) (int64, error) {
	// If ID is zero, let SQLite auto-generate it by inserting NULL.
	idVal := c.ID
	if idVal == 0 {
		idVal = 0 // still 0, but we'll handle it in VALUES
	}

	// Build INSERT with proper handling of zero ID.
	// If ID is zero, we insert NULL to trigger autoincrement.
	insertStmt := Chapters.INSERT(
		Chapters.MangaID,
		Chapters.SourceChapterID,
		Chapters.Title,
		Chapters.ChapterNum,
		Chapters.VolumeNum,
		Chapters.IsRead,
		Chapters.LastPageRead,
		Chapters.DownloadStatus,
		Chapters.FetchedAt,
		Chapters.ReleasedAt,
	).VALUES(
		c.MangaID,
		c.SourceChapterID,
		c.Title,
		c.ChapterNum,
		c.VolumeNum,
		boolToInt(c.IsRead),
		c.LastPageRead,
		c.DownloadStatus,
		RawTimestamp("CURRENT_TIMESTAMP"),
		chapterReleasedAt(c.ReleasedAt),
	).ON_CONFLICT(Chapters.MangaID, Chapters.SourceChapterID).DO_UPDATE(
		SET(
			Chapters.Title.SET(Chapters.EXCLUDED.Title),
			Chapters.ChapterNum.SET(Chapters.EXCLUDED.ChapterNum),
			Chapters.VolumeNum.SET(Chapters.EXCLUDED.VolumeNum),
			Chapters.FetchedAt.SET(RawTimestamp("CURRENT_TIMESTAMP")),
			Chapters.ReleasedAt.SET(chapterReleasedAt(c.ReleasedAt)),
		),
	)

	// Add ID column only if non-zero
	if idVal != 0 {
		insertStmt = Chapters.INSERT(
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
			Chapters.ReleasedAt,
		).VALUES(
			idVal,
			c.MangaID,
			c.SourceChapterID,
			c.Title,
			c.ChapterNum,
			c.VolumeNum,
			boolToInt(c.IsRead),
			c.LastPageRead,
			c.DownloadStatus,
			RawTimestamp("CURRENT_TIMESTAMP"),
			chapterReleasedAt(c.ReleasedAt),
		).ON_CONFLICT(Chapters.MangaID, Chapters.SourceChapterID).DO_UPDATE(
			SET(
				Chapters.Title.SET(Chapters.EXCLUDED.Title),
				Chapters.ChapterNum.SET(Chapters.EXCLUDED.ChapterNum),
				Chapters.VolumeNum.SET(Chapters.EXCLUDED.VolumeNum),
				Chapters.FetchedAt.SET(RawTimestamp("CURRENT_TIMESTAMP")),
				Chapters.ReleasedAt.SET(chapterReleasedAt(c.ReleasedAt)),
			),
		)
	}

	res, err := insertStmt.Exec(d.db)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return id, err
}

// SetChapterProgress records the last page read. It intentionally does NOT
// set is_read: a chapter is only "read" (struck through) when it's manually
// marked read or fully read (last_page_read >= total_pages), never merely
// opened. The derived Done flag is computed in GetChapterProgressForManga.
func (d *DB) SetChapterProgress(chapterID int64, lastPage int) error {
	_, err := Chapters.UPDATE().
		SET(Chapters.LastPageRead.SET(Int(int64(lastPage)))).
		WHERE(Chapters.ID.EQ(Int(chapterID))).
		Exec(d.db)
	if err != nil {
		return err
	}
	if lastPage >= 1 {
		err = d.RecordRead(chapterID, lastPage)
	}
	return err
}
