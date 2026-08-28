package database

// UpsertChapter inserts a chapter or, on a duplicate id (primary key), updates
// only the identifying/metadata fields. is_read, last_page_read, and
// download_status are left untouched so refreshing from the source never
// resets a reader's progress.
func (d *DB) UpsertChapter(c Chapter) error {
	_, err := d.db.Exec(`
		INSERT INTO chapters
			(id, manga_id, source_chapter_id, title, chapter_num, volume_num, is_read, last_page_read, download_status, fetched_at)
	VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			manga_id = excluded.manga_id,
			source_chapter_id = excluded.source_chapter_id,
			title = excluded.title,
			chapter_num = excluded.chapter_num,
			volume_num = excluded.volume_num,
			fetched_at = CURRENT_TIMESTAMP`,
		c.ID, c.MangaID, c.SourceChapterID, c.Title, c.ChapterNum, c.VolumeNum,
		intFromBool(c.IsRead), c.LastPageRead, c.DownloadStatus,
	)
	return err
}

// SetChapterProgress records the reader's last page and marks the chapter read.
func (d *DB) SetChapterProgress(chapterID string, lastPage int) error {
	_, err := d.db.Exec(
		`UPDATE chapters SET last_page_read = ?, is_read = 1 WHERE id = ?`,
		lastPage, chapterID,
	)
	return err
}

// SetDownloadStatus records a chapter's download state.
func (d *DB) SetDownloadStatus(chapterID string, status string) error {
	_, err := d.db.Exec(
		`UPDATE chapters SET download_status = ? WHERE id = ?`,
		status, chapterID,
	)
	return err
}
