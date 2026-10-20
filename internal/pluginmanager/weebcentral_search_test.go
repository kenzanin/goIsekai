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
	"goisekai/pkg/types"
)

// weebcentralCard is one search card in the shape weebcentral.com actually
// serves: an outer article.bg-base-300 holding a picture anchor (with a
// desktop AND a mobile <img> of the same cover) and a separate title anchor.
//
// The two <img> per card are the point of this fixture. The plugin used to
// zip the cover list with the card list by position, which assumed one cover
// per card; with two, every cover landed on the PREVIOUS card's title.
func weebcentralCard(ulid, title string) string {
	cover := "https://temp.compsci88.com/cover/fallback/" + ulid + ".jpg"
	return fmt.Sprintf(`<article class="bg-base-300 flex gap-4 p-4">
	<section class="w-full lg:w-[25%%]">
		<a href="https://weebcentral.com/series/%[1]s/some-slug">
			<article class="hidden lg:block w-full aspect-4/6 overflow-hidden">
				<picture>
					<source srcset="https://temp.compsci88.com/cover/normal/%[1]s.webp" type="image/webp">
					<img src="%[2]s" alt="%[3]s cover" width="400" height="600">
				</picture>
			</article>
			<article class="lg:hidden relative overflow-hidden">
				<div class="w-full aspect-4/6 overflow-hidden">
					<picture>
						<img src="%[2]s" alt="%[3]s cover" width="400" height="600">
					</picture>
				</div>
			</article>
		</a>
		<div class="min-w-0">
			<a href="https://weebcentral.com/series/%[1]s/some-slug" class="line-clamp-1 link link-hover">%[3]s</a>
		</div>
	</section>
</article>`, ulid, cover, title)
}

const weebcentralStubPage = `<!doctype html><html><body>` +
	`{{cards}}` +
	`</body></html>`

// TestLuaWeebcentralCoversMatchTheirTitle pins the search-card contract for
// the Lua reference plugin: a cover belongs to the card whose ULID appears in
// the cover URL, never to the card at the same list position.
//
// The old positional zip only got the FIRST card right and shifted the rest
// by one, so this asserts every result, not just the first.
func TestLuaWeebcentralCoversMatchTheirTitle(t *testing.T) {
	cards := []struct{ ulid, title string }{
		{"01J76XY99ST5KF8M1N0GH63XN4", "Is It Wrong to Try to Pick Up Girls in a Dungeon?"},
		{"01J76XYAMX5TJPMMT957KY6AQF", "Is It Wrong to Try to Pick Up Girls in a Dungeon? Sword Oratoria"},
		{"01J76XYBAVFJ3WEWWF3994R74Z", "Delicious in Dungeon"},
	}
	var body strings.Builder
	for _, c := range cards {
		body.WriteString(weebcentralCard(c.ulid, c.title))
	}
	page := strings.ReplaceAll(weebcentralStubPage, "{{cards}}", body.String())

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

	results, err := mgr.Search("weebcentral", types.SearchFilter{Query: "dungeon"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != len(cards) {
		t.Fatalf("search returned %d results, want %d: %+v", len(results), len(cards), results)
	}
	for i, r := range results {
		if r.ID != cards[i].ulid {
			t.Errorf("result %d id = %q, want %q", i, r.ID, cards[i].ulid)
		}
		if r.Title != cards[i].title {
			t.Errorf("result %d title = %q, want %q", i, r.Title, cards[i].title)
		}
		if !strings.Contains(r.CoverURL, r.ID) {
			t.Errorf("result %d (%q) cover %q belongs to another card, want the one carrying %q",
				i, r.Title, r.CoverURL, r.ID)
		}
	}
}

// TestWasmWeebcentralCoversMatchTheirTitle runs the shipped wasm plugin
// against the live site and checks the same invariant. The wasm plugin has no
// way to retarget its site, so this is the live half: skipped unless
// GOISEKAI_LIVE=1, because it needs the network.
func TestWasmWeebcentralCoversMatchTheirTitle(t *testing.T) {
	if os.Getenv("GOISEKAI_LIVE") == "" {
		t.Skip("set GOISEKAI_LIVE=1 to hit the real site")
	}

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	defer func() { _ = mgr.Close() }()
	if _, err := mgr.Install("../../examples/plugins/wasm/weebcentral_odin"); err != nil {
		t.Fatalf("install weebcentral_odin: %v", err)
	}

	results, err := mgr.Search("weebcentral_odin", types.SearchFilter{Query: "dungeon"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("search returned nothing")
	}
	for i, r := range results {
		if r.CoverURL == "" {
			t.Errorf("result %d (%q) has no cover", i, r.Title)
			continue
		}
		if !strings.Contains(r.CoverURL, r.ID) {
			t.Errorf("result %d (%q) cover %q belongs to another card, want the one carrying %q",
				i, r.Title, r.CoverURL, r.ID)
		}
	}
	t.Logf("live: %d results, every cover matches its own series", len(results))
}
