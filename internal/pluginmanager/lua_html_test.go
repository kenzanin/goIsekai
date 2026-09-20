package pluginmanager

import (
	"strings"
	"testing"

	lua "github.com/mmcdole/lunar"
	"goisekai/internal/hostnet"
)

// newLuaState builds a Lua state with the host table installed, the way a
// plugin gets one at load time. It goes through createLuaState so a test runs
// against the same sandbox, standard libraries included, that a plugin does.
func newLuaState(t *testing.T) *lua.State {
	t.Helper()
	state, err := createLuaState("test")
	if err != nil {
		t.Fatalf("new state: %v", err)
	}
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := mgr.setupGlobals(state, "test"); err != nil {
		_ = state.Close()
		t.Fatalf("setup globals: %v", err)
	}
	return state
}

// runLua runs a chunk on a fresh state and hands the first return value to
// check while the state is still open.
func runLua(t *testing.T, chunk string, check func(t *testing.T, first lua.Value)) {
	t.Helper()
	state := newLuaState(t)
	defer func() { _ = state.Close() }()

	fn, err := state.Load("test.lua", strings.NewReader(chunk))
	if err != nil {
		t.Fatalf("load chunk: %v", err)
	}
	results, err := state.Call(fn.Value())
	if err != nil {
		t.Fatalf("call chunk: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("chunk returned no values")
	}
	check(t, results[0])
}

func wantLuaString(want string) func(*testing.T, lua.Value) {
	return func(t *testing.T, first lua.Value) {
		t.Helper()
		got, ok := first.AsString()
		if !ok {
			t.Fatalf("expected a string, got %v", first)
		}
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	}
}

func wantLuaNumber(want int) func(*testing.T, lua.Value) {
	return func(t *testing.T, first lua.Value) {
		t.Helper()
		got, ok := first.AsNumber()
		if !ok {
			t.Fatalf("expected a number, got %v", first)
		}
		if int(got) != want {
			t.Fatalf("expected %d, got %v", want, got)
		}
	}
}

func wantLuaErrorContains(substr string) func(*testing.T, lua.Value) {
	return func(t *testing.T, first lua.Value) {
		t.Helper()
		got, ok := first.AsString()
		if !ok {
			t.Fatalf("expected an error string, got %v", first)
		}
		if !strings.Contains(got, substr) {
			t.Fatalf("expected error containing %q, got %q", substr, got)
		}
	}
}

func TestLuaHTMLParseAndFindText(t *testing.T) {
	runLua(t, `
		local doc = host.html.parse("<h1 class='t'>   Solo Leveling   </h1>")
		return host.html.find_text(doc, "h1.t")
	`, wantLuaString("Solo Leveling"))
}

func TestLuaHTMLFindAttr(t *testing.T) {
	runLua(t, `
		local doc = host.html.parse("<a class='c' href='/chapter/1'>Ch 1</a>")
		return host.html.find_attr(doc, "a.c", "href")
	`, wantLuaString("/chapter/1"))
}

func TestLuaHTMLFindListText(t *testing.T) {
	runLua(t, `
		local doc = host.html.parse("<a>Ch1</a><a>Ch2</a><a>Ch3</a>")
		return #host.html.find_list_text(doc, "a")
	`, wantLuaNumber(3))
}

func TestLuaHTMLFindListAttr(t *testing.T) {
	runLua(t, `
		local doc = host.html.parse("<img src='1.jpg'><img alt='x'><img src='2.jpg'>")
		return #host.html.find_list_attr(doc, "img", "src")
	`, wantLuaNumber(2))
}

func TestLuaHTMLXPathText(t *testing.T) {
	runLua(t, `
		local doc = host.html.parse("<div><h1 class='t'>Test</h1></div>")
		return host.html.xpath_text(doc, "//h1[@class='t']")
	`, wantLuaString("Test"))
}

func TestLuaHTMLXPathAttr(t *testing.T) {
	runLua(t, `
		local doc = host.html.parse("<div><img src='1.jpg'></div>")
		return host.html.xpath_attr(doc, "//img", "src")
	`, wantLuaString("1.jpg"))
}

func TestLuaHTMLXPathListText(t *testing.T) {
	runLua(t, `
		local doc = host.html.parse("<a>Ch1</a><a>Ch2</a>")
		return #host.html.xpath_list_text(doc, "//a")
	`, wantLuaNumber(2))
}

func TestLuaHTMLXPathListAttr(t *testing.T) {
	runLua(t, `
		local doc = host.html.parse("<img src='1.jpg'><img alt='x'><img src='2.jpg'>")
		return #host.html.xpath_list_attr(doc, "//img", "src")
	`, wantLuaNumber(2))
}

func TestLuaHTMLInvalidSelector(t *testing.T) {
	runLua(t, `
		local doc = host.html.parse("<h1>Test</h1>")
		local title, err = host.html.find_text(doc, "{invalid")
		if title ~= nil and err == nil then
			return "unexpected success"
		end
		return err
	`, wantLuaErrorContains("invalid CSS selector"))
}

func TestLuaHTMLInvalidXPath(t *testing.T) {
	runLua(t, `
		local doc = host.html.parse("<h1>Test</h1>")
		local title, err = host.html.xpath_text(doc, "[invalid")
		if title ~= nil and err == nil then
			return "unexpected success"
		end
		return err
	`, wantLuaErrorContains("invalid XPath expression"))
}

// TestLuaHTMLWrongArgumentType is the Lua half of 3.3: the handle is a typed
// native userdata, so only a real handle is accepted. A string and a table are
// both rejected by name rather than read as a document.
func TestLuaHTMLWrongArgumentType(t *testing.T) {
	runLua(t, `
		local title, err = host.html.find_text("not a handle", "div")
		if title ~= nil and err == nil then
			return "unexpected success"
		end
		local fromTable, tableErr = host.html.find_text({}, "div")
		if fromTable ~= nil and tableErr == nil then
			return "a table was accepted as a handle"
		end
		return err .. "|" .. tableErr
	`, wantLuaErrorContains("argument 0 must be a document handle"))
}

func TestLuaHTMLParseNonString(t *testing.T) {
	runLua(t, `
		local doc, err = host.html.parse(42)
		if doc ~= nil and err == nil then
			return "unexpected success"
		end
		return err
	`, wantLuaErrorContains("must be a string"))
}

// TestLuaHTMLGroupNotBareGlobal keeps `html` out of the global namespace, so
// the group is the only entry point.
func TestLuaHTMLGroupNotBareGlobal(t *testing.T) {
	runLua(t, `
		if html ~= nil then
			return "html global should not exist"
		end
		return "ok"
	`, wantLuaString("ok"))
}

// TestLuaHTMLGroupLeavesExistingHelpersAlone is 3.4: adding the html group must
// not disturb the groups that were already there.
func TestLuaHTMLGroupLeavesExistingHelpersAlone(t *testing.T) {
	runLua(t, `
		local doc = host.html.parse("<p>Hello <b>world</b></p>")
		local text = host.html.find_text(doc, "p")
		local plain = host.text.strip_html("<p>Hello <b>world</b></p>")
		local decoded = host.json.decode('{"a":[1,2]}')
		return text .. "|" .. plain .. "|" .. #decoded.a
	`, wantLuaString("Hello world|Hello world|2"))
}

// TestLuaHTMLTwoStatesInOneProcess is the regression for the handle descriptor
// having been a package global: a second state in the same process used to
// overwrite it, so the first state's handle belonged to a foreign state and
// host.html.parse panicked.
func TestLuaHTMLTwoStatesInOneProcess(t *testing.T) {
	first := newLuaState(t)
	defer func() { _ = first.Close() }()
	second := newLuaState(t)
	defer func() { _ = second.Close() }()

	chunk := `return host.html.find_text(host.html.parse("<h1>one</h1>"), "h1")`
	for name, state := range map[string]*lua.State{"first": first, "second": second} {
		fn, err := state.Load("test.lua", strings.NewReader(chunk))
		if err != nil {
			t.Fatalf("%s: load chunk: %v", name, err)
		}
		results, err := state.Call(fn.Value())
		if err != nil {
			t.Fatalf("%s: call chunk: %v", name, err)
		}
		got, ok := results[0].AsString()
		if !ok || got != "one" {
			t.Fatalf("%s: expected \"one\", got %v", name, results[0])
		}
	}
}
