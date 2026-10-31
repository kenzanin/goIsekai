package database

import (
	"strings"
	"time"

	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

// LibraryOverview aggregates library-wide stats for the dashboard row.
type LibraryOverview struct {
	TotalTitles    int
	StatusDone     int
	StatusOngoing  int
	StatusUnknown  int
	FullyRead      int
	StartedReading int
	HasUpdates     int
	PagesRead      int
	MostTitle      string
	MostCount      int
	MostDup        int
	FewestTitle    string
	FewestCount    int
	FewestDup      int
}

// LibraryOverview returns high-level stats across all in-library manga.
// LibraryOverview returns high-level stats across all in-library manga.
//
// Stored statuses are already canonical (the write boundary normalizes them),
// so the buckets compare the label directly; anything else, including rows
// written before the vocabulary was enforced, counts as unknown.
func (d *DB) LibraryOverview() (LibraryOverview, error) {
	readCond := Chapters.IsRead.EQ(Int(1)).OR(
		Chapters.TotalPages.GT(Int(0)).AND(Chapters.LastPageRead.GT_EQ(Chapters.TotalPages)))

	type mangaRow struct {
		ID            string     `alias:"mangas.manga_id"`
		Title         string     `alias:"mangas.title"`
		Status        string     `alias:"mangas.status"`
		NewSince      *time.Time `alias:"mangas.new_since"`
		TotalChapters int        `alias:"stats.total_chapters"`
		ReadChapters  int        `alias:"stats.read_chapters"`
		TotalPages    int        `alias:"stats.total_pages"`
	}

	var rows []mangaRow
	err := SELECT(
		Mangas.ID.AS("mangas.manga_id"),
		Mangas.Title.AS("mangas.title"),
		Mangas.Status.AS("mangas.status"),
		Mangas.NewSince.AS("mangas.new_since"),
		COUNT(Chapters.ID).AS("stats.total_chapters"),
		COALESCE(SUM(CASE().WHEN(readCond).THEN(Int(1)).ELSE(Int(0))), Int(0)).AS("stats.read_chapters"),
		COALESCE(SUM(Chapters.LastPageRead), Int(0)).AS("stats.total_pages"),
	).FROM(Mangas.LEFT_JOIN(Chapters, Chapters.MangaID.EQ(Mangas.ID))).
		WHERE(Mangas.InLibrary.EQ(Int(1))).
		GROUP_BY(Mangas.ID).
		Query(d.db, &rows)
	if err != nil {
		return LibraryOverview{}, err
	}

	var ov LibraryOverview
	ov.TotalTitles = len(rows)

	// track most/fewest chapters with tie counts
	maxCount, minCount := -1, -1
	for _, r := range rows {
		switch {
		case strings.EqualFold(r.Status, "Completed"):
			ov.StatusDone++
		case strings.EqualFold(r.Status, "Ongoing"), strings.EqualFold(r.Status, "Hiatus"):
			ov.StatusOngoing++
		default:
			ov.StatusUnknown++
		}
		if r.NewSince != nil {
			ov.HasUpdates++
		}
		if r.TotalChapters > 0 && r.ReadChapters == r.TotalChapters {
			ov.FullyRead++
		} else if r.ReadChapters > 0 {
			ov.StartedReading++
		}
		ov.PagesRead += r.TotalPages

		if maxCount == -1 || r.TotalChapters > maxCount {
			maxCount, ov.MostCount, ov.MostTitle, ov.MostDup = r.TotalChapters, r.TotalChapters, r.Title, 1
		} else if r.TotalChapters == ov.MostCount {
			ov.MostDup++
		}
		if minCount == -1 || r.TotalChapters < minCount {
			minCount, ov.FewestCount, ov.FewestTitle, ov.FewestDup = r.TotalChapters, r.TotalChapters, r.Title, 1
		} else if r.TotalChapters == ov.FewestCount {
			ov.FewestDup++
		}
	}
	return ov, nil
}
