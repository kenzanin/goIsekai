package pluginmanager

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goisekai/internal/hostnet"
)

// mangabuddyPage renders one page <img> the way mangabuddy1.co.uk serves it
// today. The CDN host is cdn4.sags2mobile.com - it used to be
// cdn1.love4awalk.xyz - and the page shape is {slug}/{chapter}/{index}.webp.
//
// The plugin used to pin the old host inside its matcher, so the day the site
// moved its CDN every chapter came back with zero pages.
func mangabuddyPage(slug, chapter string, index int) string {
	return fmt.Sprintf(`<img data-src="https://cdn4.sags2mobile.com/%s/%s/%d.webp">`, slug, chapter, index)
}

// mangabuddyChapterPage is one chapter page: the page images, plus the cover
// and the ad script that sit in the same markup. Neither of those has the
// {slug}/{chapter}/{index}.webp shape, so the matcher must skip them.
func mangabuddyChapterPage(slug, chapter string, count int) string {
	var b strings.Builder
	b.WriteString(`<img src="https://mangabuddy1.co.uk/assets/logo.png">`)
	b.WriteString(`<img data-src="https://cdn4.sags2mobile.com/thumb/` + slug + `.webp">`)
	b.WriteString(`<script src="https://cdn4.sags2mobile.com/ads/invoke.js"></script>`)
	for i := range count {
		b.WriteString(mangabuddyPage(slug, chapter, i))
	}
	return b.String()
}

// TestLuaMangabuddyPageListMatchesCurrentCDN pins the page matcher of the
// shipped Lua plugin against the CDN the site serves today.
//
// The old matcher hardcoded cdn1.love4awalk.xyz, so after the site moved to
// cdn4.sags2mobile.com it matched nothing and every chapter opened empty.
// The matcher keys on the page SHAPE, not on the host, so a future CDN move
// does not break it again.
func TestLuaMangabuddyPageListMatchesCurrentCDN(t *testing.T) {
	const slug = "uglymug-epicfighter.ZDOW2Q"
	const chapter = "chapter-1"
	const count = 5

	page := mangabuddyChapterPage("uglymug-epicfighter", "1", count)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, page)
	}))
	defer srv.Close()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	defer func() { _ = mgr.Close() }()
	if _, err := mgr.Install("../../examples/plugins/lua/mangabuddy"); err != nil {
		t.Fatalf("install mangabuddy: %v", err)
	}
	if err := mgr.ensureLoaded("mangabuddy"); err != nil {
		t.Fatalf("load mangabuddy: %v", err)
	}
	mgr.mu.RLock()
	state := mgr.plugins["mangabuddy"].lunar
	mgr.mu.RUnlock()
	if _, err := state.DoString("test-base", fmt.Sprintf("BASE = %q", srv.URL)); err != nil {
		t.Fatalf("retarget BASE: %v", err)
	}

	pages, err := mgr.GetPageList("mangabuddy", slug+":"+chapter)
	if err != nil {
		t.Fatalf("GetPageList: %v", err)
	}
	if len(pages) != count {
		t.Fatalf("got %d pages, want %d", len(pages), count)
	}
	for i, p := range pages {
		want := fmt.Sprintf("https://cdn4.sags2mobile.com/uglymug-epicfighter/1/%d.webp", i)
		if p.URL != want {
			t.Errorf("page %d url = %q, want %q", i, p.URL, want)
		}
	}
}

// The cover (/thumb/...) and the ad script must never be mistaken for a page.
func TestLuaMangabuddyPageListSkipsCoverAndAds(t *testing.T) {
	const slug = "uglymug-epicfighter.ZDOW2Q"
	const chapter = "chapter-1"

	page := `<img data-src="https://cdn4.sags2mobile.com/thumb/uglymug-epicfighter.webp">` +
		`<script src="https://cdn4.sags2mobile.com/ads/invoke.js"></script>` +
		mangabuddyPage("uglymug-epicfighter", "1", 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, page)
	}))
	defer srv.Close()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	defer func() { _ = mgr.Close() }()
	if _, err := mgr.Install("../../examples/plugins/lua/mangabuddy"); err != nil {
		t.Fatalf("install mangabuddy: %v", err)
	}
	if err := mgr.ensureLoaded("mangabuddy"); err != nil {
		t.Fatalf("load mangabuddy: %v", err)
	}
	mgr.mu.RLock()
	state := mgr.plugins["mangabuddy"].lunar
	mgr.mu.RUnlock()
	if _, err := state.DoString("test-base", fmt.Sprintf("BASE = %q", srv.URL)); err != nil {
		t.Fatalf("retarget BASE: %v", err)
	}

	pages, err := mgr.GetPageList("mangabuddy", slug+":"+chapter)
	if err != nil {
		t.Fatalf("GetPageList: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	if !strings.HasSuffix(pages[0].URL, "/1/0.webp") {
		t.Errorf("page url = %q, want the /1/0.webp page", pages[0].URL)
	}
}

// The ABI wants a page index on every page; the plugin must number them in
// reading order.
func TestLuaMangabuddyPageListIsNumbered(t *testing.T) {
	const slug = "uglymug-epicfighter.ZDOW2Q"
	const chapter = "chapter-1"

	page := mangabuddyChapterPage("uglymug-epicfighter", "1", 3)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, page)
	}))
	defer srv.Close()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	defer func() { _ = mgr.Close() }()
	if _, err := mgr.Install("../../examples/plugins/lua/mangabuddy"); err != nil {
		t.Fatalf("install mangabuddy: %v", err)
	}
	if err := mgr.ensureLoaded("mangabuddy"); err != nil {
		t.Fatalf("load mangabuddy: %v", err)
	}
	mgr.mu.RLock()
	state := mgr.plugins["mangabuddy"].lunar
	mgr.mu.RUnlock()
	if _, err := state.DoString("test-base", fmt.Sprintf("BASE = %q", srv.URL)); err != nil {
		t.Fatalf("retarget BASE: %v", err)
	}

	pages, err := mgr.GetPageList("mangabuddy", slug+":"+chapter)
	if err != nil {
		t.Fatalf("GetPageList: %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("got %d pages, want 3", len(pages))
	}
	for i, p := range pages {
		if p.Index != i {
			t.Errorf("page %d has index %d, want %d", i, p.Index, i)
		}
	}
}

// A page list that comes back empty is a failure signal, not an answer: the
// host must not cache it, or the chapter stays empty forever.
func TestLuaMangabuddyPageListEmptyOnUnreachableSite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	defer func() { _ = mgr.Close() }()
	if _, err := mgr.Install("../../examples/plugins/lua/mangabuddy"); err != nil {
		t.Fatalf("install mangabuddy: %v", err)
	}
	if err := mgr.ensureLoaded("mangabuddy"); err != nil {
		t.Fatalf("load mangabuddy: %v", err)
	}
	mgr.mu.RLock()
	state := mgr.plugins["mangabuddy"].lunar
	mgr.mu.RUnlock()
	if _, err := state.DoString("test-base", fmt.Sprintf("BASE = %q", srv.URL)); err != nil {
		t.Fatalf("retarget BASE: %v", err)
	}

	pages, err := mgr.GetPageList("mangabuddy", "uglymug-epicfighter.ZDOW2Q:chapter-1")
	if err != nil {
		t.Fatalf("GetPageList: %v", err)
	}
	if len(pages) != 0 {
		t.Fatalf("got %d pages from a failing site, want 0", len(pages))
	}
}
