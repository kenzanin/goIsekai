package database

import (
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
// chapters of a manga. Marking read leaves page progress alone; unmarking also
// drops the page mark of any fully-read row, see clearFullyReadPages.
func (d *DB) SetChaptersRead(mangaIntID int64, sourceIDs []string, read bool) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	ids := make([]Expression, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		ids = append(ids, String(id))
	}
	where := Chapters.MangaID.EQ(Int(mangaIntID)).AND(Chapters.SourceChapterID.IN(ids...))
	if !read {
		if err := d.clearFullyReadPages(where); err != nil {
			return err
		}
	}
	_, err := Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(readFlag(read)))).
		WHERE(where).
		Exec(d.db)
	return err
}

// clearFullyReadPages drops the page mark on the rows in where that are done by
// page count rather than by is_read. The detail page renders the derived
// ChapterProgress.Done (is_read OR fully read), so un-reading a chapter read to
// the end has to clear last_page_read as well — otherwise the row keeps its
// strikethrough and the bulk "unread" actions look like they did nothing. A
// partially-read chapter keeps its bookmark, since only is_read can make it
// done. Mirrors the same rule as ToggleChapterRead.
func (d *DB) clearFullyReadPages(where BoolExpression) error {
	_, err := Chapters.UPDATE().
		SET(Chapters.LastPageRead.SET(Int(0))).
		WHERE(where.AND(fullyReadPages())).
		Exec(d.db)
	return err
}

// fullyReadPages matches chapters whose "done" state comes from the page count:
// a page total is known and the reader reached it.
func fullyReadPages() BoolExpression {
	return Chapters.TotalPages.GT(Int(0)).AND(Chapters.LastPageRead.GT_EQ(Chapters.TotalPages))
}

// SetMangaChaptersRead marks (or unmarks) every chapter of a manga, leaving
// per-chapter page progress untouched when marking read. Unmarking also drops
// the page mark of fully-read rows, see clearFullyReadPages.
func (d *DB) SetMangaChaptersRead(mangaIntID int64, read bool) error {
	where := Chapters.MangaID.EQ(Int(mangaIntID))
	if !read {
		if err := d.clearFullyReadPages(where); err != nil {
			return err
		}
	}
	_, err := Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(readFlag(read)))).
		WHERE(where).
		Exec(d.db)
	return err
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
