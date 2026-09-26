package database

import (
	"database/sql"
	"fmt"
	"testing"
)

func ftsRow(t *testing.T, db *DB, mangaID int64) (title, alt string, ok bool) {
	t.Helper()
	err := db.db.QueryRow(
		`SELECT title, alt FROM library_fts WHERE CAST(manga_row_id AS TEXT) = CAST(? AS TEXT)`, mangaID,
	).Scan(&title, &alt)
	if err == sql.ErrNoRows {
		return "", "", false
	}
	if err != nil {
		t.Fatalf("fts row %d: %v", mangaID, err)
	}
	return title, alt, true
}

func ftsCount(t *testing.T, db *DB) int {
	t.Helper()
	var n int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM library_fts`).Scan(&n); err != nil {
		t.Fatalf("fts count: %v", err)
	}
	return n
}

func assertFTSConsistent(t *testing.T, db *DB) {
	t.Helper()
	drift, err := db.LibraryFTSDrift()
	if err != nil {
		t.Fatalf("drift: %v", err)
	}
	if drift {
		t.Fatal("library_fts drifts from library membership/content")
	}
}

// TestEnsureLibraryFTSRebuildPopulatesTitleAlt: a wiped index is rebuilt with
// both title and alt for every in-library manga, and nothing for the rest.
func TestEnsureLibraryFTSRebuildPopulatesTitleAlt(t *testing.T) {
	db := openTestDB(t)
	id1, err := db.UpsertManga(Manga{PluginID: "p1", SourceMangaID: "s1", Title: "Solo Leveling", InLibrary: true})
	if err != nil {
		t.Fatal(err)
	}
	id2, err := db.UpsertManga(Manga{PluginID: "p1", SourceMangaID: "s2", Title: "Tower of God", InLibrary: true})
	if err != nil {
		t.Fatal(err)
	}
	id3, err := db.UpsertManga(Manga{PluginID: "p2", SourceMangaID: "s3", Title: "Berserk", InLibrary: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertManga(Manga{PluginID: "p2", SourceMangaID: "s4", Title: "Not In Library"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddAltTitles(fmt.Sprint(id1), []string{"Na Honjaman Level Up"}, "mal"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddAltTitles(fmt.Sprint(id3), []string{"Berserk Prototype"}, "anilist"); err != nil {
		t.Fatal(err)
	}

	// Simulate a badly stale index (the pre-fix live DB state).
	if _, err := db.db.Exec(`DELETE FROM library_fts`); err != nil {
		t.Fatal(err)
	}

	rebuilt, err := db.EnsureLibraryFTS()
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if !rebuilt {
		t.Fatal("expected rebuild on empty index")
	}
	if n := ftsCount(t, db); n != 3 {
		t.Fatalf("fts rows = %d, want 3", n)
	}

	want := []struct {
		id    int64
		title string
		alt   string
	}{
		{id1, "Solo Leveling", "Na Honjaman Level Up"},
		{id2, "Tower of God", ""},
		{id3, "Berserk", "Berserk Prototype"},
	}
	for _, w := range want {
		title, alt, ok := ftsRow(t, db, w.id)
		if !ok {
			t.Fatalf("manga %d missing from index", w.id)
		}
		if title != w.title || alt != w.alt {
			t.Fatalf("manga %d indexed (%q, %q), want (%q, %q)", w.id, title, alt, w.title, w.alt)
		}
	}

	assertFTSConsistent(t, db)
	rebuilt, err = db.EnsureLibraryFTS()
	if err != nil {
		t.Fatalf("ensure 2: %v", err)
	}
	if rebuilt {
		t.Fatal("second ensure rebuilt a consistent index")
	}
}

// TestEnsureLibraryFTSDriftTriggersRebuild: every drift shape (missing row,
// unsynced insert, duplicate rows, dangling rows, stale content) triggers one
// rebuild and then the check goes quiet.
func TestEnsureLibraryFTSDriftTriggersRebuild(t *testing.T) {
	cases := []struct {
		name    string
		corrupt func(t *testing.T, db *DB, id int64)
	}{
		{"missing rows", func(t *testing.T, db *DB, _ int64) {
			if _, err := db.db.Exec(`DELETE FROM library_fts`); err != nil {
				t.Fatal(err)
			}
		}},
		{"unsynced in-library insert", func(t *testing.T, db *DB, _ int64) {
			if _, err := db.db.Exec(`INSERT INTO mangas (plugin_id, source_manga_id, title, in_library) VALUES ('px', 'sx', 'Ghost', 1)`); err != nil {
				t.Fatal(err)
			}
		}},
		{"duplicate rows", func(t *testing.T, db *DB, id int64) {
			if _, err := db.db.Exec(`INSERT INTO library_fts (manga_row_id, plugin_id, title, alt) SELECT id, plugin_id, title, '' FROM mangas WHERE id = ?`, id); err != nil {
				t.Fatal(err)
			}
		}},
		{"dangling row", func(t *testing.T, db *DB, _ int64) {
			if _, err := db.db.Exec(`INSERT INTO library_fts (manga_row_id, plugin_id, title, alt) VALUES ('9999', 'p1', 'Ghost', '')`); err != nil {
				t.Fatal(err)
			}
		}},
		{"stale content", func(t *testing.T, db *DB, _ int64) {
			if _, err := db.db.Exec(`UPDATE library_fts SET title = 'Stale'`); err != nil {
				t.Fatal(err)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			id, err := db.UpsertManga(Manga{PluginID: "p1", SourceMangaID: "s1", Title: "Old Main", InLibrary: true})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.AddAltTitles(fmt.Sprint(id), []string{"Alias One"}, "mal"); err != nil {
				t.Fatal(err)
			}
			assertFTSConsistent(t, db)

			tc.corrupt(t, db, id)

			drift, err := db.LibraryFTSDrift()
			if err != nil {
				t.Fatalf("drift: %v", err)
			}
			if !drift {
				t.Fatal("drift check missed corruption")
			}
			rebuilt, err := db.EnsureLibraryFTS()
			if err != nil {
				t.Fatalf("ensure: %v", err)
			}
			if !rebuilt {
				t.Fatal("expected rebuild after drift")
			}
			assertFTSConsistent(t, db)
			if rebuilt, err = db.EnsureLibraryFTS(); err != nil || rebuilt {
				t.Fatalf("second ensure: rebuilt=%v err=%v", rebuilt, err)
			}
		})
	}
}

// TestLibraryFTSMutationPathsKeepIndexConsistent: every mutation path that can
// change library membership or titles must leave the index consistent and
// up to date.
func TestLibraryFTSMutationPathsKeepIndexConsistent(t *testing.T) {
	cases := []struct {
		name        string
		apply       func(t *testing.T, db *DB, id int64)
		wantIndexed bool
		wantTitle   string
		wantAlt     string
	}{
		{
			name: "upsert title refresh",
			apply: func(t *testing.T, db *DB, id int64) {
				if _, err := db.UpsertManga(Manga{PluginID: "p1", SourceMangaID: "s1", Title: "New Main"}); err != nil {
					t.Fatal(err)
				}
			},
			wantIndexed: true, wantTitle: "New Main", wantAlt: "Alias One",
		},
		{
			name: "upsert insert in library",
			apply: func(t *testing.T, db *DB, _ int64) {
				if _, err := db.UpsertManga(Manga{PluginID: "p1", SourceMangaID: "s9", Title: "Fresh Add", InLibrary: true}); err != nil {
					t.Fatal(err)
				}
			},
			wantIndexed: true, wantTitle: "Old Main", wantAlt: "Alias One",
		},
		{
			name: "toggle off",
			apply: func(t *testing.T, db *DB, id int64) {
				if err := db.ToggleLibrary(id); err != nil {
					t.Fatal(err)
				}
			},
			wantIndexed: false,
		},
		{
			name: "toggle off and back on",
			apply: func(t *testing.T, db *DB, id int64) {
				if err := db.ToggleLibrary(id); err != nil {
					t.Fatal(err)
				}
				if err := db.ToggleLibrary(id); err != nil {
					t.Fatal(err)
				}
			},
			wantIndexed: true, wantTitle: "Old Main", wantAlt: "Alias One",
		},
		{
			name: "add alt titles",
			apply: func(t *testing.T, db *DB, id int64) {
				if _, err := db.AddAltTitles(fmt.Sprint(id), []string{"Alias Two"}, "mal"); err != nil {
					t.Fatal(err)
				}
			},
			wantIndexed: true, wantTitle: "Old Main", wantAlt: "Alias One Alias Two",
		},
		{
			name: "remove alt title",
			apply: func(t *testing.T, db *DB, id int64) {
				if err := db.RemoveAltTitle(fmt.Sprint(id), "Alias One"); err != nil {
					t.Fatal(err)
				}
			},
			wantIndexed: true, wantTitle: "Old Main", wantAlt: "",
		},
		{
			name: "swap main title",
			apply: func(t *testing.T, db *DB, _ int64) {
				if err := db.SwapMainTitle("p1", "s1", "Alias One"); err != nil {
					t.Fatal(err)
				}
			},
			wantIndexed: true, wantTitle: "Alias One", wantAlt: "Old Main",
		},
		{
			name: "reset enrichment",
			apply: func(t *testing.T, db *DB, id int64) {
				if err := db.ResetEnrichment(fmt.Sprint(id)); err != nil {
					t.Fatal(err)
				}
			},
			wantIndexed: true, wantTitle: "Old Main", wantAlt: "",
		},
		{
			name: "repoint manga",
			apply: func(t *testing.T, db *DB, id int64) {
				if err := db.RepointManga(id, "p2", "s2", "Repointed", "", "", ""); err != nil {
					t.Fatal(err)
				}
			},
			wantIndexed: true, wantTitle: "Repointed", wantAlt: "Alias One",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			id, err := db.UpsertManga(Manga{PluginID: "p1", SourceMangaID: "s1", Title: "Old Main", InLibrary: true})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.AddAltTitles(fmt.Sprint(id), []string{"Alias One"}, "mal"); err != nil {
				t.Fatal(err)
			}
			assertFTSConsistent(t, db)

			tc.apply(t, db, id)

			assertFTSConsistent(t, db)
			title, alt, ok := ftsRow(t, db, id)
			if ok != tc.wantIndexed {
				t.Fatalf("indexed=%v, want %v", ok, tc.wantIndexed)
			}
			if tc.wantIndexed && (title != tc.wantTitle || alt != tc.wantAlt) {
				t.Fatalf("indexed (%q, %q), want (%q, %q)", title, alt, tc.wantTitle, tc.wantAlt)
			}
		})
	}
}

// TestSyncFTSReplacesNotDuplicates: regression for the string/integer key
// mismatch that made SyncFTS insert a fresh row without deleting the old one.
func TestSyncFTSReplacesNotDuplicates(t *testing.T) {
	db := openTestDB(t)
	id, err := db.UpsertManga(Manga{PluginID: "p1", SourceMangaID: "s1", Title: "Old Main", InLibrary: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := db.SyncFTS(fmt.Sprint(id)); err != nil {
			t.Fatal(err)
		}
	}
	if n := ftsCount(t, db); n != 1 {
		t.Fatalf("fts rows = %d, want 1", n)
	}
	assertFTSConsistent(t, db)
}
