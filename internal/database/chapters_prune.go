package database

import (
	"math"
	"strings"

	"goisekai/internal/database/.gen/model"
	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

// idSeparators are the characters plugins use to join a manga slug with a
// chapter id ("slug:chapter-42", "slug/c001", "uuid:42"). Everything after the
// last separator is the plugin's own chapter id.
const idSeparators = ":#?=&%!@~,;|"

// minTailLen keeps short ids out of the supersede check: a tail of "1" would
// match a chapter row numbered "1" of an unrelated chapter.
const minTailLen = 3

// PruneSupersededChapters removes chapter rows of a manga that the plugin has
// re-issued under a new source id: the plugin now answers "slug:chapter-42"
// while the row still holds the old "chapter-42". Only SourceChapterID and
// ChapterNum of the returned chapters are read.
//
// A row is removed only when the proof is complete: its replacement was in the
// same answer, carries the same chapter number, and the row itself holds no
// read progress, skip or download. An empty answer prunes nothing - a plugin
// that failed to list its chapters proves nothing about rows it never sent.
//
// ponytail: the rule assumes a plugin re-issues ids by prefixing the old one.
// A plugin that renames ids arbitrarily leaves its old rows behind; widen the
// rule when a second such plugin shows up.
func (d *DB) PruneSupersededChapters(mangaID int64, returned []Chapter) (int64, error) {
	if len(returned) == 0 {
		return 0, nil
	}

	// Tail of every returned id, mapped to the chapter number it belongs to.
	// A tail seen twice with different numbers is ambiguous and dropped.
	type tailRef struct {
		num       float64
		ambiguous bool
	}
	tails := make(map[string]tailRef, len(returned))
	live := make(map[string]struct{}, len(returned))
	for _, c := range returned {
		live[c.SourceChapterID] = struct{}{}
		tail, ok := idTail(c.SourceChapterID)
		if !ok {
			continue
		}
		ref, seen := tails[tail]
		switch {
		case !seen:
			tails[tail] = tailRef{num: c.ChapterNum}
		case !ref.ambiguous && !sameChapterNum(ref.num, c.ChapterNum):
			tails[tail] = tailRef{ambiguous: true}
		}
	}

	var rows []model.Chapters
	err := Chapters.SELECT(Chapters.AllColumns).
		WHERE(Chapters.MangaID.EQ(Int(mangaID))).
		Query(d.db, &rows)
	if err != nil {
		return 0, err
	}

	victims := make([]Expression, 0, len(rows))
	for _, m := range rows {
		src := m.SourceChapterID
		if _, ok := live[src]; ok {
			continue // the plugin still sends this id
		}
		ref, ok := tails[src]
		if !ok || ref.ambiguous {
			continue // no proof of a replacement
		}
		if !sameChapterNum(m.ChapterNum, ref.num) {
			continue
		}
		if chapterHasProgress(m) {
			continue
		}
		victims = append(victims, Int(derefInt64(m.ID)))
	}
	if len(victims) == 0 {
		return 0, nil
	}
	res, err := Chapters.DELETE().
		WHERE(Chapters.ID.IN(victims...)).
		Exec(d.db)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// idTail returns the part of a plugin chapter id after its last separator.
func idTail(id string) (string, bool) {
	i := strings.LastIndexAny(id, idSeparators)
	if i <= 0 {
		return "", false
	}
	tail := id[i+1:]
	if len(tail) < minTailLen {
		return "", false
	}
	return tail, true
}

// sameChapterNum compares chapter numbers with float tolerance.
func sameChapterNum(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// chapterHasProgress reports whether a chapter row holds state a user would
// lose if the row disappeared: read progress, a skip mark or a download.
func chapterHasProgress(m model.Chapters) bool {
	if derefInt64(m.IsRead) > 0 || derefInt64(m.IsSkipped) > 0 {
		return true
	}
	if derefInt64(m.LastPageRead) > 0 || derefInt64(m.TotalPages) > 0 {
		return true
	}
	status := derefStr(m.DownloadStatus)
	return status != "" && status != DownloadNotDownloaded
}
