package pluginmanager

import (
	"strings"
	"testing"

	"github.com/dop251/goja"
	"goisekai/internal/hostnet"
)

// runJS runs a chunk on a fresh VM with the host object installed, the way a
// plugin gets one at load time, and returns the chunk's value.
func runJS(t *testing.T, chunk string) goja.Value {
	t.Helper()
	vm := goja.New()
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "test"); err != nil {
		t.Fatalf("register js natives: %v", err)
	}
	val, err := vm.RunString(chunk)
	if err != nil {
		t.Fatalf("run chunk: %v", err)
	}
	return val
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

func wantJSLength(t *testing.T, val goja.Value, want int) {
	t.Helper()
	n, ok := val.Export().(int64)
	if !ok {
		t.Fatalf("expected a number, got %v", val.Export())
	}
	if int(n) != want {
		t.Fatalf("expected %d, got %d", want, n)
	}
}

func TestJSHTMLParseAndFindText(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<h1 class='t'>   Solo Leveling   </h1>");
		host.html.find_text(doc, "h1.t");
	`)
	wantJSString(t, val.String(), "Solo Leveling")
}

func TestJSHTMLFindAttr(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<a class='c' href='/chapter/1'>Ch 1</a>");
		host.html.find_attr(doc, "a.c", "href");
	`)
	wantJSString(t, val.String(), "/chapter/1")
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
	wantJSString(t, val.String(), "Test")
}

func TestJSHTMLXPathAttr(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<div><img src='1.jpg'></div>");
		host.html.xpath_attr(doc, "//img", "src");
	`)
	wantJSString(t, val.String(), "1.jpg")
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
	if !strings.Contains(val.String(), "invalid CSS selector") {
		t.Fatalf("expected invalid CSS selector error, got %q", val.String())
	}
}

func TestJSHTMLInvalidXPathExpression(t *testing.T) {
	val := runJS(t, jsThrownMessage(`
		var doc = host.html.parse("<h1>Test</h1>");
		host.html.xpath_text(doc, "[invalid");
	`))
	if !strings.Contains(val.String(), "invalid XPath expression") {
		t.Fatalf("expected invalid XPath error, got %q", val.String())
	}
}

func TestJSHTMLWrongArgumentType(t *testing.T) {
	val := runJS(t, jsThrownMessage(`host.html.find_text("not a handle", "div");`))
	if !strings.Contains(val.String(), "argument 0 must be a document handle") {
		t.Fatalf("expected handle type error, got %q", val.String())
	}
	val = runJS(t, jsThrownMessage(`host.html.find_text({}, "div");`))
	if !strings.Contains(val.String(), "argument 0 must be a document handle") {
		t.Fatalf("expected a plain object to be rejected, got %q", val.String())
	}
}

func TestJSHTMLParseNonString(t *testing.T) {
	val := runJS(t, jsThrownMessage(`host.html.parse(42);`))
	if !strings.Contains(val.String(), "must be a string") {
		t.Fatalf("expected string argument error, got %q", val.String())
	}
}

func TestJSHTMLGroupNotBareGlobal(t *testing.T) {
	val := runJS(t, `
		if (typeof html !== 'undefined') {
			throw new Error("html global should not exist");
		}
		host.html.parse("<h1>Test</h1>") !== null;
	`)
	if !val.ToBoolean() {
		t.Fatalf("expected true, got %v", val.Export())
	}
}

// TestJSHTMLHandleMethodSetNotReachable guards the reason the handle is a
// JS-side wrapper: handing goja the Go pointer would expose the Go method set
// as JS properties and reintroduce doc.find_text(sel).
func TestJSHTMLHandleMethodSetNotReachable(t *testing.T) {
	val := runJS(t, `
		var doc = host.html.parse("<h1>Test</h1>");
		var leaked = typeof doc.find_text !== 'undefined' || typeof doc.FindText !== 'undefined';
		leaked;
	`)
	if val.ToBoolean() {
		t.Fatal("Go method set should not be reachable from JS")
	}
}
