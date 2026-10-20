package pluginmanager

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"goisekai/internal/hostnet"
)

// weebcentralChapterRow is one row of the site's /full-chapter-list
// fragment, in the shape weebcentral.com serves it: the anchor carries the
// chapter label, a "Last Read" marker and a <time> publish stamp.
//
// The site serves the list NEWEST-FIRST, and that is the ABI order the host
// expects (its cached list is sorted by chapter_num descending and its reader
// resolves prev/next assuming newest-first). The plugin used to reverse the
// fragment, which made the detail page list chapters oldest-first.
func weebcentralChapterRow(chid, label, published string) string {
	return fmt.Sprintf(`<a href="/chapters/%[1]s" class="hover:bg-base-300 flex-1 flex items-center p-2">
<span class="grow flex items-center gap-2">
<span class="">%[2]s</span>
<span class="hidden md:inline">Last Read</span>
</span>
<time class="text-datetime opacity-50" datetime="%[3]s">%[3]s</time>
</a>`, chid, label, published)
}

// TestLuaWeebcentralChaptersStayNewestFirst pins the chapter order of the
// Lua reference plugin: the list comes off the site newest-first and must be
// delivered newest-first, each row carrying its chapter number.
//
// The plugin used to reverse the fragment with a "reader wants ascending"
// comment, which listed the detail page oldest-first.
func TestLuaWeebcentralChaptersStayNewestFirst(t *testing.T) {
	rows := []struct{ chid, label, published string }{
		{"01JCDXGQQ42DQZEET3VMBECT71", "Chapter 3", "2024-11-11T15:29:45.641Z"},
		{"01JCDXGQQ42DQZEET3VMBECT72", "Chapter 2", "2024-10-02T09:00:00.000Z"},
		{"01JCDXGQQ42DQZEET3VMBECT73", "Chapter 1", "2024-09-07T17:04:15.717Z"},
	}
	var body strings.Builder
	for _, r := range rows {
		body.WriteString(weebcentralChapterRow(r.chid, r.label, r.published))
	}
	page := "<!doctype html><html><body>" + body.String() + "</body></html>"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, page)
	}))
	defer srv.Close()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	defer func() { _ = mgr.Close() }()
	if _, err := mgr.Install("../../examples/plugins/lua/weebcentral"); err != nil {
		t.Fatalf("install weebcentral: %v", err)
	}
	if err := mgr.ensureLoaded("weebcentral"); err != nil {
		t.Fatalf("load weebcentral: %v", err)
	}
	mgr.mu.RLock()
	state := mgr.plugins["weebcentral"].lunar
	mgr.mu.RUnlock()
	if _, err := state.DoString("test-base", fmt.Sprintf("BASE = %q", srv.URL)); err != nil {
		t.Fatalf("retarget BASE: %v", err)
	}

	chapters, err := mgr.GetChapterList("weebcentral", "01J76XY99ST5KF8M1N0GH63XN4")
	if err != nil {
		t.Fatalf("GetChapterList: %v", err)
	}
	if len(chapters) != len(rows) {
		t.Fatalf("got %d chapters, want %d: %+v", len(chapters), len(rows), chapters)
	}
	for i, want := range rows {
		got := chapters[i]
		if got.Title != want.label {
			t.Errorf("chapter %d title = %q, want %q (list must stay newest-first)", i, got.Title, want.label)
		}
		if got.ChapterNum != float64(len(rows)-i) {
			t.Errorf("chapter %d (%q) chapter_num = %v, want %v", i, got.Title, got.ChapterNum, len(rows)-i)
		}
		if got.ID != want.chid {
			t.Errorf("chapter %d id = %q, want %q", i, got.ID, want.chid)
		}
	}
	for i := 1; i < len(chapters); i++ {
		if chapters[i].ChapterNum >= chapters[i-1].ChapterNum {
			t.Errorf("chapters are not descending at %d: %v then %v",
				i, chapters[i-1].ChapterNum, chapters[i].ChapterNum)
		}
	}
}

// TestWasmWeebcentralChaptersStayNewestFirst runs the shipped wasm plugin
// against the live site and checks the same invariant. The wasm plugin has no
// way to retarget its site, so this is the live half: skipped unless
// GOISEKAI_LIVE=1, because it needs the network.
func TestWasmWeebcentralChaptersStayNewestFirst(t *testing.T) {
	if os.Getenv("GOISEKAI_LIVE") == "" {
		t.Skip("set GOISEKAI_LIVE=1 to hit the real site")
	}

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	defer func() { _ = mgr.Close() }()
	if _, err := mgr.Install("../../examples/plugins/wasm/weebcentral_odin"); err != nil {
		t.Fatalf("install weebcentral_odin: %v", err)
	}

	chapters, err := mgr.GetChapterList("weebcentral_odin", "01J76XY99ST5KF8M1N0GH63XN4")
	if err != nil {
		t.Fatalf("GetChapterList: %v", err)
	}
	if len(chapters) < 2 {
		t.Fatalf("got %d chapters, want at least 2", len(chapters))
	}
	for i, c := range chapters {
		if c.ChapterNum == 0 {
			t.Errorf("chapter %d (%q) has no chapter number", i, c.Title)
		}
		if i > 0 && c.ChapterNum >= chapters[i-1].ChapterNum {
			t.Errorf("chapters are not descending at %d: %v then %v",
				i, chapters[i-1].ChapterNum, c.ChapterNum)
		}
	}
	t.Logf("live: %d chapters, newest-first, all numbered", len(chapters))
}

// TestWasmComizyChaptersStayNewestFirst is the live half for comizy, whose
// API base cannot be retargeted. It also checks the split-chapter labels the
// site uses ("Chapter 12-1", "Chapter 12-2"): the host's text.chapter_num
// reads the leading number only, so the plugin folds the hyphen into the
// decimal the dot form already uses. Without that the two halves of a split
// chapter collide on one number.
func TestWasmComizyChaptersStayNewestFirst(t *testing.T) {
	if os.Getenv("GOISEKAI_LIVE") == "" {
		t.Skip("set GOISEKAI_LIVE=1 to hit the real site")
	}

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	defer func() { _ = mgr.Close() }()
	if _, err := mgr.Install("../../examples/plugins/wasm/comizy_odin"); err != nil {
		t.Fatalf("install comizy_odin: %v", err)
	}

	const mangaID = "4OGVJBjM|a-banished-failure-survives-in-the-borderland-and-becomes-an-s-ranked-exorcist"
	chapters, err := mgr.GetChapterList("comizy_odin", mangaID)
	if err != nil {
		t.Fatalf("GetChapterList: %v", err)
	}
	if len(chapters) < 2 {
		t.Fatalf("got %d chapters, want at least 2", len(chapters))
	}
	seen := make(map[float64]string, len(chapters))
	for i, c := range chapters {
		if c.ChapterNum == 0 {
			t.Errorf("chapter %d (%q) has no chapter number", i, c.Title)
		}
		if i > 0 && c.ChapterNum >= chapters[i-1].ChapterNum {
			t.Errorf("chapters are not descending at %d: %v then %v",
				i, chapters[i-1].ChapterNum, c.ChapterNum)
		}
		if prev, dup := seen[c.ChapterNum]; dup {
			t.Errorf("chapter number %v is used twice: %q and %q", c.ChapterNum, prev, c.Title)
		}
		seen[c.ChapterNum] = c.Title
	}
	t.Logf("live: %d chapters, newest-first, %d distinct numbers", len(chapters), len(seen))
}
