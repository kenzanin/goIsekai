package httpserver

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"goisekai/internal/database"
	"goisekai/pkg/types"
)

// Every /view/... page renders forms. csrfInput reads data.csrf_token, but the
// partials that render the detail and plugin pages receive an explicit literal
// table rather than the view's own data — so the token silently arrived as ""
// and 14 buttons posted an empty token. This walks every view and asserts no
// rendered form carries an empty one.
func TestNoRenderedFormHasAnEmptyCSRFToken(t *testing.T) {
	s := testServerFull(t, "", true)
	for _, path := range []string{
		"/view/library", "/view/settings", "/view/plugins",
		"/view/logs", "/view/about", "/view/history", "/view/updates",
	} {
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, rec.Code)
			continue
		}
		if n := strings.Count(rec.Body.String(), `name="csrf_token" value=""`); n > 0 {
			t.Errorf("%s: %d form(s) render an empty csrf_token; the token is not reaching that view", path, n)
		}
	}
}

// The bulk mark read/unread checkbox must mirror the chapter's read state, or a
// finished chapter looks unread on the page that reports its progress.
// The checkbox is a selection, not a read indicator.
//
// It used to render checked whenever the chapter was read, which silently broke
// the range actions: "set up to" takes an extreme of the ticked rows, so every
// already-read chapter - all pre-ticked - widened the range. Ticking chapter 27
// on a manga whose 1-20 were read set 1..27 instead of just what was selected.
//
// Read state stays visible without the checkbox: the row already carries a
// strikethrough title and a green tick, so nothing is lost by decoupling them.
func TestChapterCheckboxIsSelectionNotReadState(t *testing.T) {
	s := testServerFull(t, "", true)
	chapters := []types.Chapter{{ID: "c-unread", ChapterNum: 1}, {ID: "c-read", ChapterNum: 2}}
	progress := map[string]database.ChapterProgress{
		"c-unread": {LastPageRead: 3, TotalPages: 18},
		"c-read":   {LastPageRead: 9, TotalPages: 9, Done: true},
	}
	body := renderChaptersFragment(t, s, chapters, progress)
	for _, id := range []string{"c-read", "c-unread"} {
		if tag := checkboxTag(t, body, id); strings.Contains(tag, "checked") {
			t.Errorf("chapter %s: checkbox rendered checked; it is a selection and "+
				"must start empty after every action. Tag was %s", id, tag)
		}
	}
}

// Read state must still be reported on the row itself, since the checkbox no
// longer carries it. Both signals come from the same Done flag the badge and the
// strikethrough use.
func TestReadChapterRowStillShowsDoneState(t *testing.T) {
	s := testServerFull(t, "", true)
	chapters := []types.Chapter{
		{ID: "c-done", ChapterNum: 1, Title: "Finished Chapter"},
		{ID: "c-open", ChapterNum: 2, Title: "Ongoing Chapter"},
	}
	progress := map[string]database.ChapterProgress{
		"c-done": {LastPageRead: 9, TotalPages: 9, Done: true},
		"c-open": {LastPageRead: 2, TotalPages: 18},
	}
	body := renderChaptersFragment(t, s, chapters, progress)

	done := rowFor(t, body, "c-done")
	if !strings.Contains(done, "line-through") {
		t.Errorf("a finished chapter is not struck through: %s", done)
	}
	if !strings.Contains(done, "text-emerald-400") {
		t.Errorf("a finished chapter has no done badge: %s", done)
	}

	open := rowFor(t, body, "c-open")
	if strings.Contains(open, "line-through") {
		t.Errorf("an unfinished chapter must not be struck through: %s", open)
	}
}

// rowFor returns the rendered row for one chapter, so a done-state assertion
// cannot pass on a neighbouring row's markup.
func rowFor(t *testing.T, body, chapterID string) string {
	t.Helper()
	anchor := `value="` + chapterID + `"`
	i := strings.Index(body, anchor)
	if i < 0 {
		t.Fatalf("no row for chapter %s", chapterID)
	}
	rest := body[i:]
	if j := strings.Index(rest, `<a href="/view/read/`); j > 0 {
		rest = rest[j:]
	}
	if j := strings.Index(rest, `py-3 px-2`); j > 0 {
		rest = rest[:j]
	}
	return rest
}

// checkboxTag returns the full <input ...> tag for one chapter's bulk-action
// checkbox, so an assertion cannot wander into a neighbouring row.
func checkboxTag(t *testing.T, body, chapterID string) string {
	t.Helper()
	re := regexp.MustCompile(`<input[^>]*value="` + regexp.QuoteMeta(chapterID) + `"[^>]*>`)
	m := re.FindString(body)
	if m == "" {
		t.Fatalf("no checkbox for chapter %s", chapterID)
	}
	return m
}

// A chapter whose page count was never recorded is unfinished, not finished.
// computeContinue and continueFromHistory used to disagree on this exact case,
// and the history-driven one won, so Continue skipped to a newer unread chapter.
func TestContinueDoesNotSkipAPartiallyReadChapter(t *testing.T) {
	chapters := []types.Chapter{
		{ID: "c5", ChapterNum: 5},
		{ID: "c4", ChapterNum: 4},
		{ID: "c3", ChapterNum: 3},
	}
	progress := map[string]database.ChapterProgress{
		"c3": {LastPageRead: 2}, // total_pages unknown
	}
	got := computeContinue(chapters, progress)
	if got == nil {
		t.Fatal("computeContinue returned nil")
	}
	if got.ChapterID != "c3" {
		t.Errorf("computeContinue chose %q, want c3 (the chapter actually in progress)", got.ChapterID)
	}
	if got.Page != 2 {
		t.Errorf("Page = %d, want 2 (resume where the reader stopped)", got.Page)
	}
}

// renderChaptersFragment runs the chapter partial the same way the detail view
// does, so the assertions see the same bytes the browser would.
func renderChaptersFragment(t *testing.T, s *Server, chapters []types.Chapter, progress map[string]database.ChapterProgress) string {
	t.Helper()
	data := map[string]any{
		"PluginID": "p", "MangaID": "m", "Manga": map[string]any{},
		"Chapters": chapters, "Progress": progress,
		"ChPage": 1, "ChTotalPages": 1, "csrf_token": "tok",
	}
	var buf strings.Builder
	if err := s.engine.RenderPartial(&buf, "partials/detail_chapters", data); err != nil {
		t.Fatalf("render partial: %v", err)
	}
	return buf.String()
}
