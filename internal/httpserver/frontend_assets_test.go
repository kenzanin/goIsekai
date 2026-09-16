package httpserver

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// The detail page wires every enrichment chip (alt title, alt synopsis,
// category, related) with an inline handler that calls a helper exported by
// the frontend bundle. Because the bundle is one IIFE, a single syntax error
// stops the whole file from running: every helper becomes undefined and every
// chip silently stops responding, with no server-side error to notice. That
// exact failure shipped for three commits. These tests execute the real bundle
// and cross-check it against the templates.

const (
	frontendLibDir = "../../cmd/goisekai/frontend/lib"
	templatesDir   = "../templates"
)

// frontendGlobals are the helpers templates and reader code call by name. A
// missing one means clicks silently do nothing.
var frontendGlobals = []string{
	"submitForm",
	"showToast",
	"toggleGenreTags",
	"setLoading",
	"syncEnrichmentPanel",
	"syncViewMode",
}

// nodePath returns the node binary or skips the test.
func nodePath(t *testing.T) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; skipping frontend bundle checks")
	}
	return node
}

// jsStubHarness loads the bundle into a stubbed browser and prints the type of
// each requested global as JSON. Running the file (not just parsing it) is the
// point: it catches a throw at load time as well as a syntax error.
const jsStubHarness = `
global.window = global;
const noop = () => {};
const mkEl = () => ({
  style: {}, dataset: {},
  classList: { add: noop, remove: noop, toggle: noop, contains: () => false },
  appendChild: noop, setAttribute: noop, remove: noop,
  querySelector: () => null, querySelectorAll: () => [],
});
global.document = {
  createElement: mkEl,
  head: { appendChild: noop },
  documentElement: { appendChild: noop },
  body: { appendChild: noop },
  addEventListener: noop,
  getElementById: () => null,
  querySelector: () => null,
  querySelectorAll: () => [],
};
global.localStorage = { getItem: () => null, setItem: noop, removeItem: noop };
global.Alpine = { store: () => null, initTree: noop, data: noop, plugin: noop };
global.addEventListener = noop;
global.fetch = () => Promise.resolve({ ok: true, text: () => Promise.resolve('') });
global.history = { replaceState: noop, state: null };
global.location = { origin: 'http://localhost', href: 'http://localhost', pathname: '/' };
global.MutationObserver = class { observe() {} disconnect() {} };
global.requestAnimationFrame = noop;

require(process.env.GOISEKAI_BUNDLE);

const names = (process.env.GOISEKAI_NAMES || '').split(',').filter(Boolean);
const out = {};
for (const n of names) out[n] = typeof global[n];
console.log(JSON.stringify(out));
`

func frontendBundles(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(frontendLibDir, "*.js"))
	if err != nil {
		t.Fatalf("glob frontend js: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no frontend bundles found under %s", frontendLibDir)
	}
	sort.Strings(matches)
	return matches
}

// TestFrontendJSParses runs `node --check` on every served bundle.
func TestFrontendJSParses(t *testing.T) {
	node := nodePath(t)
	for _, bundle := range frontendBundles(t) {
		t.Run(filepath.Base(bundle), func(t *testing.T) {
			out, err := exec.Command(node, "--check", bundle).CombinedOutput()
			if err != nil {
				t.Fatalf("%s does not parse — the whole file stops executing and every "+
					"inline handler it exports becomes undefined:\n%s", bundle, out)
			}
		})
	}
}

// TestFrontendBundleDefinesGlobals executes the bundle and asserts the helpers
// templates rely on exist at runtime.
func TestFrontendBundleDefinesGlobals(t *testing.T) {
	node := nodePath(t)
	bundle, err := filepath.Abs(filepath.Join(frontendLibDir, "alpine-components.js"))
	if err != nil {
		t.Fatal(err)
	}

	harness := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(harness, []byte(jsStubHarness), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}

	cmd := exec.Command(node, harness)
	cmd.Env = append(os.Environ(),
		"GOISEKAI_BUNDLE="+bundle,
		"GOISEKAI_NAMES="+strings.Join(frontendGlobals, ","),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("loading %s threw — a syntax error or a throw at load time kills "+
			"every handler at once:\n%s", filepath.Base(bundle), out)
	}

	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("harness output %q is not JSON: %v", out, err)
	}
	for _, name := range frontendGlobals {
		if got[name] != "function" {
			t.Errorf("window.%s is %s, want function — templates call it from inline handlers, "+
				"so clicks would silently do nothing", name, got[name])
		}
	}
}

// leadingCall matches the first call in an inline handler attribute value:
// onclick="submitForm(..." or @click="toggleGenreTags(...".
var leadingCall = regexp.MustCompile(`(?:@[a-zA-Z.\-]+|on(?:click|change|submit|input|blur|focus))="\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)

// chainedCall matches a call that follows a ";" in the same handler, e.g.
// onclick="event.preventDefault();submitForm(this.closest('form'))".
var chainedCall = regexp.MustCompile(`;\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)

// windowExport matches a global assignment in the bundle.
var windowExport = regexp.MustCompile(`window\.([A-Za-z_$][A-Za-z0-9_$]*)\s*=`)

// scriptFunction matches a helper declared in a template's own <script> block.
var scriptFunction = regexp.MustCompile(`function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)

// browserBuiltins are handler calls the browser provides, plus the Alpine magic
// objects, which are resolved by Alpine rather than by a global.
//
// ponytail: manual allowlist. Add a name here when a template newly leans on a
// browser or Alpine global; only a real JS parser would remove the upkeep.
var browserBuiltins = map[string]bool{
	"event": true, "window": true, "document": true, "alert": true,
	"confirm": true, "console": true, "history": true, "location": true,
	"fetch": true, "setInterval": true, "clearInterval": true,
	"setTimeout": true, "clearTimeout": true, "requestAnimationFrame": true,
	"cancelAnimationFrame": true, "queueMicrotask": true, "$": true, "Alpine": true,
}

// jsKeywords are language constructs the extractor can catch mid-handler, e.g.
// the "for" and "if" in @click="if (x) { for (...) submitForm(f) }". They are
// never callable helpers, so they must not be reported as undefined.
var jsKeywords = map[string]bool{
	"if": true, "else": true, "for": true, "while": true, "do": true,
	"switch": true, "case": true, "default": true, "return": true, "typeof": true,
	"instanceof": true, "in": true, "of": true, "new": true, "delete": true,
	"void": true, "await": true, "async": true, "try": true, "catch": true,
	"finally": true, "throw": true, "break": true, "continue": true,
	"function": true, "class": true, "var": true, "let": true, "const": true,
	"this": true, "super": true, "yield": true, "with": true, "debugger": true,
}

func scanTemplates(t *testing.T) map[string][]string {
	t.Helper()
	files := map[string][]string{}
	err := filepath.WalkDir(templatesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".lua") {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		files[path] = []string{string(data)}
		return nil
	})
	if err != nil {
		t.Fatalf("walk templates: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no templates found under %s", templatesDir)
	}
	return files
}

// templateHandlerCalls returns, per file, the helper names the file invokes
// from an inline event handler.
func templateHandlerCalls(files map[string][]string) map[string][]string {
	calls := map[string][]string{}
	for path, contents := range files {
		re := regexp.MustCompile(leadingCall.String() + "|" + chainedCall.String())
		for _, loc := range re.FindAllStringSubmatchIndex(contents[0], -1) {
			for i := 2; i+1 < len(loc); i += 2 {
				if loc[i] < 0 {
					continue
				}
				name := contents[0][loc[i]:loc[i+1]]
				calls[name] = append(calls[name], filepath.Base(path))
			}
		}
	}
	return calls
}

// TestFrontendTemplateHandlersAreResolvable fails when a template invokes a
// helper from an inline handler that neither the bundle exports nor a template
// script defines. A rename on either side breaks the control with no error.
func TestFrontendTemplateHandlersAreResolvable(t *testing.T) {
	files := scanTemplates(t)

	bundle, err := os.ReadFile(filepath.Join(frontendLibDir, "alpine-components.js"))
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	defined := map[string]bool{}
	for _, m := range windowExport.FindAllStringSubmatch(string(bundle), -1) {
		defined[m[1]] = true
	}
	for _, contents := range files {
		for _, m := range scriptFunction.FindAllStringSubmatch(contents[0], -1) {
			defined[m[1]] = true
		}
	}

	calls := templateHandlerCalls(files)
	if len(calls) == 0 {
		t.Fatal("found no inline handler calls in templates — the extractor is broken")
	}
	for name, sites := range calls {
		if defined[name] || browserBuiltins[name] || jsKeywords[name] {
			continue
		}
		sort.Strings(sites)
		t.Errorf("templates call %s() from inline handlers (%s) but neither the frontend "+
			"bundle nor a template <script> defines it — the control would silently do nothing",
			name, strings.Join(sites, ", "))
	}
}

// TestEnrichmentChipsCallSubmitForm pins the mechanism the chips depend on: the
// clickable spans delegate to submitForm, which posts the surrounding form.
// Paired with TestFrontendBundleDefinesGlobals, this covers both a chip that
// lost its handler and a helper that stopped being exported.
func TestEnrichmentChipsCallSubmitForm(t *testing.T) {
	calls := templateHandlerCalls(scanTemplates(t))
	if len(calls["submitForm"]) == 0 {
		t.Fatal("no template uses submitForm() — enrichment chips would not be clickable")
	}
	if !contains(calls["submitForm"], "detail_alt.lua") {
		t.Errorf("detail_alt.lua does not use submitForm(); callers: %v", calls["submitForm"])
	}
	if !contains(frontendGlobals, "submitForm") {
		t.Error("submitForm is not in frontendGlobals, so nothing asserts the bundle exports it")
	}
}

// spaContentRegex pulls the inline <main id="content"> extraction regex out of
// the bundle: html.match(/<main.../i);
var spaContentRegex = regexp.MustCompile(`html\.match\((/<main[^\n]*?/i)\);`)

// spaHarness applies the extracted regex to each shell and reports the capture.
const spaHarness = `
const re = eval(process.env.GOISEKAI_REGEX);
const shells = JSON.parse(process.env.GOISEKAI_SHELLS);
const out = {};
for (const k of Object.keys(shells)) {
  const m = shells[k].match(re);
  out[k] = m ? m[1] : null;
}
console.log(JSON.stringify(out));
`

// TestFrontendSPAContentExtraction guards the SPA router's content swap. The
// router pulls the inside of <main id="content"> out of an X-Partial response
// with an inline regex that is duplicated across the bundle. A typo in one copy
// (shipped once as [sS]) or a stricter copy that stops tolerating attributes
// silently swaps the wrong thing, with no server-side error.
func TestFrontendSPAContentExtraction(t *testing.T) {
	node := nodePath(t)

	bundle, err := os.ReadFile(filepath.Join(frontendLibDir, "alpine-components.js"))
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}

	copies := spaContentRegex.FindAllStringSubmatch(string(bundle), -1)
	if len(copies) == 0 {
		t.Fatal(`no <main id="content"> extraction regex found in the bundle — ` +
			"the SPA router cannot swap page content")
	}
	want := copies[0][1]
	for i, m := range copies[1:] {
		if m[1] != want {
			t.Errorf("SPA content regex copy %d is %q, want %q — every copy must be identical",
				i+2, m[1], want)
		}
	}

	// The regex must match the shells the layouts actually emit. base.lua puts
	// class attributes after id="content"; blank.lua puts none.
	shells := map[string]string{}
	for _, layout := range []string{"base", "blank"} {
		path := filepath.Join(templatesDir, "layouts", layout+".lua")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		_, rest, _ := strings.Cut(string(data), "<main")
		open, _, ok := strings.Cut("<main"+rest, ">")
		if !ok {
			t.Fatalf("%s has no <main> opening tag", path)
		}
		shells[layout] = open + ">\nCONTENT\n</main>"
	}

	payload, err := json.Marshal(shells)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "spa.js")
	if err := os.WriteFile(script, []byte(spaHarness), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}
	cmd := exec.Command(node, script)
	cmd.Env = append(os.Environ(),
		"GOISEKAI_REGEX="+want,
		"GOISEKAI_SHELLS="+string(payload),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("SPA regex harness failed:\n%s", out)
	}
	var got map[string]*string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("harness output %q is not JSON: %v", out, err)
	}
	for name := range shells {
		if got[name] == nil || *got[name] != "\nCONTENT\n" {
			t.Errorf("%s layout: inner <main> extracted as %v, want %q", name, got[name], "\nCONTENT\n")
		}
	}
}

// actionSubmitHarness submits a POST form pointing at /action/ and reports
// whether the bundle handled it in place: no new history entry, no navigation,
// and the response body swapped into #content. A genre chip or a library toggle
// must not reload the page, because the reader clicks several of them in a row.
const actionSubmitHarness = `
global.window = global;
const noop = () => {};
const calls = { fetch: [], push: 0, replace: 0 };
const listeners = { document: {} };
const mkEl = () => ({
  style: {}, dataset: {},
  classList: { add: noop, remove: noop, toggle: noop, contains: () => false },
  appendChild: noop, setAttribute: noop, remove: noop,
  querySelector: () => null, querySelectorAll: () => [],
});
const content = { innerHTML: '' };
const form = {
  tagName: 'FORM',
  getAttribute: (n) => ({ method: 'post', action: '/action/add-category/demo/m1' }[n] ?? null),
  querySelector: () => null,
  requestSubmit: noop,
  submit: noop,
};
global.document = {
  createElement: mkEl,
  head: { appendChild: noop },
  documentElement: { appendChild: noop },
  body: { appendChild: noop },
  addEventListener: (t, fn) => { (listeners.document[t] = listeners.document[t] || []).push(fn); },
  getElementById: (id) => (id === 'content' ? content : null),
  querySelector: () => null,
  querySelectorAll: () => [],
};
global.localStorage = { getItem: () => null, setItem: noop, removeItem: noop };
global.Alpine = { store: () => null, initTree: noop, data: noop, plugin: noop };
global.addEventListener = noop;
global.FormData = function () { return [['category', 'Shounen']]; };
global.fetch = (url, opts) => {
  calls.fetch.push([url, opts.method]);
  return Promise.resolve({
    ok: true, status: 200, url: 'http://localhost/view/manga/demo/m1',
    headers: { get: () => null },
    text: () => Promise.resolve('<main id="content">swapped</main>'),
  });
};
global.history = {
  state: null,
  pushState: () => { calls.push++; },
  replaceState: () => { calls.replace++; },
};
global.location = new URL('http://localhost/view/manga/demo/m1');
global.scrollTo = noop;
global.scrollY = 0;
global.MutationObserver = class { observe() {} disconnect() {} };
global.requestAnimationFrame = noop;

require(process.env.GOISEKAI_BUNDLE);

(listeners.document['alpine:init'] || []).forEach((fn) => fn());
const evt = {
  target: form, submitter: null, defaultPrevented: false,
  preventDefault() { this.defaultPrevented = true; },
};
(listeners.document.submit || []).forEach((fn) => fn(evt));

setTimeout(() => {
  console.log(JSON.stringify({
    method: calls.fetch.length ? calls.fetch[0][1] : null,
    url: calls.fetch.length ? calls.fetch[0][0] : null,
    fetches: calls.fetch.length,
    prevented: evt.defaultPrevented,
    push: calls.push,
    content: content.innerHTML,
    href: global.location.href,
  }));
}, 10);
`

// TestFrontendActionSubmitStaysInPlace covers the action layer that the reader
// uses repeatedly (genre chips, library toggle, fetch enrichment). Those clicks
// must stay off the history stack: a native POST would add an entry, so going
// back afterwards would land on the action instead of the page the reader came
// from, losing the search results they were browsing.
func TestFrontendActionSubmitStaysInPlace(t *testing.T) {
	node := nodePath(t)
	bundle, err := filepath.Abs(filepath.Join(frontendLibDir, "alpine-components.js"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "action.js")
	if err := os.WriteFile(script, []byte(actionSubmitHarness), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}

	cmd := exec.Command(node, script)
	cmd.Env = append(os.Environ(), "GOISEKAI_BUNDLE="+bundle)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("loading %s threw:\n%s", filepath.Base(bundle), err)
	}
	var got struct {
		Method    string `json:"method"`
		URL       string `json:"url"`
		Fetches   int    `json:"fetches"`
		Prevented bool   `json:"prevented"`
		Push      int    `json:"push"`
		Content   string `json:"content"`
		Href      string `json:"href"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("harness output %q is not JSON: %v", out, err)
	}
	if !got.Prevented {
		t.Fatal("a POST form to /action/ was left to the browser — the page reloads " +
			"and the action lands on the history stack")
	}
	if got.Fetches != 1 || got.Method != "POST" || got.URL != "/action/add-category/demo/m1" {
		t.Errorf("bundle made %d request(s) [%s %s], want exactly one POST to the form action",
			got.Fetches, got.Method, got.URL)
	}
	if got.Push != 0 {
		t.Errorf("action pushed %d history entr(ies), want 0 — the action is not a page", got.Push)
	}
	if got.Content != "swapped" {
		t.Errorf("#content holds %q after the action, want the swapped body", got.Content)
	}
	if got.Href != "http://localhost/view/manga/demo/m1" {
		t.Errorf("action navigated to %q, want the URL to stay put", got.Href)
	}
}

func contains(haystack []string, needle string) bool {
	return slices.Contains(haystack, needle)
}
