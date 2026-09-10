package database

import (
	"path/filepath"
	"testing"
)

func TestGetMangaCached(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cached.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.UpsertManga(Manga{
		ID: "m1", PluginID: "plugin-1", SourceMangaID: "src-100",
		Title: "Cached Manga", CoverURL: "http://example.com/cover.jpg",
		Description: "A cached manga description", Status: "Ongoing",
	}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}

	// Cache hit: matching plugin+source.
	m, err := db.GetMangaCached("plugin-1", "src-100")
	if err != nil {
		t.Fatalf("GetMangaCached: %v", err)
	}
	if m.ID != "m1" {
		t.Errorf("manga id = %q, want %q", m.ID, "m1")
	}
	if m.Title != "Cached Manga" {
		t.Errorf("title = %q, want %q", m.Title, "Cached Manga")
	}
	if m.CoverURL != "http://example.com/cover.jpg" {
		t.Errorf("cover = %q, want %q", m.CoverURL, "http://example.com/cover.jpg")
	}
	if m.Description != "A cached manga description" {
		t.Errorf("desc = %q, want %q", m.Description, "A cached manga description")
	}

	// Cache miss: wrong plugin returns empty manga.
	m2, err := db.GetMangaCached("plugin-2", "src-100")
	if err != nil {
		t.Fatalf("GetMangaCached plugin miss: %v", err)
	}
	if m2.ID != "" {
		t.Errorf("expected empty manga for cache miss, got id=%q", m2.ID)
	}

	// Cache miss: wrong source id returns empty manga.
	m2, err = db.GetMangaCached("plugin-1", "src-999")
	if err != nil {
		t.Fatalf("GetMangaCached source miss: %v", err)
	}
	if m2.ID != "" {
		t.Errorf("expected empty manga for cache miss, got id=%q", m2.ID)
	}
}

func TestListChaptersCached(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chapters.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.UpsertManga(Manga{
		ID: "plugin-1|src-100", PluginID: "plugin-1", SourceMangaID: "src-100",
		Title: "Test Manga",
	}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}

	// Insert chapters in reverse numeric order (DB order should sort descending).
	// Chapter row ids are "<pluginID>|<mangaRowID>|<sourceChapterID>" and MangaID
	// is the manga row id "<pluginID>|<sourceMangaID>".
	chapters := []Chapter{
		{ID: "plugin-1|src-100|ch-1", MangaID: "plugin-1|src-100", SourceChapterID: "ch-1", Title: "Chapter 1", ChapterNum: 1},
		{ID: "plugin-1|src-100|ch-3", MangaID: "plugin-1|src-100", SourceChapterID: "ch-3", Title: "Chapter 3", ChapterNum: 3},
		{ID: "plugin-1|src-100|ch-2", MangaID: "plugin-1|src-100", SourceChapterID: "ch-2", Title: "Chapter 2", ChapterNum: 2},
	}
	for _, c := range chapters {
		if err := db.UpsertChapter(c); err != nil {
			t.Fatalf("upsert chapter %s: %v", c.ID, err)
		}
	}

	// Fetch cached chapters.
	cached, err := db.ListChaptersCached("plugin-1|src-100")
	if err != nil {
		t.Fatalf("ListChaptersCached: %v", err)
	}
	if len(cached) != 3 {
		t.Fatalf("expected 3 chapters, got %d", len(cached))
	}

	// Must be newest-first (descending chapter_num).
	for i, c := range cached {
		wantNum := float64(3 - i)
		if c.ChapterNum != wantNum {
			t.Errorf("chapter[%d].ChapterNum = %v, want %v", i, c.ChapterNum, wantNum)
		}
	}
}

func TestListChaptersCachedEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	// No chapters stored.
	cached, err := db.ListChaptersCached("nonexistent")
	if err != nil {
		t.Fatalf("ListChaptersCached: %v", err)
	}
	if cached == nil {
		t.Fatal("expected empty slice (not nil) for no chapters")
	}
	if len(cached) != 0 {
		t.Fatalf("expected 0 chapters, got %d", len(cached))
	}
}

func TestMangaDescriptionIfCustom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-desc.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.UpsertManga(Manga{
		ID: "m1", PluginID: "p1", SourceMangaID: "s1",
		Title: "Original Title", Description: "Original description",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// No custom description yet.
	_, custom, err := db.MangaDescriptionIfCustom("p1", "s1")
	if err != nil {
		t.Fatalf("MangaDescriptionIfCustom: %v", err)
	}
	if custom {
		t.Fatal("custom should be false when no swap was made")
	}

	// Upsert with custom_description=1 — simulates user swap.
	_, err = db.db.Exec(
		`UPDATE mangas SET custom_description = 1, description = ? WHERE plugin_id = ? AND source_manga_id = ?`,
		"Custom swapped description", "p1", "s1",
	)
	if err != nil {
		t.Fatalf("set custom: %v", err)
	}

	// Now the custom description should be returned.
	dbDesc, custom, err := db.MangaDescriptionIfCustom("p1", "s1")
	if err != nil {
		t.Fatalf("MangaDescriptionIfCustom after swap: %v", err)
	}
	if !custom {
		t.Fatal("custom should be true after swap")
	}
	if dbDesc != "Custom swapped description" {
		t.Errorf("custom desc = %q, want %q", dbDesc, "Custom swapped description")
	}

	// Original description should be preserved by UpsertManga when custom_description=1.
	if err := db.UpsertManga(Manga{
		ID: "m1", PluginID: "p1", SourceMangaID: "s1",
		Title: "New Title", Description: "New plugin description",
	}); err != nil {
		t.Fatalf("upsert with new data: %v", err)
	}

	dbDesc, _, err = db.MangaDescriptionIfCustom("p1", "s1")
	if err != nil {
		t.Fatalf("MangaDescriptionIfCustom after re-upsert: %v", err)
	}
	if dbDesc != "Custom swapped description" {
		t.Errorf("custom desc should survive re-upsert: got %q, want %q", dbDesc, "Custom swapped description")
	}
}

func TestGetMangaDetailsOfflineFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "offline.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Pre-populate DB with a cached manga + chapters (simulating a prior sync).
	if err := db.UpsertManga(Manga{
		ID: "test-plugin|src-555", PluginID: "test-plugin", SourceMangaID: "src-555",
		Title: "Offline Manga", CoverURL: "http://example.com/offline-cover.jpg",
		Description: "Cached description for offline use", Status: "Ongoing",
	}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	rowID := "test-plugin|src-555"
	chapters := []Chapter{
		{ID: "test-plugin|src-555|ch-1", MangaID: rowID, SourceChapterID: "ch-1", Title: "Chapter 1", ChapterNum: 1},
		{ID: "test-plugin|src-555|ch-2", MangaID: rowID, SourceChapterID: "ch-2", Title: "Chapter 2", ChapterNum: 2},
	}
	for _, c := range chapters {
		if err := db.UpsertChapter(c); err != nil {
			t.Fatalf("upsert chapter %s: %v", c.ID, err)
		}
	}

	// Verify DB-level cached fetch works.
	manga, err := db.GetMangaCached("test-plugin", "src-555")
	if err != nil {
		t.Fatalf("GetMangaCached: %v", err)
	}
	if manga.Title != "Offline Manga" {
		t.Errorf("title = %q, want %q", manga.Title, "Offline Manga")
	}
	if manga.CoverURL != "http://example.com/offline-cover.jpg" {
		t.Errorf("cover = %q, want %q", manga.CoverURL, "http://example.com/offline-cover.jpg")
	}
	if manga.Description != "Cached description for offline use" {
		t.Errorf("desc = %q, want %q", manga.Description, "Cached description for offline use")
	}

	cached, err := db.ListChaptersCached(rowID)
	if err != nil {
		t.Fatalf("ListChaptersCached: %v", err)
	}
	if len(cached) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(cached))
	}
}

func TestNewSinceField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new-since.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.db.Exec(
		`INSERT INTO mangas (id, plugin_id, source_manga_id, title, new_since, updated_at, created_at) VALUES ('m1', 'p1', 's1', 'New Manga', '2026-09-10T12:00:00Z', '2026-09-10T12:00:00Z', '2026-09-10T12:00:00Z')`,
	)
	if err != nil {
		t.Fatalf("insert with new_since: %v", err)
	}

	// Verify new_since is stored.
	var ns string
	err = db.db.QueryRow(`SELECT new_since FROM mangas WHERE id = ?`, "m1").Scan(&ns)
	if err != nil {
		t.Fatalf("scan new_since: %v", err)
	}
	if ns == "" {
		t.Fatal("new_since should not be empty")
	}
}
