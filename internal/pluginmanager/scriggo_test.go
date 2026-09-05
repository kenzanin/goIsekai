package pluginmanager

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// scriggoPluginsDir copies the scriggo fixture into a temp plugins dir.
func scriggoPluginsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, "scriggotest")
	if err := copyDir("testdata/scriggotest", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dir
}

func TestScriggoDiscoveryAndSearch(t *testing.T) {
	dir := scriggoPluginsDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// After Discover the plugin is registered but NOT yet loaded (lazy).
	p := mgr.plugins["scriggotest"]
	if p.kind != "scriggo" {
		t.Fatalf("kind = %q, want scriggo", p.kind)
	}
	if p.loaded {
		t.Fatal("plugin should not be loaded right after Discover")
	}

	// Search triggers lazy load + invocation.
	mangas, err := mgr.Search("scriggotest", types.SearchFilter{Query: "test"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(mangas) != 1 || mangas[0].ID != "S1" || mangas[0].Title != "Scriggo test" {
		t.Fatalf("unexpected Search result: %+v", mangas)
	}

	detail, err := mgr.GetMangaDetail("scriggotest", "m1")
	if err != nil {
		t.Fatalf("GetMangaDetail: %v", err)
	}
	if detail.Title != "Detail m1" {
		t.Fatalf("unexpected detail: %+v", detail)
	}

	chapters, err := mgr.GetChapterList("scriggotest", "m1")
	if err != nil {
		t.Fatalf("GetChapterList: %v", err)
	}
	if len(chapters) != 1 || chapters[0].ChapterNum != 1.0 || chapters[0].MangaID != "m1" {
		t.Fatalf("unexpected chapters: %+v", chapters)
	}

	pages, err := mgr.GetPageList("scriggotest", "m1:c1")
	if err != nil {
		t.Fatalf("GetPageList: %v", err)
	}
	if len(pages) != 1 || pages[0].URL != "https://example.com/img/1.png" {
		t.Fatalf("unexpected pages: %+v", pages)
	}
}

func TestScriggoMissingRequiredFunction(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "scriggobad")
	if err := copyDir("testdata/scriggobad", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	_, err := mgr.Search("scriggobad", types.SearchFilter{Query: "x"})
	if err == nil {
		t.Fatal("Search must fail when a required function is missing")
	}
	if !strings.Contains(err.Error(), "scriggobad") {
		t.Errorf("error should contain plugin id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "GetPageList") {
		t.Errorf("error should name the missing function, got: %v", err)
	}
}

func TestScriggoSandboxBlocksOS(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "scriggosandbox")
	if err := copyDir("testdata/scriggosandbox", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	_, err := mgr.Search("scriggosandbox", types.SearchFilter{Query: "x"})
	if err == nil {
		t.Fatal("Search must fail when plugin imports an unregistered package")
	}
	if !strings.Contains(err.Error(), "scriggosandbox") {
		t.Errorf("error should contain plugin id, got: %v", err)
	}
}

func TestScriggoInfiniteLoopTimesOut(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "scriggotimeout")
	if err := copyDir("testdata/scriggotimeout", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	done := make(chan error, 1)
	go func() {
		_, err := mgr.Search("scriggotimeout", types.SearchFilter{Query: "x"})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("infinite loop must time out with an error")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Errorf("error should contain 'timed out', got: %v", err)
		}
	case <-time.After(invokeTimeout + 15*time.Second):
		t.Fatal("search did not finish within invokeTimeout + margin")
	}
}

func TestScriggoPanicIsolation(t *testing.T) {
	// Put both a panicking plugin and a healthy one in the same plugins dir.
	dir := t.TempDir()
	for _, id := range []string{"scriggopanic", "scriggotest"} {
		dst := filepath.Join(dir, id)
		if err := copyDir("testdata/"+id, dst); err != nil {
			t.Fatalf("copy fixture %s: %v", id, err)
		}
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// Calling the panicking plugin must return an error, not crash the host.
	_, err := mgr.Search("scriggopanic", types.SearchFilter{Query: "x"})
	if err == nil {
		t.Fatal("panicking plugin must return an error")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Errorf("error should mention panic, got: %v", err)
	}
	if !strings.Contains(err.Error(), "scriggo panic test") {
		t.Errorf("error should contain panic message, got: %v", err)
	}

	// Host must survive: a healthy plugin still works.
	mangas, err := mgr.Search("scriggotest", types.SearchFilter{Query: "test"})
	if err != nil {
		t.Fatalf("healthy plugin Search failed after panic: %v", err)
	}
	if len(mangas) != 1 || mangas[0].ID != "S1" {
		t.Fatalf("unexpected Search result: %+v", mangas)
	}
}

func TestScriggoHTTPGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello from test")
	}))
	defer srv.Close()

	dir := t.TempDir()
	dst := filepath.Join(dir, "scriggohttp")
	if err := copyDir("testdata/scriggohttp", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// The fixture's Search passes its arg string straight to hostnet.Get
	// and returns the raw body as a JSON string. We call through the
	// internal call path with the raw server URL as input, since mgr.Search
	// would wrap the URL in a SearchFilter JSON object that isn't a valid URL.
	p := mgr.plugins["scriggohttp"]
	out, err := mgr.call(p, types.SearchFunc, srv.URL)
	if err != nil {
		t.Fatalf("call Search: %v", err)
	}
	if !strings.Contains(out, "hello from test") {
		t.Errorf("expected output to contain 'hello from test', got: %s", out)
	}
}
