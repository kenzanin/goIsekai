package database

import (
	"encoding/json"
	"fmt"
	"testing"
)

// GetAltTitles is the JSON-payload read path over alt_titles: absent manga
// errors, a manga with no rows returns "", and rows collapse to
// {"source": ..., "titles": [...]}.
func TestGetAltTitlesPayloadAndAbsence(t *testing.T) {
	db := openTestDB(t)

	// Unknown manga: the row lookup must fail, not return empty JSON.
	if _, err := db.GetAltTitles("nope", "missing"); err == nil {
		t.Fatal("GetAltTitles on unknown manga must error")
	}

	mangaID, err := db.UpsertManga(Manga{PluginID: "p1", SourceMangaID: "s1", Title: "Main"})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}

	// Manga with no alt rows: empty payload, no error.
	got, err := db.GetAltTitles("p1", "s1")
	if err != nil {
		t.Fatalf("GetAltTitles empty: %v", err)
	}
	if got != "" {
		t.Fatalf("GetAltTitles with no rows = %q, want empty", got)
	}

	if _, err := db.AddAltTitles(fmt.Sprint(mangaID), []string{"Alpha", "Beta"}, "mal"); err != nil {
		t.Fatalf("add: %v", err)
	}
	got, err = db.GetAltTitles("p1", "s1")
	if err != nil {
		t.Fatalf("GetAltTitles: %v", err)
	}
	var payload struct {
		Source string   `json:"source"`
		Titles []string `json:"titles"`
	}
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("payload %q is not JSON: %v", got, err)
	}
	if payload.Source != "mal" {
		t.Fatalf("payload source = %q, want mal", payload.Source)
	}
	if len(payload.Titles) != 2 {
		t.Fatalf("payload titles = %v, want 2 entries", payload.Titles)
	}
	have := map[string]bool{}
	for _, title := range payload.Titles {
		have[title] = true
	}
	if !have["Alpha"] || !have["Beta"] {
		t.Fatalf("payload titles = %v, want Alpha+Beta", payload.Titles)
	}
}
