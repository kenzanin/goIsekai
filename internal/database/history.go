package database

import (
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
