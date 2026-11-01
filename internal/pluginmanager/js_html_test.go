package pluginmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"modernc.org/quickjs"

	"goisekai/internal/hostnet"
)

// runJS runs a chunk on a fresh VM with the host object installed, the way a
// plugin gets one at load time, and returns the chunk's value.
func runJS(t *testing.T, chunk string) any {
	t.Helper()
	vm, err := quickjs.NewVM()
	if err != nil {
		t.Fatalf("NewVM: %v", err)
	}
	t.Cleanup(func() { _ = vm.Close() })

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "test"); err != nil {
		t.Fatalf("register js natives: %v", err)
	}
	val, err := vm.Eval(chunk, quickjs.EvalGlobal)
	if err != nil {
		t.Fatalf("run chunk: %v", err)
	}
	return val
}

// jsString renders a quickjs Eval result the way JS would print it.
func jsValString(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case *quickjs.Object:
		return t.String()
	case quickjs.Undefined:
		return "undefined"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// jsThrownMessage runs a chunk that is expected to throw and returns the
// message, so error paths read like the Lua ones.
func jsThrownMessage(body string) string {
	return `
		var err;
		try { ` + body + ` } catch (e) { err = e.message; }
		err;
	`
}

func wantJSString(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func wantJSLength(t *testing.T, val any, want int) {
	t.Helper()
	n, ok := val.(int)
	if !ok {
		t.Fatalf("expected a number, got %v", val)
	}
	if n != want {
		t.Fatalf("expected %d, got %d", want, n)
	}
}

func TestJSHTMLParseAndFindText(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<h1 class='t'>   Solo Leveling   </h1>");
		host.html.find_text(doc, "h1.t");
	`)
	wantJSString(t, jsValString(val), "Solo Leveling")
}

func TestJSHTMLFindAttr(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<a class='c' href='/chapter/1'>Ch 1</a>");
		host.html.find_attr(doc, "a.c", "href");
	`)
	wantJSString(t, jsValString(val), "/chapter/1")
}

func TestJSHTMLFindListText(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<a>Ch1</a><a>Ch2</a><a>Ch3</a>");
		host.html.find_list_text(doc, "a").length;
	`)
	wantJSLength(t, val, 3)
}

func TestJSHTMLFindListAttr(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<img src='1.jpg'><img alt='x'><img src='2.jpg'>");
		host.html.find_list_attr(doc, "img", "src").length;
	`)
	wantJSLength(t, val, 2)
}

func TestJSHTMLXPathText(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<div><h1 class='t'>Test</h1></div>");
		host.html.xpath_text(doc, "//h1[@class='t']");
	`)
	wantJSString(t, jsValString(val), "Test")
}

func TestJSHTMLXPathAttr(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<div><img src='1.jpg'></div>");
		host.html.xpath_attr(doc, "//img", "src");
	`)
	wantJSString(t, jsValString(val), "1.jpg")
}

func TestJSHTMLXPathListText(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<a>Ch1</a><a>Ch2</a>");
		host.html.xpath_list_text(doc, "//a").length;
	`)
	wantJSLength(t, val, 2)
}

func TestJSHTMLXPathListAttr(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<img src='1.jpg'><img alt='x'><img src='2.jpg'>");
		host.html.xpath_list_attr(doc, "//img", "src").length;
	`)
	wantJSLength(t, val, 2)
}

func TestJSHTMLInvalidSelector(t *testing.T) {
	val := runJS(t, jsThrownMessage(`
		var doc = host.html.parse("<h1>Test</h1>");
		host.html.find_text(doc, "{invalid");
	`))
	if !strings.Contains(jsValString(val), "invalid CSS selector") {
		t.Fatalf("expected invalid CSS selector error, got %q", jsValString(val))
	}
}

func TestJSHTMLInvalidXPathExpression(t *testing.T) {
	val := runJS(t, jsThrownMessage(`
		var doc = host.html.parse("<h1>Test</h1>");
		host.html.xpath_text(doc, "[invalid");
	`))
	if !strings.Contains(jsValString(val), "invalid XPath expression") {
		t.Fatalf("expected invalid XPath error, got %q", jsValString(val))
	}
}

func TestJSHTMLWrongArgumentType(t *testing.T) {
	val := runJS(t, jsThrownMessage(`host.html.find_text("not a handle", "div");`))
	if !strings.Contains(jsValString(val), "argument 0 must be a document handle") {
		t.Fatalf("expected handle type error, got %q", jsValString(val))
	}
	val = runJS(t, jsThrownMessage(`host.html.find_text({}, "div");`))
	if !strings.Contains(jsValString(val), "argument 0 must be a document handle") {
		t.Fatalf("expected a plain object to be rejected, got %q", jsValString(val))
	}
}

func TestJSHTMLParseNonString(t *testing.T) {
	val := runJS(t, jsThrownMessage(`host.html.parse(42);`))
	if !strings.Contains(jsValString(val), "must be a string") {
		t.Fatalf("expected string argument error, got %q", jsValString(val))
	}
}

func TestJSHTMLGroupNotBareGlobal(t *testing.T) {
	val := runJS(t, `
		if (typeof html !== 'undefined') {
			throw new Error("html global should not exist");
		}
		host.html.parse("<h1>Test</h1>") !== null;
	`)
	if b, ok := val.(bool); !ok || !b {
		t.Fatalf("expected true, got %v", val)
	}
}

// TestJSHTMLHandleMethodSetNotReachable guards the reason the handle is an
// opaque value: handing the runtime a Go pointer would expose the Go method
// set as JS properties and reintroduce doc.find_text(sel).
func TestJSHTMLHandleMethodSetNotReachable(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<h1>Test</h1>");
		var leaked = typeof doc.find_text !== 'undefined' || typeof doc.FindText !== 'undefined';
		leaked;
	`)
	if b, ok := val.(bool); !ok || b {
		t.Fatal("Go method set should not be reachable from JS")
	}
}

// htmlRegistryCount reports how many VMs currently hold an html registry.
func htmlRegistryCount() int {
	htmlRegistriesMu.Lock()
	defer htmlRegistriesMu.Unlock()
	return len(htmlRegistries)
}

// TestJSHTMLRegistryReleasedOnClose checks that closing a JS plugin drops its
// html registry and handles, and that a second close does not panic.
func TestJSHTMLRegistryReleasedOnClose(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "htmlleak")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mainJS := `
		function search(input) { return "[]"; }
		function getMangaDetail(input) { return "{}"; }
		function getChapterList(input) { return "[]"; }
		function getPageList(input) { return "[]"; }
		var doc = host.html.parse("<h1 class='t'>Leak</h1>");
	`
	if err := os.WriteFile(filepath.Join(pluginDir, "main.js"), []byte(mainJS), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"name":"HTML leak","contract_version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	before := htmlRegistryCount()

	p, err := mgr.loadJS("htmlleak", pluginDir)
	if err != nil {
		t.Fatalf("loadJS: %v", err)
	}
	if got := htmlRegistryCount(); got != before+1 {
		t.Fatalf("after load: registries = %d, want %d", got, before+1)
	}

	htmlRegistriesMu.Lock()
	reg := htmlRegistries[p.js]
	htmlRegistriesMu.Unlock()
	reg.mu.Lock()
	handles := len(reg.docs)
	reg.mu.Unlock()
	if handles != 1 {
		t.Fatalf("expected 1 html handle from load-time parse, got %d", handles)
	}

	e := jsEngine{}
	if err := e.close(mgr, p); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if got := htmlRegistryCount(); got != before {
		t.Fatalf("after close: registries = %d, want %d", got, before)
	}
	if err := e.close(mgr, p); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if got := htmlRegistryCount(); got != before {
		t.Fatalf("after second close: registries = %d, want %d", got, before)
	}
}
