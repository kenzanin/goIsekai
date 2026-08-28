package database

// RecordRead appends a read-history entry for a chapter's current page.
func (d *DB) RecordRead(chapterID string, pageNum int) error {
	_, err := d.db.Exec(
		`INSERT INTO read_history (chapter_id, page_num) VALUES (?, ?)`,
		chapterID, pageNum,
	)
	return err
}
