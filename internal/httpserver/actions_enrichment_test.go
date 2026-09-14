package httpserver

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"goisekai/internal/database"
)

// The enrichment panel is a chain of round trips: the template renders a chip
// with a hidden field, the browser posts it, the action handler hands it to the
// service, the service writes the row, and the re-rendered page must show the
// new state. A break anywhere in that chain looks identical to the user — the
// click does nothing. These tests drive the real router and assert the stored
// state, not just the status code.

// seedMangaDesc seeds a manga that already has a plugin-supplied description,
// which is what the alt-summary swap demotes into alt_descriptions.
func seedMangaDesc(t *testing.T, db *database.DB, id, pluginID, sourceID, title, desc string) {
	t.Helper()
	if err := db.UpsertManga(database.Manga{
		ID:            id,
		PluginID:      pluginID,
		SourceMangaID: sourceID,
		Title:         title,
		Description:   desc,
		InLibrary:     true,
	}); err != nil {
		t.Fatalf("upsert manga %s: %v", id, err)
	}
	if err := db.SyncFTS(id); err != nil {
		t.Fatalf("sync fts %s: %v", id, err)
	}
}

// postAction drives one action route with a form body and returns the recorder.
func postAction(t *testing.T, s *Server, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	return rec
}

func TestActionSetTitleSwapsMainAndDemotesOld(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	seedManga(t, db, "p1|m1", "p1", "m1", "Main Title")
	if _, err := db.AddAltTitles("p1|m1", []string{"Alt One", "Alt Two"}, "src"); err != nil {
		t.Fatalf("add alt titles: %v", err)
	}

	if rec := postAction(t, s, "/action/set-title/p1/m1", url.Values{"title": {"Alt One"}}); rec.Code != 200 {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body)
	}

	got, err := db.MangaTitle("p1", "m1")
	if err != nil {
		t.Fatalf("manga title: %v", err)
	}
	if got != "Alt One" {
		t.Errorf("main title = %q, want Alt One", got)
	}
	alts := altTitleNames(t, db, "p1|m1")
	if !contains(alts, "Main Title") {
		t.Errorf("old main title was not demoted into alt_titles: %v", alts)
	}
	if contains(alts, "Alt One") {
		t.Errorf("promoted title is still listed as an alternative: %v", alts)
	}
	if !contains(alts, "Alt Two") {
		t.Errorf("unrelated alt title was dropped: %v", alts)
	}
}

func TestActionSetTitleRejectsUnknownTitle(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	seedManga(t, db, "p1|m1", "p1", "m1", "Main Title")

	rec := postAction(t, s, "/action/set-title/p1/m1", url.Values{"title": {"Never Fetched"}})
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400 for a title not in the alt list", rec.Code)
	}
	got, err := db.MangaTitle("p1", "m1")
	if err != nil {
		t.Fatalf("manga title: %v", err)
	}
	if got != "Main Title" {
		t.Errorf("main title was changed to %q despite the rejection", got)
	}
}

func TestActionRemoveAltTitleDropsRow(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	seedManga(t, db, "p1|m1", "p1", "m1", "Main Title")
	if _, err := db.AddAltTitles("p1|m1", []string{"Alt One", "Alt Two"}, "src"); err != nil {
		t.Fatalf("add alt titles: %v", err)
	}

	if rec := postAction(t, s, "/action/remove-alt-title/p1/m1", url.Values{"title": {"Alt One"}}); rec.Code != 200 {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body)
	}
	alts := altTitleNames(t, db, "p1|m1")
	if contains(alts, "Alt One") {
		t.Errorf("Alt One was not removed: %v", alts)
	}
	if !contains(alts, "Alt Two") {
		t.Errorf("Alt Two was removed too: %v", alts)
	}
}

func TestActionSetSummarySwapsMainAndDemotesOld(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	seedMangaDesc(t, db, "p1|m1", "p1", "m1", "Main Title", "Original synopsis.")
	if _, err := db.AddAltDescriptions("p1|m1", []string{"Better synopsis."}, "src"); err != nil {
		t.Fatalf("add alt descriptions: %v", err)
	}

	if rec := postAction(t, s, "/action/set-summary/p1/m1", url.Values{"description": {"Better synopsis."}}); rec.Code != 200 {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body)
	}

	desc, custom, err := db.MangaDescriptionIfCustom("p1", "m1")
	if err != nil {
		t.Fatalf("manga description: %v", err)
	}
	if desc != "Better synopsis." {
		t.Errorf("main description = %q, want Better synopsis.", desc)
	}
	if !custom {
		t.Error("description was not flagged custom, so a plugin refresh would overwrite it")
	}
	sums := altSummaryTexts(t, db, "p1|m1")
	if !contains(sums, "Original synopsis.") {
		t.Errorf("old main synopsis was not demoted: %v", sums)
	}
	if contains(sums, "Better synopsis.") {
		t.Errorf("promoted synopsis is still listed as an alternative: %v", sums)
	}
}

func TestActionSetSummaryRejectsEmpty(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	seedMangaDesc(t, db, "p1|m1", "p1", "m1", "Main Title", "Original synopsis.")

	for _, in := range []string{"", "   "} {
		rec := postAction(t, s, "/action/set-summary/p1/m1", url.Values{"description": {in}})
		if rec.Code != 400 {
			t.Fatalf("description %q: status = %d, want 400", in, rec.Code)
		}
	}
	desc, _, err := db.MangaDescriptionIfCustom("p1", "m1")
	if err != nil {
		t.Fatalf("manga description: %v", err)
	}
	if desc != "Original synopsis." {
		t.Errorf("description = %q, want the original preserved", desc)
	}
}

func TestActionRemoveAltSummaryDropsRow(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	seedMangaDesc(t, db, "p1|m1", "p1", "m1", "Main Title", "Original synopsis.")
	if _, err := db.AddAltDescriptions("p1|m1", []string{"Alt synopsis A", "Alt synopsis B"}, "src"); err != nil {
		t.Fatalf("add alt descriptions: %v", err)
	}

	if rec := postAction(t, s, "/action/remove-alt-summary/p1/m1", url.Values{"description": {"Alt synopsis A"}}); rec.Code != 200 {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body)
	}
	sums := altSummaryTexts(t, db, "p1|m1")
	if contains(sums, "Alt synopsis A") {
		t.Errorf("Alt synopsis A was not removed: %v", sums)
	}
	if !contains(sums, "Alt synopsis B") {
		t.Errorf("Alt synopsis B was removed too: %v", sums)
	}
}

func TestActionAddCategoryStoresAndTogglesGenre(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	seedManga(t, db, "p1|m1", "p1", "m1", "Main Title")

	if rec := postAction(t, s, "/action/add-category/p1/m1", url.Values{"category": {"Isekai"}}); rec.Code != 200 {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body)
	}

	cats := categoryValues(t, db, "p1|m1")
	if !contains(cats, "Isekai") {
		t.Errorf("category was not stored: %v", cats)
	}
	// The handler also toggles the genre override so the library reflects it.
	if !contains(genreOverride(t, db, "p1|m1"), "Isekai") {
		t.Errorf("category was not mirrored into the genre override: %v", genreOverride(t, db, "p1|m1"))
	}

	// Second click removes it again (add-category is only offered when absent,
	// but the toggle must still be symmetric).
	if rec := postAction(t, s, "/action/add-category/p1/m1", url.Values{"category": {"Isekai"}}); rec.Code != 200 {
		t.Fatalf("second add: status = %d, want 200", rec.Code)
	}
	if contains(genreOverride(t, db, "p1|m1"), "Isekai") {
		t.Error("genre override did not toggle back off")
	}
}

func TestActionRemoveCategoryDropsRowAndTogglesGenre(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	seedManga(t, db, "p1|m1", "p1", "m1", "Main Title")
	if err := db.AddCategory("p1|m1", "Drama"); err != nil {
		t.Fatalf("add category: %v", err)
	}
	if err := db.SetMangaGenres("p1|m1", []string{"Drama"}); err != nil {
		t.Fatalf("set genres: %v", err)
	}

	if rec := postAction(t, s, "/action/remove-category/p1/m1", url.Values{"category": {"Drama"}}); rec.Code != 200 {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body)
	}
	if cats := categoryValues(t, db, "p1|m1"); contains(cats, "Drama") {
		t.Errorf("category was not removed: %v", cats)
	}
	if contains(genreOverride(t, db, "p1|m1"), "Drama") {
		t.Error("genre override was not toggled off with the category")
	}
}

func TestActionRemoveRelatedDropsRow(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	seedManga(t, db, "p1|m1", "p1", "m1", "Main Title")
	if _, err := db.AddRelated("p1|m1", []database.RelatedRow{{Title: "Other Manga"}, {Title: "Keep Me"}}, "src"); err != nil {
		t.Fatalf("add related: %v", err)
	}

	if rec := postAction(t, s, "/action/remove-related/p1/m1", url.Values{"title": {"Other Manga"}}); rec.Code != 200 {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body)
	}
	rel, err := db.ListRelated("p1|m1")
	if err != nil {
		t.Fatalf("list related: %v", err)
	}
	var titles []string
	for _, r := range rel {
		titles = append(titles, r.Title)
	}
	if contains(titles, "Other Manga") {
		t.Errorf("related row was not removed: %v", titles)
	}
	if !contains(titles, "Keep Me") {
		t.Errorf("unrelated row was removed too: %v", titles)
	}
}

func TestActionResetEnrichmentClearsEverything(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	seedMangaDesc(t, db, "p1|m1", "p1", "m1", "Main Title", "Original synopsis.")
	if _, err := db.AddAltTitles("p1|m1", []string{"Alt One"}, "src"); err != nil {
		t.Fatalf("add alt titles: %v", err)
	}
	if _, err := db.AddAltDescriptions("p1|m1", []string{"Alt synopsis"}, "src"); err != nil {
		t.Fatalf("add alt descriptions: %v", err)
	}
	if err := db.AddCategory("p1|m1", "Drama"); err != nil {
		t.Fatalf("add category: %v", err)
	}
	if _, err := db.AddRelated("p1|m1", []database.RelatedRow{{Title: "Other Manga"}}, "src"); err != nil {
		t.Fatalf("add related: %v", err)
	}
	if err := db.SetMangaGenres("p1|m1", []string{"Drama"}); err != nil {
		t.Fatalf("set genres: %v", err)
	}
	// Promote a title first so the reset has an override to undo.
	if rec := postAction(t, s, "/action/set-title/p1/m1", url.Values{"title": {"Alt One"}}); rec.Code != 200 {
		t.Fatalf("seed set-title: status = %d, want 200", rec.Code)
	}

	if rec := postAction(t, s, "/action/reset-enrichment/p1/m1", url.Values{}); rec.Code != 200 {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body)
	}

	if got := altTitleNames(t, db, "p1|m1"); len(got) != 0 {
		t.Errorf("alt titles survived the reset: %v", got)
	}
	if got := altSummaryTexts(t, db, "p1|m1"); len(got) != 0 {
		t.Errorf("alt summaries survived the reset: %v", got)
	}
	if got := categoryValues(t, db, "p1|m1"); len(got) != 0 {
		t.Errorf("categories survived the reset: %v", got)
	}
	if rel, err := db.ListRelated("p1|m1"); err != nil {
		t.Fatalf("list related: %v", err)
	} else if len(rel) != 0 {
		t.Errorf("related rows survived the reset: %v", rel)
	}
	if got := genreOverride(t, db, "p1|m1"); len(got) != 0 {
		t.Errorf("genre override survived the reset: %v", got)
	}
	if _, custom, err := db.MangaDescriptionIfCustom("p1", "m1"); err != nil {
		t.Fatalf("manga description: %v", err)
	} else if custom {
		t.Error("description is still marked custom after the reset")
	}
}

// ── read-back helpers ──────────────────────────────────────────────────────

func altTitleNames(t *testing.T, db *database.DB, rowID string) []string {
	t.Helper()
	rows, err := db.ListAltTitles(rowID)
	if err != nil {
		t.Fatalf("list alt titles: %v", err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Title)
	}
	return out
}

func altSummaryTexts(t *testing.T, db *database.DB, rowID string) []string {
	t.Helper()
	rows, err := db.ListAltDescriptions(rowID)
	if err != nil {
		t.Fatalf("list alt descriptions: %v", err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Description)
	}
	return out
}

func categoryValues(t *testing.T, db *database.DB, rowID string) []string {
	t.Helper()
	rows, err := db.ListCategories(rowID)
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Category)
	}
	return out
}

func genreOverride(t *testing.T, db *database.DB, rowID string) []string {
	t.Helper()
	genres, _, err := db.GetMangaGenres(rowID)
	if err != nil {
		t.Fatalf("get manga genres: %v", err)
	}
	return genres
}
