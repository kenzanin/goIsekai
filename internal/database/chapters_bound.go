package database

import (
	"fmt"

	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

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
// anchored on the ticked chapters. Upward takes the highest ticked chapter and
// covers everything at or before it; downward takes the lowest and covers
// everything at or after it.
//
// The order is the pair (chapter_num, id), not chapter_num alone. Some sources
// emit chapter_num = 0 for every chapter (comizy does), and a bound of 0 makes
// `chapter_num <= 0` (and `>= 0`) match the whole manga — so "set up to as
// read" on chapter 5 marked all seven chapters. id is insertion order, which is
// the source's own chapter order, so it is the right tie-break.
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
	var rows []struct {
		ID         int64
		ChapterNum float64
	}
	err := SELECT(Chapters.ID.AS("id"), Chapters.ChapterNum.AS("chapter_num")).
		FROM(Chapters).
		WHERE(Chapters.MangaID.EQ(Int(mangaIntID)).AND(Chapters.SourceChapterID.IN(ids...))).
		Query(d.db, &rows)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("set chapters %s: no matching chapters", dir)
	}
	boundID, boundNum := rows[0].ID, rows[0].ChapterNum
	for _, r := range rows[1:] {
		lower := r.ChapterNum < boundNum || (r.ChapterNum == boundNum && r.ID < boundID)
		higher := r.ChapterNum > boundNum || (r.ChapterNum == boundNum && r.ID > boundID)
		if (down && lower) || (!down && higher) {
			boundID, boundNum = r.ID, r.ChapterNum
		}
	}
	// Lexicographic comparison: chapter_num alone is not a key.
	eqNum := Chapters.ChapterNum.EQ(Float(boundNum))
	range_ := Chapters.ChapterNum.LT(Float(boundNum)).
		OR(eqNum.AND(Chapters.ID.LT_EQ(Int(boundID))))
	if down {
		range_ = Chapters.ChapterNum.GT(Float(boundNum)).
			OR(eqNum.AND(Chapters.ID.GT_EQ(Int(boundID))))
	}
	where := Chapters.MangaID.EQ(Int(mangaIntID)).AND(range_)
	if !read {
		if err := d.clearFullyReadPages(where); err != nil {
			return err
		}
	}
	_, err = Chapters.UPDATE().
		SET(Chapters.IsRead.SET(Int(readFlag(read)))).
		WHERE(where).
		Exec(d.db)
	return err
}
