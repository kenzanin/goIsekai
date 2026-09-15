package pluginmanager

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/dop251/goja"

	lua "github.com/mmcdole/lunar"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// TestLuaLuaEscape covers the host native plugins call instead of shipping
// their own copy. The slug case is the one that bit: '-' is the lazy
// quantifier, so an unescaped slug silently matches the wrong text.
func TestLuaLuaEscape(t *testing.T) {
	chunk := `
		assert(host.text.lua_escape("a-b") == "a%-b", host.text.lua_escape("a-b"))
		assert(host.text.lua_escape("a.b") == "a%.b")
		assert(host.text.lua_escape("50%") == "50%%")
		assert(host.text.lua_escape("a(b)[c]") == "a%(b%)%[c%]")
		assert(host.text.lua_escape("a+b*c?") == "a%+b%*c%?")
		assert(host.text.lua_escape("plain") == "plain")
		-- the whole point: the escaped slug is a literal pattern
		assert(string.find("x-a-b-y", host.text.lua_escape("a-b")) == 3)
	`
	if err := luaEval(t, chunk); err != nil {
		t.Fatalf("host.text.lua_escape: %v", err)
	}
}

// TestLuaNormalizeStatusForms covers both call shapes plugins use: the
// one-argument default-vocabulary form, and the two-argument form with a
// plugin-supplied map. An empty map must mean "use the defaults", not
// "pass everything through untouched".
func TestLuaNormalizeStatusForms(t *testing.T) {
	chunk := `
		assert(host.text.normalize_status("ongoing") == "Ongoing")
		assert(host.text.normalize_status("RELEASING") == "Ongoing")
		assert(host.text.normalize_status("cancelled") == "Dropped")
		assert(host.text.normalize_status("On-Going") == "Ongoing")
		assert(host.text.normalize_status("") == "")
		-- unknown values pass through, not blanked
		assert(host.text.normalize_status("Weird Status") == "Weird Status")
		-- empty map == defaults
		assert(host.text.normalize_status({}, "completed") == "Completed")
		-- explicit map wins
		assert(host.text.normalize_status({serialised = "Ongoing"}, "serialised") == "Ongoing")
	`
	if err := luaEval(t, chunk); err != nil {
		t.Fatalf("host.text.normalize_status: %v", err)
	}
}

// TestJSNormalizeStatusForms mirrors the Lua coverage for the JS runtime,
// which exposes the same callable with a .default property attached.
func TestJSNormalizeStatusForms(t *testing.T) {
	vm := goja.New()
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "norms"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}

	chunk := `
		function assert(c, msg) { if (!c) throw new Error(msg || "assertion failed"); }
		assert(host.text.normalize_status("ongoing") === "Ongoing");
		assert(host.text.normalize_status("RELEASING") === "Ongoing");
		assert(host.text.normalize_status("cancelled") === "Dropped");
		assert(host.text.normalize_status("Weird Status") === "Weird Status");
		assert(host.text.normalize_status({}, "completed") === "Completed");
		assert(host.text.normalize_status({serialised: "Ongoing"}, "serialised") === "Ongoing");
	`
	if _, err := vm.RunString(chunk); err != nil {
		t.Fatalf("host.text helpers in goja: %v", err)
	}
}

// htmlFixtureMarkup is the markup every runtime scrapes in the equivalence
// checks. The untrimmed title and the img without src are deliberate: they are
// where a runtime that forgot the trim rule or the skip rule would diverge.
// testdata/yaegihtml/main.go carries the same string, which
// TestHostHTMLYaegiFixtureSharesMarkup pins.
const htmlFixtureMarkup = `<div class="header"><h1 class="title">   Solo Leveling   </h1></div>
<div class="chapters"><a href="/c/1">Chapter 1</a><a href="/c/2">Chapter 2</a></div>
<img src="/img1.jpg"><img alt="x"><img src="/img2.jpg">`

// htmlLookups is the shared shape all three runtimes must produce.
type htmlLookups struct {
	Title  string   `json:"title"`
	Href   string   `json:"href"`
	Texts  []string `json:"texts"`
	Srcs   []string `json:"srcs"`
	XTitle string   `json:"xtitle"`
	XHref  string   `json:"xhref"`
	XTexts []string `json:"xtexts"`
	XSrcs  []string `json:"xsrcs"`
}

// sameLookups compares two result sets by value; the slices make the struct
// itself non-comparable.
func sameLookups(a, b htmlLookups) bool {
	return reflect.DeepEqual(a, b)
}

// htmlLookupsExpected is written out rather than computed so that a change in
// any runtime has to be an explicit decision, not a silently matching pair.
var htmlLookupsExpected = htmlLookups{
	Title:  "Solo Leveling",
	Href:   "/c/1",
	Texts:  []string{"Chapter 1", "Chapter 2"},
	Srcs:   []string{"/img1.jpg", "/img2.jpg"},
	XTitle: "Solo Leveling",
	XHref:  "/c/1",
	XTexts: []string{"Chapter 1", "Chapter 2"},
	XSrcs:  []string{"/img1.jpg", "/img2.jpg"},
}

// htmlLookupCalls are the eight lookups written the way a plugin writes them in
// both Lua and JS. The text is shared with both runtimes on purpose: it is what
// proves the two surfaces have the same names, the same argument order, and no
// receiver.
var htmlLookupCalls = []string{
	`find_text(doc, "h1.title")`,
	`find_attr(doc, "a", "href")`,
	`find_list_text(doc, "a")`,
	`find_list_attr(doc, "img", "src")`,
	`xpath_text(doc, "//h1[@class='title']")`,
	`xpath_attr(doc, "//a", "href")`,
	`xpath_list_text(doc, "//a")`,
	`xpath_list_attr(doc, "//img", "src")`,
}

// luaString runs a chunk that returns one string and reads it before the state
// closes.
func luaString(t *testing.T, chunk string) string {
	t.Helper()
	var out string
	runLua(t, chunk, func(t *testing.T, first lua.Value) {
		s, ok := first.AsString()
		if !ok {
			t.Fatalf("expected a string, got %v", first)
		}
		out = s
	})
	return out
}

func jsString(t *testing.T, chunk string) string {
	t.Helper()
	return runJS(t, chunk).String()
}

// luaHTMLLookups and jsHTMLLookups run the same eight lookups over the same
// markup and hand back the same struct, so a divergence shows up as a field
// difference rather than as two opaque strings.
func luaHTMLLookups(t *testing.T) htmlLookups {
	t.Helper()
	encoded := luaString(t, fmt.Sprintf(`
		local doc = host.html.parse(%q)
		return host.json.encode({
			title = host.html.find_text(doc, "h1.title"),
			href = host.html.find_attr(doc, "a", "href"),
			texts = host.html.find_list_text(doc, "a"),
			srcs = host.html.find_list_attr(doc, "img", "src"),
			xtitle = host.html.xpath_text(doc, "//h1[@class='title']"),
			xhref = host.html.xpath_attr(doc, "//a", "href"),
			xtexts = host.html.xpath_list_text(doc, "//a"),
			xsrcs = host.html.xpath_list_attr(doc, "//img", "src"),
		})
	`, htmlFixtureMarkup))
	return decodeLookups(t, "lua", encoded)
}

func jsHTMLLookups(t *testing.T) htmlLookups {
	t.Helper()
	encoded := jsString(t, fmt.Sprintf(`
		var doc = host.html.parse(%q);
		JSON.stringify({
			title: host.html.find_text(doc, "h1.title"),
			href: host.html.find_attr(doc, "a", "href"),
			texts: host.html.find_list_text(doc, "a"),
			srcs: host.html.find_list_attr(doc, "img", "src"),
			xtitle: host.html.xpath_text(doc, "//h1[@class='title']"),
			xhref: host.html.xpath_attr(doc, "//a", "href"),
			xtexts: host.html.xpath_list_text(doc, "//a"),
			xsrcs: host.html.xpath_list_attr(doc, "//img", "src"),
		});
	`, htmlFixtureMarkup))
	return decodeLookups(t, "js", encoded)
}

func decodeLookups(t *testing.T, runtime, encoded string) htmlLookups {
	t.Helper()
	var got htmlLookups
	if err := json.Unmarshal([]byte(encoded), &got); err != nil {
		t.Fatalf("%s lookups did not encode as JSON: %v (got %s)", runtime, err, encoded)
	}
	return got
}

// TestHostHTMLSameValuesEverywhere is 6.1: the same markup and the same
// selectors must produce the same values in the Lua and JS runtimes.
func TestHostHTMLSameValuesEverywhere(t *testing.T) {
	luaGot := luaHTMLLookups(t)
	jsGot := jsHTMLLookups(t)

	if !sameLookups(luaGot, htmlLookupsExpected) {
		t.Errorf("lua lookups = %+v, want %+v", luaGot, htmlLookupsExpected)
	}
	if !sameLookups(jsGot, htmlLookupsExpected) {
		t.Errorf("js lookups = %+v, want %+v", jsGot, htmlLookupsExpected)
	}
	if !sameLookups(luaGot, jsGot) {
		t.Errorf("runtimes disagree:\n lua = %+v\n  js = %+v", luaGot, jsGot)
	}
}

// TestHostHTMLSameCallTextInLuaAndJS is 6.3: each lookup is written once, and
// that exact text is evaluated in both runtimes. If a runtime renamed a lookup
// or took its arguments in a different order, one side would fail to resolve.
func TestHostHTMLSameCallTextInLuaAndJS(t *testing.T) {
	for _, call := range htmlLookupCalls {
		t.Run(call, func(t *testing.T) {
			fromLua := luaString(t, fmt.Sprintf(
				"local doc = host.html.parse(%q)\nreturn host.json.encode(host.html.%s)",
				htmlFixtureMarkup, call))
			fromJS := jsString(t, fmt.Sprintf(
				"var doc = host.html.parse(%q);\nJSON.stringify(host.html.%s)",
				htmlFixtureMarkup, call))

			if fromLua != fromJS {
				t.Fatalf("same call text diverges:\n lua = %s\n  js = %s", fromLua, fromJS)
			}
		})
	}

	// The handle is an argument, so a receiver call must not resolve. Both
	// runtimes fail here, each in its own way: Lua because the group has no
	// metatable, JS because the handle carries no methods.
	if _, err := runJSString(t, fmt.Sprintf(
		`var doc = host.html.parse(%q); host.html.find_text.call(doc, "h1.title");`,
		htmlFixtureMarkup)); err == nil {
		t.Error("a receiver-style call should not resolve in JS")
	}
}

// runJSString runs a chunk that is expected to fail and returns the error.
func runJSString(t *testing.T, chunk string) (goja.Value, error) {
	t.Helper()
	vm := goja.New()
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "calltext"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}
	return vm.RunString(chunk)
}

// TestHostHTMLErrorTextIdentical is 6.2: the same invalid selector and the same
// invalid XPath expression must be reported with the same text by both script
// runtimes. A whole message, not a substring, because "roughly the same" is
// what makes a plugin's error handling differ per runtime.
func TestHostHTMLErrorTextIdentical(t *testing.T) {
	const luaPrefix = `local doc = host.html.parse("<h1>t</h1>")
		local _, err = host.html.%s
		if err == nil then return "NO ERROR" end
		return err`
	const jsPrefix = `var doc = host.html.parse("<h1>t</h1>");
		(function(){ try { host.html.%s } catch (e) { return e.message } return "NO ERROR" })()`

	for _, call := range []string{
		`find_text(doc, "{invalid")`,
		`find_attr(doc, "{invalid", "href")`,
		`find_list_text(doc, "{invalid")`,
		`find_list_attr(doc, "{invalid", "src")`,
		`xpath_text(doc, "[invalid")`,
		`xpath_attr(doc, "[invalid", "href")`,
		`xpath_list_text(doc, "[invalid")`,
		`xpath_list_attr(doc, "[invalid", "src")`,
	} {
		t.Run(call, func(t *testing.T) {
			fromLua := luaString(t, fmt.Sprintf(luaPrefix, call))
			fromJS := jsString(t, fmt.Sprintf(jsPrefix, call))
			if fromLua != fromJS {
				t.Fatalf("error text diverges:\n lua = %q\n  js = %q", fromLua, fromJS)
			}
		})
	}
}

// TestHostHTMLYaegiFixtureSharesMarkup keeps the Yaegi fixture's copy of the
// fixture markup honest, since the interpreter cannot import a test constant.
func TestHostHTMLYaegiFixtureSharesMarkup(t *testing.T) {
	src, err := os.ReadFile("testdata/yaegihtml/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), htmlFixtureMarkup) {
		t.Fatal("testdata/yaegihtml/main.go no longer carries htmlFixtureMarkup verbatim")
	}
}

// TestHostHTMLYaegiMatchesScriptRuntimes is 6.4: the Yaegi bridge reaches the
// same implementation, so it must return the same values and the same error
// text as the two script runtimes.
func TestHostHTMLYaegiMatchesScriptRuntimes(t *testing.T) {
	m := loadYaegiFixture(t, "yaegihtml")

	out, err := callYaegi(m, m.plugins["yaegihtml"], "Search", `"x"`)
	if err != nil {
		t.Fatalf("Search through the Yaegi bridge: %v", err)
	}
	yaegiGot := decodeLookups(t, "yaegi", out)

	if !sameLookups(yaegiGot, htmlLookupsExpected) {
		t.Errorf("yaegi lookups = %+v, want %+v", yaegiGot, htmlLookupsExpected)
	}
	if !sameLookups(yaegiGot, luaHTMLLookups(t)) || !sameLookups(yaegiGot, jsHTMLLookups(t)) {
		t.Errorf("yaegi disagrees with the script runtimes: %+v", yaegiGot)
	}

	// The error path has to travel the same distance: the bridge wraps a Go
	// error, and the text a plugin sees must still name the offending value.
	out, err = callYaegi(m, m.plugins["yaegihtml"], "Search", `"bogus"`)
	if err != nil {
		t.Fatalf("Search through the Yaegi bridge (error path): %v", err)
	}
	var got struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("yaegi error payload is not JSON: %v", err)
	}
	if !strings.Contains(got.Error, "invalid CSS selector") {
		t.Fatalf("yaegi error does not name the failure: %q", got.Error)
	}
}

// TestYaegiHTMLDemoExampleLoads is 5.2: the Yaegi example the docs point
// plugin authors at has to keep loading (its body is compiled by the
// interpreter, so a misspelled or unexported hostnet symbol is a load error)
// and has to return real values rather than empty results.
func TestYaegiHTMLDemoExampleLoads(t *testing.T) {
	m := NewManager(hostnet.NewProxy(), "testdata")
	p, err := m.loadYaegi("yaegidemo", "../../examples/plugins/yaegi/yaegidemo")
	if err != nil {
		t.Fatalf("the shipped Yaegi example no longer loads: %v", err)
	}
	m.plugins["yaegidemo"] = p

	const markup = `<html><body><h1>  Solo Leveling  </h1><img src="/a.jpg"><img src="/b.jpg"></body></html>`
	// ExampleHTMLDemo is not an ABI export, so drive it through the
	// interpreter directly; Eval of the call yields its first return value.
	got, err := p.yaegi.i.Eval(fmt.Sprintf("ExampleHTMLDemo(%q)", markup))
	if err != nil {
		t.Fatalf("ExampleHTMLDemo: %v", err)
	}
	text, _ := got.Interface().(string)
	if !strings.Contains(text, `"title":"Solo Leveling"`) {
		t.Errorf("ExampleHTMLDemo title = %s", text)
	}
	if !strings.Contains(text, `"/a.jpg"`) || !strings.Contains(text, `"/b.jpg"`) {
		t.Errorf("ExampleHTMLDemo images = %s", text)
	}
}

// TestLuaHTMLScrapeExample runs the shipping example plugin
// examples/plugins/lua/htmlscrape against a stub site, so the example is a
// plugin that has actually been executed rather than a snippet in a doc.
func TestLuaHTMLScrapeExample(t *testing.T) {
	const searchPage = `<!doctype html><html><body>
		<a class="card" href="/manga/solo"><span class="title">Solo Leveling</span><img src="/c1.jpg"></a>
		<a class="card" href="/manga/tomb"><span class="title">Tomb Raider</span><img src="/c2.jpg"></a>
		</body></html>`
	const detailPage = `<!doctype html><html><body>
		<h1 class="title">   Solo Leveling   </h1>
		<a class="author" href="/author/chugong">Chugong</a>
		<img class="cover" src="/cover.jpg">
		<span class="status" data-status="ongoing">Ongoing</span>
		<div class="summary">A weak hunter becomes strong.</div>
		<a class="genre" href="/genre/action">Action</a>
		<a class="genre" href="/genre/fantasy">Fantasy</a>
		<ul class="chapters">
			<li><a href="/chapter/1">Chapter 1</a></li>
			<li><a href="/chapter/2">Chapter 2</a></li>
		</ul>
		</body></html>`
	const readerPage = `<!doctype html><html><body>
		<img class="page" data-src="/p/1.jpg">
		<img class="page" data-src="/p/2.jpg">
		<img class="banner" data-src="/ad.gif">
		</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch {
		case strings.HasPrefix(r.URL.Path, "/search"):
			_, _ = io.WriteString(w, searchPage)
		case strings.HasPrefix(r.URL.Path, "/chapter/"):
			_, _ = io.WriteString(w, readerPage)
		case strings.HasPrefix(r.URL.Path, "/manga/"):
			_, _ = io.WriteString(w, detailPage)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	defer func() { _ = mgr.Close() }()
	if _, err := mgr.Install("../../examples/plugins/lua/htmlscrape"); err != nil {
		t.Fatalf("install example plugin: %v", err)
	}

	detail, err := mgr.GetMangaDetail("htmlscrape", srv.URL+"/manga/solo")
	if err != nil {
		t.Fatalf("GetMangaDetail: %v", err)
	}
	if detail.Title != "Solo Leveling" {
		t.Errorf("title = %q, want the trimmed heading", detail.Title)
	}
	if detail.CoverURL != "/cover.jpg" {
		t.Errorf("cover_url = %q, want /cover.jpg", detail.CoverURL)
	}
	if detail.Author != "/author/chugong" {
		t.Errorf("author = %q", detail.Author)
	}
	if detail.Status != "Ongoing" {
		t.Errorf("status = %q, want the host's canonical vocabulary", detail.Status)
	}
	if len(detail.Genres) != 2 {
		t.Errorf("genres = %v, want two", detail.Genres)
	}

	chapters, err := mgr.GetChapterList("htmlscrape", srv.URL+"/manga/solo")
	if err != nil {
		t.Fatalf("GetChapterList: %v", err)
	}
	if len(chapters) != 2 || chapters[1].ChapterNum != 2 || chapters[0].URL != "/chapter/1" {
		t.Errorf("chapters = %+v, want the two list rows in order", chapters)
	}

	pages, err := mgr.GetPageList("htmlscrape", srv.URL+"/chapter/1")
	if err != nil {
		t.Fatalf("GetPageList: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("pages = %+v, want 2 (the banner img must not match img.page)", pages)
	}
	if pages[0].URL != "/p/1.jpg" || pages[1].URL != "/p/2.jpg" {
		t.Errorf("page urls = %q, %q", pages[0].URL, pages[1].URL)
	}
	if pages[0].Index != 1 || pages[1].Index != 2 {
		t.Errorf("page indexes = %d, %d, want 1-based", pages[0].Index, pages[1].Index)
	}

	// Search builds its URL from the plugin's own BASE, so the stub server
	// cannot answer it. That still exercises the example's failure path: it must
	// log and return an empty list rather than error.
	results, err := mgr.Search("htmlscrape", types.SearchFilter{Query: "solo"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("search = %+v, want no results from an unreachable BASE", results)
	}
}

// liveScrapeURL is a real manga page the live check scrapes. It is a home feed
// rather than a chapter reader so the check does not depend on a chapter still
// existing, and it serves server-rendered <img src> markup (sites that render
// their catalogue from a JSON payload have no images in the HTML to read).
const liveScrapeURL = "https://weebcentral.com/"

// TestLuaHTMLScrapeLivePage is the live half of the verification: a real page,
// fetched through the host proxy by the Lua runtime, read with the same lookups
// a plugin uses. Skipped unless GOISEKAI_LIVE=1, because it needs the network.
func TestLuaHTMLScrapeLivePage(t *testing.T) {
	if os.Getenv("GOISEKAI_LIVE") == "" {
		t.Skip("set GOISEKAI_LIVE=1 to scrape a real page")
	}

	encoded := luaString(t, fmt.Sprintf(`
		local resp = host.http.get(%q, {["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"})
		if not resp or resp.status ~= 200 then
			return "status " .. tostring(resp and resp.status)
		end
		local doc = host.html.parse(resp.body)
		local _, bad = host.html.xpath_text(doc, "///")
		return host.json.encode({
			title = host.html.find_text(doc, "title"),
			images = #host.html.find_list_attr(doc, "img", "src"),
			links = #host.html.xpath_list_attr(doc, "//a", "href"),
			bad_selector = bad or "none",
		})
	`, liveScrapeURL))

	var got struct {
		Title       string `json:"title"`
		Images      int    `json:"images"`
		Links       int    `json:"links"`
		BadSelector string `json:"bad_selector"`
	}
	if err := json.Unmarshal([]byte(encoded), &got); err != nil {
		t.Fatalf("live scrape did not encode as JSON: %v (%s)", err, encoded)
	}
	if got.Title == "" {
		t.Error("read no title from a real page")
	}
	if got.Images == 0 || got.Links == 0 {
		t.Errorf("read an empty list from a real page: %d images, %d links", got.Images, got.Links)
	}
	if !strings.Contains(got.BadSelector, "invalid XPath expression") {
		t.Errorf("a wrong expression should report a parse error, got %q", got.BadSelector)
	}
	t.Logf("live: title=%q images=%d links=%d", got.Title, got.Images, got.Links)
}
