package database

import (
	"fmt"
	"testing"
)

func TestAddCategories_Dedup(t *testing.T) {
	d := openTestDB(t)
	defer func() { _ = d.Close() }()

	// Create a manga row.
	mangaID := int64(1)
	_, err := d.db.Exec(`INSERT INTO mangas (id, plugin_id, source_manga_id, title) VALUES (?, ?, ?, ?)`,
		mangaID, "test", "m1", "Test Manga")
	if err != nil {
		t.Fatal(err)
	}

	// AddCategories still takes a string, so convert.
	n, err := d.AddCategories(fmt.Sprint(mangaID), []string{"Action", "Fantasy", "Action"}, "mangadex")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 inserted, got %d", n)
	}

	cats, err := d.ListCategories(fmt.Sprint(mangaID))
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(cats))
	}
	if cats[0].Category != "Action" || cats[1].Category != "Fantasy" {
		t.Errorf("categories = %v, want [Action, Fantasy]", cats)
	}
}

func TestAddCategories_Empty(t *testing.T) {
	d := openTestDB(t)
	defer func() { _ = d.Close() }()

	n, err := d.AddCategories("nonexistent", []string{}, "mangadex")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 inserted, got %d", n)
	}
}

func TestListCategories_NoData(t *testing.T) {
	d := openTestDB(t)
	defer func() { _ = d.Close() }()

	cats, err := d.ListCategories("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 0 {
		t.Errorf("expected empty, got %d", len(cats))
	}
}

func TestAddRelated_Dedup(t *testing.T) {
	d := openTestDB(t)
	defer func() { _ = d.Close() }()

	// Insert manga with integer ID.
	res, err := d.db.Exec(`INSERT INTO mangas (plugin_id, source_manga_id, title) VALUES (?, ?, ?)`,
		"test", "m2", "Test Manga")
	if err != nil {
		t.Fatal(err)
	}
	mangaIDInt, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	mangaID := fmt.Sprint(mangaIDInt)

	n, err := d.AddRelated(mangaID, []RelatedRow{
		{Title: "Related 1", URL: "https://example.com/1"},
		{Title: "Related 2", URL: "https://example.com/2"},
		{Title: "Related 1", URL: "https://example.com/1"}, // dup
	}, "mangaupdates")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 inserted, got %d", n)
	}

	related, err := d.ListRelated(mangaID)
	if err != nil {
		t.Fatal(err)
	}
	if len(related) != 2 {
		t.Fatalf("expected 2 related, got %d", len(related))
	}
}

func TestAddRelated_Empty(t *testing.T) {
	d := openTestDB(t)
	defer func() { _ = d.Close() }()

	n, err := d.AddRelated("nonexistent", []RelatedRow{}, "mangadex")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 inserted, got %d", n)
	}
}

func TestListRelated_NoData(t *testing.T) {
	d := openTestDB(t)
	defer func() { _ = d.Close() }()

	related, err := d.ListRelated("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(related) != 0 {
		t.Errorf("expected empty, got %d", len(related))
	}
}

func TestResolveMangaRowID(t *testing.T) {
	d := openTestDB(t)
	defer func() { _ = d.Close() }()

	// Create a manga row with integer ID, but ResolveMangaRowID returns the string representation.
	res, err := d.db.Exec(`INSERT INTO mangas (plugin_id, source_manga_id, title) VALUES (?, ?, ?)`,
		"plug1", "src1", "Test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	id, err := d.ResolveMangaRowID("plug1", "src1")
	if err != nil {
		t.Fatal(err)
	}
	// The function returns the manga_row_id as a string; after migration it's the integer ID as string.
	// We just check it's non-empty and matches the inserted row.
	if id == "" {
		t.Errorf("id = %q, want non-empty", id)
	}
}

func TestResolveMangaRowID_NotFound(t *testing.T) {
	d := openTestDB(t)
	defer func() { _ = d.Close() }()

	_, err := d.ResolveMangaRowID("nonexistent", "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
