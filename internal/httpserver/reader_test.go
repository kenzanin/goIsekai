package httpserver

import (
	"net/http/httptest"
	"strings"
	"testing"

	"goisekai/internal/database"
)

// seedChapters inserts chapters for a manga, oldest first.
func seedChapters(t *testing.T, db *database.DB, mangaIntID int64, chapters ...database.Chapter) {
	t.Helper()
	for _, c := range chapters {
		c.MangaID = mangaIntID
		if _, err := db.UpsertChapter(c); err != nil {
			t.Fatalf("upsert chapter %s: %v", c.SourceChapterID, err)
		}
	}
}

// The reader takes chapter navigation from the persisted list, so a plugin that
// returns a partial chapter list cannot strand the reader at the end of a
// chapter that has a next one. The test plugin manager points at an empty dir,
// which is exactly that case.
func TestReaderNeighborsUsePersistedChapters(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	mangaIntID := seedManga(t, db, "p1", "m1", "Manga")
	seedChapters(t, db, mangaIntID,
		database.Chapter{SourceChapterID: "c1", ChapterNum: 1},
		database.Chapter{SourceChapterID: "c2", ChapterNum: 2},
		database.Chapter{SourceChapterID: "c3", ChapterNum: 3},
	)

	req := httptest.NewRequest("GET", "/view/read/p1/m1/c2", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`data-next-chapter-id="c3"`, `data-prev-chapter-id="c1"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("reader body missing %s in: %s", want, body)
		}
	}
}

// A chapter at either end of the list offers no neighbour in that direction.
func TestReaderNeighborsAtEnds(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	mangaIntID := seedManga(t, db, "p1", "m1", "Manga")
	seedChapters(t, db, mangaIntID,
		database.Chapter{SourceChapterID: "c1", ChapterNum: 1},
		database.Chapter{SourceChapterID: "c2", ChapterNum: 2},
	)

	req := httptest.NewRequest("GET", "/view/read/p1/m1/c2", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `data-next-chapter-id=""`) {
		t.Fatalf("newest chapter should have no next chapter; got: %s", body)
	}
}
