package templates

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"goisekai/internal/logger"

	lua "github.com/mmcdole/lunar"
)

// countingFS wraps an fs.FS and tallies reads, so a test can assert that
// steady-state rendering never touches the template tree.
type countingFS struct {
	fs.FS
	mu     sync.Mutex
	counts map[string]int
}

func newCountingFS(inner fs.FS) *countingFS {
	return &countingFS{FS: inner, counts: map[string]int{}}
}

func (c *countingFS) Open(name string) (fs.File, error) {
	c.mu.Lock()
	c.counts[name]++
	c.mu.Unlock()
	return c.FS.Open(name)
}

func (c *countingFS) snapshot() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int, len(c.counts))
	for k, v := range c.counts {
		out[k] = v
	}
	return out
}

// reset zeroes the tallies so a test can measure one phase only (e.g. renders
// after the startup compile walk).
func (c *countingFS) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts = map[string]int{}
}

func (c *countingFS) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.counts {
		n += v
	}
	return n
}

// realTemplateFS points at the repository's actual template tree.
func realTemplateFS(t *testing.T) fs.FS {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "internal", "templates")
	if _, err := os.Stat(filepath.Join(dir, "views", "library.lua")); err != nil {
		t.Skipf("template tree not available: %v", err)
	}
	return os.DirFS(dir)
}

// views/library requires four partials, so before the source cache existed this
// cost four file reads per render on top of the cached prototype.
const readHeavyView = "views/library"

// Task 1.3: with hot reload off, rendering must not read any template file.
// This is the guard for the regression that motivated the source cache — a
// cache-only implementation still re-read every required partial through
// lunar's FSLoader.
func TestCacheModeRenderReadsNoTemplateFiles(t *testing.T) {
	cfs := newCountingFS(realTemplateFS(t))
	eng, err := NewLuaEngine(cfs, false)
	if err != nil {
		t.Fatalf("NewLuaEngine: %v", err)
	}
	// Startup compiles every template once; only renders are under test.
	cfs.reset()

	for range 5 {
		var out strings.Builder
		if err := eng.Render(&out, readHeavyView, map[string]any{"mangas": []any{}}); err != nil {
			t.Fatalf("render: %v", err)
		}
		if out.Len() == 0 {
			t.Fatal("render produced no output")
		}
	}

	if n := cfs.total(); n != 0 {
		var parts []string
		for k, v := range cfs.snapshot() {
			parts = append(parts, k+"="+itoa(v))
		}
		t.Errorf("cache-mode renders read %d template files (%s); want 0",
			n, strings.Join(parts, " "))
	}
}

// Task 1.3: the source cache is what makes the above possible — every template
// compiled at startup must have its source available to require.
func TestSourcesCachedForEveryTemplate(t *testing.T) {
	eng, err := NewLuaEngine(realTemplateFS(t), false)
	if err != nil {
		t.Fatalf("NewLuaEngine: %v", err)
	}
	eng.mu.RLock()
	defer eng.mu.RUnlock()
	if len(eng.sources) == 0 {
		t.Fatal("sources cache is empty")
	}
	if len(eng.sources) != len(eng.protos) {
		t.Errorf("sources has %d entries but protos has %d; a required module "+
			"would fall through to disk", len(eng.sources), len(eng.protos))
	}
	for name, src := range eng.sources {
		if !strings.Contains(src, "return function") {
			t.Errorf("cached source for %s does not look like a template module", name)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// modSrc is a minimal template module: lua.Compile needs a chunk that returns a
// function, and render calls it with the data table.
func modSrc(marker string) string {
	return "return function(data) return '" + marker + "' end\n"
}

func modFS(files map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for name, body := range files {
		out[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return out
}

func renderTo(t *testing.T, eng *LuaEngine, name string) string {
	t.Helper()
	var out strings.Builder
	if err := eng.Render(&out, name, map[string]any{}); err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	return out.String()
}

func protoPtr(t *testing.T, eng *LuaEngine, name string) *lua.Prototype {
	t.Helper()
	eng.mu.RLock()
	defer eng.mu.RUnlock()
	return eng.protos[name]
}

// Task 1.5: hot reload recompiles only what changed. The pointer identity check
// matters because recompiling everything would look correct while paying the
// full cost the hash comparison exists to avoid.
func TestHotReloadRecompilesOnlyChangedTemplates(t *testing.T) {
	fsys := modFS(map[string]string{
		"a.lua": modSrc("A1"),
		"b.lua": modSrc("B1"),
		"c.lua": modSrc("C1"),
	})
	eng, err := NewLuaEngine(fsys, true)
	if err != nil {
		t.Fatalf("NewLuaEngine: %v", err)
	}

	if got := renderTo(t, eng, "a"); !strings.Contains(got, "A1") {
		t.Fatalf("render a = %q, want it to contain A1", got)
	}
	a0, b0, c0 := protoPtr(t, eng, "a"), protoPtr(t, eng, "b"), protoPtr(t, eng, "c")

	// An untouched tree must not recompile anything.
	renderTo(t, eng, "b")
	if protoPtr(t, eng, "a") != a0 {
		t.Error("unchanged template a was recompiled")
	}

	// Edit exactly one template.
	fsys["b.lua"] = &fstest.MapFile{Data: []byte(modSrc("B2"))}
	if got := renderTo(t, eng, "a"); !strings.Contains(got, "B2") {
		// a does not depend on b, so this checks the refresh happened at all;
		// the content check below is what proves b itself took effect.
		t.Logf("render a = %q (b not required by a, expected)", got)
	}
	if got := renderTo(t, eng, "b"); !strings.Contains(got, "B2") {
		t.Errorf("render b = %q, want it to contain B2 after the edit", got)
	}

	if protoPtr(t, eng, "b") == b0 {
		t.Error("changed template b kept its old prototype")
	}
	if protoPtr(t, eng, "a") != a0 {
		t.Error("template a was recompiled even though only b changed")
	}
	if protoPtr(t, eng, "c") != c0 {
		t.Error("template c was recompiled even though only b changed")
	}

	// The hash must track the new content, or the next render recompiles again.
	if got, want := eng.hashFor("b"), sha256.Sum256([]byte(modSrc("B2"))); got != want {
		t.Errorf("hash for b = %x, want %x", got, want)
	}
}

// Task 1.6: one bad edit must not take the page down. The previous bytecode
// keeps serving and the failure is reported.
func TestHotReloadKeepsPreviousOnCompileError(t *testing.T) {
	fsys := modFS(map[string]string{
		"a.lua": modSrc("GOOD"),
		"b.lua": modSrc("ALSO_GOOD"),
	})
	eng, err := NewLuaEngine(fsys, true)
	if err != nil {
		t.Fatalf("NewLuaEngine: %v", err)
	}
	goodA := protoPtr(t, eng, "a")
	goodB := protoPtr(t, eng, "b")

	// Init installs the capture handler that backs GetLines; without it slog's
	// default logger writes to stderr and the ring buffer stays empty.
	if err := logger.Init("debug"); err != nil {
		t.Fatalf("logger.Init: %v", err)
	}
	logger.Clear()
	fsys["a.lua"] = &fstest.MapFile{Data: []byte("return function(data) this is not lua")}

	got := renderTo(t, eng, "a")
	if !strings.Contains(got, "GOOD") {
		t.Errorf("render a = %q, want the previous GOOD bytecode to keep serving", got)
	}
	if protoPtr(t, eng, "a") != goodA {
		t.Error("a broken template replaced its previous prototype")
	}
	if protoPtr(t, eng, "b") != goodB {
		t.Error("an unrelated template was disturbed by a's compile failure")
	}

	logged := strings.Join(logger.GetLines(), "\n")
	if !strings.Contains(logged, "hot reload") {
		t.Errorf("compile failure was not logged; log tail:\n%s", logged)
	}
}

// A broken template at startup has no previous entry to fall back to, so it must
// fail loudly rather than silently render nothing.
func TestStartupRejectsBrokenTemplate(t *testing.T) {
	fsys := modFS(map[string]string{"a.lua": modSrc("OK")})
	fsys["bad.lua"] = &fstest.MapFile{Data: []byte("return function(data) nope")}
	if _, err := NewLuaEngine(fsys, false); err == nil {
		t.Fatal("NewLuaEngine accepted a template that does not compile")
	}
}

// Task 1.4: every compiled template carries a content hash, otherwise hot reload
// would recompile the whole tree on every render.
func TestHashesPopulatedForEveryTemplate(t *testing.T) {
	files := map[string]string{"a.lua": modSrc("A"), "b.lua": modSrc("B")}
	eng, err := NewLuaEngine(modFS(files), false)
	if err != nil {
		t.Fatalf("NewLuaEngine: %v", err)
	}
	eng.mu.RLock()
	defer eng.mu.RUnlock()
	for name, src := range files {
		want := sha256.Sum256([]byte(src))
		got, ok := eng.hashes[strings.TrimSuffix(name, ".lua")]
		if !ok {
			t.Errorf("template %s has no recorded hash", name)
			continue
		}
		if got != want {
			t.Errorf("hash for %s = %x, want %x", name, got, want)
		}
	}
	if len(eng.hashes) != len(eng.protos) {
		t.Errorf("hashes has %d entries, protos has %d", len(eng.hashes), len(eng.protos))
	}
}
