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

// spaRouterHarness loads the bundle and records which document and window
// listeners it registers, plus whether navigate is exported.
const spaRouterHarness = `
global.window = global;
const noop = () => {};
const mkEl = () => ({
  style: {}, dataset: {},
  classList: { add: noop, remove: noop, toggle: noop, contains: () => false },
  appendChild: noop, setAttribute: noop, remove: noop,
  querySelector: () => null, querySelectorAll: () => [],
});
const docEvents = [];
const winEvents = [];
global.document = {
  createElement: mkEl,
  head: { appendChild: noop },
  documentElement: { appendChild: noop },
  body: { appendChild: noop },
  addEventListener: (t) => docEvents.push(t),
  getElementById: () => null,
  querySelector: () => null,
  querySelectorAll: () => [],
};
global.localStorage = { getItem: () => null, setItem: noop, removeItem: noop };
global.Alpine = { store: () => null, initTree: noop, data: noop, plugin: noop };
global.addEventListener = (t) => winEvents.push(t);
global.fetch = () => Promise.resolve({ ok: true, text: () => Promise.resolve('') });
global.history = { replaceState: noop, pushState: noop, state: null };
global.location = { origin: 'http://localhost', href: 'http://localhost', pathname: '/' };
global.MutationObserver = class { observe() {} disconnect() {} };
global.requestAnimationFrame = noop;

require(process.env.GOISEKAI_BUNDLE);

console.log(JSON.stringify({ doc: docEvents, win: winEvents, navigate: typeof window.navigate }));
`

// TestFrontendSPARouterRegistersListeners guards the router's wiring. The router
// once shipped nested inside window.setLoading: the listeners only appeared
// after the first loading-state call, and re-registered on every later one, so
// navigation was dead on load and duplicated afterwards. Nothing failed to
// parse, so only an explicit load-time assertion catches it.
func TestFrontendSPARouterRegistersListeners(t *testing.T) {
	node := nodePath(t)
	bundle, err := filepath.Abs(filepath.Join(frontendLibDir, "alpine-components.js"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "router.js")
	if err := os.WriteFile(script, []byte(spaRouterHarness), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}

	cmd := exec.Command(node, script)
	cmd.Env = append(os.Environ(), "GOISEKAI_BUNDLE="+bundle)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("loading %s threw:\n%s", filepath.Base(bundle), out)
	}

	var got struct {
		Doc      []string `json:"doc"`
		Win      []string `json:"win"`
		Navigate string   `json:"navigate"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("harness output %q is not JSON: %v", out, err)
	}

	for _, evt := range []string{"click", "submit"} {
		if !slices.Contains(got.Doc, evt) {
			t.Errorf("document has no %q listener at load time (registered: %v) — "+
				"the SPA router never intercepts, so every link does a full page load", evt, got.Doc)
		}
	}
	if !slices.Contains(got.Win, "popstate") {
		t.Errorf("window has no %q listener at load time (registered: %v) — "+
			"back and forward would reload the document", "popstate", got.Win)
	}
	if got.Navigate != "function" {
		t.Errorf("window.navigate is %s, want function — actions and templates cannot "+
			"trigger in-place navigation", got.Navigate)
	}
}

// spaFailureHarness drives one navigate() call with a failing fetch and reports
// the toasts raised and where the fallback navigated to. GOISEKAI_FETCH=reject
// simulates a network error, any other value a non-2xx response.
const spaFailureHarness = `
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
const toasts = [];
global.Alpine = {
  store: (name) => (name === 'toast' ? { show: (m, l) => toasts.push([m, l]) } : null),
  initTree: noop, data: noop, plugin: noop,
};
global.addEventListener = noop;
const mode = process.env.GOISEKAI_FETCH;
global.fetch = () => mode === 'reject'
  ? Promise.reject(new Error('offline'))
  : Promise.resolve({ ok: false, status: 500, headers: { get: () => null }, text: () => Promise.resolve('') });
global.history = { replaceState: noop, pushState: noop, state: null };
global.location = { origin: 'http://localhost', href: 'http://localhost', pathname: '/' };
global.MutationObserver = class { observe() {} disconnect() {} };
global.requestAnimationFrame = noop;

require(process.env.GOISEKAI_BUNDLE);

window.navigate('/view/library', { push: true });

setTimeout(() => {
  console.log(JSON.stringify({ toasts, href: global.location.href }));
}, 10);
`

// TestFrontendSPANavigationFailureReportsAndFallsBack covers the spec's fetch
// error scenario: a failed navigation must both fall back to a standard page
// load and tell the user. Without the toast the page just goes blank with no
// explanation, which is what shipped before.
func TestFrontendSPANavigationFailureReportsAndFallsBack(t *testing.T) {
	node := nodePath(t)
	bundle, err := filepath.Abs(filepath.Join(frontendLibDir, "alpine-components.js"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "failure.js")
	if err := os.WriteFile(script, []byte(spaFailureHarness), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}

	for _, mode := range []string{"500", "reject"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.Command(node, script)
			cmd.Env = append(os.Environ(),
				"GOISEKAI_BUNDLE="+bundle,
				"GOISEKAI_FETCH="+mode,
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("loading %s threw:\n%s", filepath.Base(bundle), out)
			}
			var got struct {
				Toasts [][2]string `json:"toasts"`
				Href   string      `json:"href"`
			}
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("harness output %q is not JSON: %v", out, err)
			}
			if len(got.Toasts) == 0 {
				t.Error("a failed navigation raised no toast - the user gets no explanation")
			} else if got.Toasts[0][1] != "error" {
				t.Errorf("failed navigation raised a %q toast, want error", got.Toasts[0][1])
			}
			if got.Href != "/view/library" {
				t.Errorf("fallback navigation landed on %q, want the requested /view/library - "+
					"a failed SPA fetch must hand off to a standard page load", got.Href)
			}
		})
	}
}

// spaNavHighlightHarness records which navigation anchors the bundle
// highlights after each code path that swaps the body from a partial response.
// GOISEKAI_PATH selects the path: "click" drives navigate(), "popstate" drives
// the back/forward handler. Both must apply the X-Active-Nav header.
const spaNavHighlightHarness = `
global.window = global;
const noop = () => {};
const tokens = ['library', 'plugins', 'authed'];
// Stable element instances: the bundle mutates their classList in place, so the
// harness must hand back the SAME objects on every querySelectorAll.
const navLinks = tokens.map((token) => {
  const classes = new Set(['nav-link']);
  return {
    classList: {
      add: (...c) => c.forEach((x) => classes.add(x)),
      remove: (...c) => c.forEach((x) => classes.delete(x)),
      toggle: noop,
      contains: (c) => classes.has(c),
    },
    getAttribute: (n) => (n === 'data-nav' ? token : '/view/' + token),
    closest: () => null,
  };
});
const listeners = { window: {}, document: {} };
global.document = {
  createElement: () => ({ style: {}, appendChild: noop, setAttribute: noop }),
  head: { appendChild: noop },
  documentElement: { appendChild: noop },
  body: { appendChild: noop },
  addEventListener: (t, fn) => { listeners.document[t] = fn; },
  getElementById: () => ({ innerHTML: '', addEventListener: noop }),
  querySelector: (sel) => (sel === 'nav' ? {} : null),
  querySelectorAll: (sel) => (sel === 'a[data-nav]' ? navLinks : []),
};
global.localStorage = { getItem: () => null, setItem: noop, removeItem: noop };
global.Alpine = { store: () => null, initTree: noop, data: noop, plugin: noop };
global.addEventListener = (t, fn) => { listeners.window[t] = fn; };
const activeHeader = process.env.GOISEKAI_ACTIVE_NAV;
global.fetch = () => Promise.resolve({
  ok: true,
  status: 200,
  headers: { get: (n) => (n === 'X-Active-Nav' ? activeHeader : null) },
  text: () => Promise.resolve('<main id="content">page</main>'),
});
global.history = { replaceState: noop, pushState: noop, state: null };
global.location = { origin: 'http://localhost', href: 'http://localhost', pathname: '/view/plugins' };
global.scrollTo = noop;
global.scrollY = 0;
global.MutationObserver = class { observe() {} disconnect() {} };
global.requestAnimationFrame = noop;

require(process.env.GOISEKAI_BUNDLE);

if (process.env.GOISEKAI_PATH === 'click') {
  window.navigate('/view/plugins', { push: true });
} else if (process.env.GOISEKAI_PATH === 'popstate-null') {
  // The entry a freshly loaded document sits on carries null state.
  listeners.window.popstate({ state: null });
} else {
  listeners.window.popstate({ state: { scrollY: 0 } });
}
setTimeout(() => {
  const highlighted = navLinks
    .filter((l) => l.classList.contains('border-indigo-400'))
    .map((l) => l.getAttribute('data-nav'));
  console.log(JSON.stringify({ highlighted }));
}, 10);
`

// TestFrontendSPANavHighlightFollowsPartialResponse covers the spec's
// "Navigation highlight follows the page" scenario for BOTH paths that swap the
// body. The popstate handler shipped without applying X-Active-Nav, so
// back/forward moved the page but left the old item highlighted.
func TestFrontendSPANavHighlightFollowsPartialResponse(t *testing.T) {
	node := nodePath(t)
	bundle, err := filepath.Abs(filepath.Join(frontendLibDir, "alpine-components.js"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "nav.js")
	if err := os.WriteFile(script, []byte(spaNavHighlightHarness), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}

	for _, path := range []string{"click", "popstate", "popstate-null"} {
		t.Run(path, func(t *testing.T) {
			cmd := exec.Command(node, script)
			cmd.Env = append(os.Environ(),
				"GOISEKAI_BUNDLE="+bundle,
				"GOISEKAI_PATH="+path,
				"GOISEKAI_ACTIVE_NAV=plugins",
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("loading %s threw:\n%s", filepath.Base(bundle), out)
			}
			var got struct {
				Highlighted []string `json:"highlighted"`
			}
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("harness output %q is not JSON: %v", out, err)
			}
			if len(got.Highlighted) != 1 || got.Highlighted[0] != "plugins" {
				t.Errorf("%s path highlighted %v, want exactly [plugins] from the X-Active-Nav "+
					"header — the navigation highlight goes stale when it is not applied here",
					path, got.Highlighted)
			}
		})
	}
}

// spaShellSwapHarness drives one navigate() from a page with no nav in the DOM
// (the reader's blank layout) to a page that has one. A partial swap only
// replaces <main>, so the router must fall back to a full load instead of
// leaving the nav bar permanently missing.
const spaShellSwapHarness = `
global.window = global;
const noop = () => {};
let swapped = false;
global.document = {
  createElement: () => ({ style: {}, appendChild: noop, setAttribute: noop }),
  head: { appendChild: noop },
  documentElement: { appendChild: noop },
  body: { appendChild: noop },
  addEventListener: noop,
  getElementById: () => ({ innerHTML: '', addEventListener: noop }),
  querySelector: () => null,
  querySelectorAll: () => [],
};
global.localStorage = { getItem: () => null, setItem: noop, removeItem: noop };
global.Alpine = { store: () => null, initTree: noop, data: noop, plugin: noop };
global.addEventListener = noop;
global.fetch = () => Promise.resolve({
  ok: true,
  status: 200,
  headers: { get: (n) => (n === 'X-Active-Nav' ? 'detail' : null) },
  text: () => { swapped = true; return Promise.resolve('<main id="content">page</main>'); },
});
global.history = { replaceState: noop, pushState: noop, state: null };
global.location = {
  origin: 'http://localhost', href: 'http://localhost/view/read/p/m/ch',
  pathname: '/view/read/p/m/ch',
  replace: (u) => { global.location.href = u; },
};
global.scrollTo = noop;
global.scrollY = 0;
global.MutationObserver = class { observe() {} disconnect() {} };
global.requestAnimationFrame = noop;

require(process.env.GOISEKAI_BUNDLE);

window.navigate('/view/manga/p/m', { push: true });

setTimeout(() => {
  console.log(JSON.stringify({ swapped, href: global.location.href }));
}, 10);
`

// TestFrontendSPAReloadsWhenTheShellChanges covers the reader -> detail case:
// the reader renders the blank layout, so there is no nav in the document. A
// <main>-only swap cannot bring it back, and the reader's Back button left the
// nav bar missing until this guard.
func TestFrontendSPAReloadsWhenTheShellChanges(t *testing.T) {
	node := nodePath(t)
	bundle, err := filepath.Abs(filepath.Join(frontendLibDir, "alpine-components.js"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "shell.js")
	if err := os.WriteFile(script, []byte(spaShellSwapHarness), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}

	cmd := exec.Command(node, script)
	cmd.Env = append(os.Environ(), "GOISEKAI_BUNDLE="+bundle)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("loading %s threw:\n%s", filepath.Base(bundle), out)
	}
	var got struct {
		Swapped bool   `json:"swapped"`
		Href    string `json:"href"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("harness output %q is not JSON: %v", out, err)
	}
	if got.Swapped {
		t.Error("navigation swapped only <main> into a document with no nav - " +
			"the nav bar cannot come back that way")
	}
	if got.Href != "/view/manga/p/m" {
		t.Errorf("fallback navigation landed on %q, want the requested /view/manga/p/m - "+
			"a shell change must hand off to a full page load", got.Href)
	}
}

// spaFormHarness submits a GET form and reports the URL the router navigated
// to. The handler once forwarded only form.action, so nothing the user typed
// survived the trip and the results never changed.
const spaFormHarness = `
global.window = global;
const noop = () => {};
const mkEl = () => ({
  style: {}, dataset: {},
  classList: { add: noop, remove: noop, toggle: noop, contains: () => false },
  appendChild: noop, setAttribute: noop, remove: noop,
  querySelector: () => null, querySelectorAll: () => [],
});
const listeners = {};
global.document = {
  createElement: mkEl,
  head: { appendChild: noop },
  documentElement: { appendChild: noop },
  body: { appendChild: noop },
  addEventListener: (t, fn) => { listeners[t] = fn; },
  getElementById: () => null,
  querySelector: () => null,
  querySelectorAll: () => [],
};
global.localStorage = { getItem: () => null, setItem: noop, removeItem: noop };
global.Alpine = { store: () => null, initTree: noop, data: noop, plugin: noop };
global.addEventListener = noop;

// Record where the router tries to go instead of really fetching.
const requested = [];
global.fetch = (url) => {
  requested.push(url);
  return Promise.resolve({
    ok: true, status: 200, url,
    headers: { get: () => null },
    text: () => Promise.resolve('<main id="content">ok</main>'),
  });
};
global.history = { replaceState: noop, pushState: noop, state: null };
// toString mirrors how a browser can use window.location as a URL base.
global.location = {
  origin: 'http://localhost',
  href: 'http://localhost/view/search',
  pathname: '/view/search',
  toString: () => 'http://localhost/view/search',
};
global.MutationObserver = class { observe() {} disconnect() {} };
global.requestAnimationFrame = noop;

// Node's FormData needs a real DOM element; stand in the values a search form
// would carry so the harness can prove they reach the URL.
let formDataSawForm = null;
global.FormData = class {
  constructor(form) { formDataSawForm = form || null; }
  forEach(fn) { fn('dungeon', 'q'); fn('kaliscan', 'pluginID'); fn('', 'empty'); }
};

require(process.env.GOISEKAI_BUNDLE);

// A GET search form carrying the query the user typed.
const form = {
  method: 'get',
  action: 'http://localhost/view/search',
  getAttribute: (n) => (n === 'method' ? 'get' : '/view/search'),
};
const submitEvent = {
  target: form,
  preventDefault: () => { submitEvent.defaultPrevented = true; },
  defaultPrevented: false,
};
listeners.submit(submitEvent);

setTimeout(() => {
  console.log(JSON.stringify({
    requested,
    prevented: submitEvent.defaultPrevented,
    formDataSawForm: formDataSawForm === form,
  }));
}, 10);
`

// TestFrontendSPAGetFormKeepsQuery guards that an in-place GET form submission
// carries the user's input. Dropping it looks like a working navigation: the
// body swaps, the URL updates, and the results are simply wrong.
func TestFrontendSPAGetFormKeepsQuery(t *testing.T) {
	node := nodePath(t)
	bundle, err := filepath.Abs(filepath.Join(frontendLibDir, "alpine-components.js"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "form.js")
	if err := os.WriteFile(script, []byte(spaFormHarness), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}

	cmd := exec.Command(node, script)
	cmd.Env = append(os.Environ(), "GOISEKAI_BUNDLE="+bundle)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("loading %s threw:\n%s", filepath.Base(bundle), out)
	}
	var got struct {
		Requested       []string `json:"requested"`
		Prevented       bool     `json:"prevented"`
		FormDataSawForm bool     `json:"formDataSawForm"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("harness output %q is not JSON: %v", out, err)
	}
	if !got.Prevented {
		t.Fatal("the submit handler did not preventDefault — the browser would do a full page load")
	}
	if !got.FormDataSawForm {
		t.Error("the handler built FormData from something other than the submitted form")
	}
	if len(got.Requested) != 1 {
		t.Fatalf("router made %d requests (%v), want exactly 1", len(got.Requested), got.Requested)
	}
	want := "/view/search?q=dungeon&pluginID=kaliscan"
	if got.Requested[0] != want {
		t.Errorf("router navigated to %q, want %q — the user's query must survive an "+
			"in-place form submission, and blank fields must be left out", got.Requested[0], want)
	}
}

// spaScrollHarness reports where the router leaves the scroll position for both
// kinds of swap: a link (new page, must start at the top) and a popstate (a
// revisiting page, must restore where the reader was).
const spaScrollHarness = `
global.window = global;
const noop = () => {};
const mkEl = () => ({
  style: {}, dataset: {},
  classList: { add: noop, remove: noop, toggle: noop, contains: () => false },
  appendChild: noop, setAttribute: noop, remove: noop,
  querySelector: () => null, querySelectorAll: () => [],
});
const listeners = {};
global.document = {
  createElement: mkEl,
  head: { appendChild: noop },
  documentElement: { appendChild: noop },
  body: { appendChild: noop },
  addEventListener: (t, fn) => { listeners[t] = fn; },
  getElementById: () => ({ innerHTML: '', addEventListener: noop }),
  querySelector: () => null,
  querySelectorAll: () => [],
};
global.localStorage = { getItem: () => null, setItem: noop, removeItem: noop };
global.Alpine = { store: () => null, initTree: noop, data: noop, plugin: noop };
global.addEventListener = (t, fn) => { listeners['win:' + t] = fn; };
global.fetch = () => Promise.resolve({
  ok: true, status: 200,
  headers: { get: () => null },
  text: () => Promise.resolve('<main id="content">page</main>'),
});
const scrolled = [];
const replaces = [];
const pushes = [];
global.history = {
  replaceState: (state, _t, u) => replaces.push({ state, url: u }),
  pushState: (state, _t, u) => pushes.push({ state, url: u }),
  state: null,
};
global.location = { origin: 'http://localhost', href: 'http://localhost/view/library', pathname: '/view/library' };
global.scrollTo = (x, y) => { scrolled.push(y); };
global.scrollY = 600;
global.MutationObserver = class { observe() {} disconnect() {} };
global.requestAnimationFrame = noop;

require(process.env.GOISEKAI_BUNDLE);

if (process.env.GOISEKAI_PATH === 'click') {
  window.navigate('/view/history', { push: true });
} else {
  listeners['win:popstate']({ state: { scrollY: 123 } });
}

setTimeout(() => {
  console.log(JSON.stringify({ scrolled, replaces, pushes }));
}, 10);
`

// TestFrontendSPAScrollBehaviour pins the scroll contract in three parts: a link
// must open the new page at the top, back/forward must restore the offset
// recorded for the entry, and that offset must belong to the page being left.
// Recording it on the outgoing entry (the obvious-looking version) hands every
// page the previous page's position.
func TestFrontendSPAScrollBehaviour(t *testing.T) {
	node := nodePath(t)
	bundle, err := filepath.Abs(filepath.Join(frontendLibDir, "alpine-components.js"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "scroll.js")
	if err := os.WriteFile(script, []byte(spaScrollHarness), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}

	run := func(t *testing.T, path string) map[string]any {
		t.Helper()
		cmd := exec.Command(node, script)
		cmd.Env = append(os.Environ(),
			"GOISEKAI_BUNDLE="+bundle,
			"GOISEKAI_PATH="+path,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("loading %s threw:\n%s", filepath.Base(bundle), out)
		}
		var got map[string]any
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("harness output %q is not JSON: %v", out, err)
		}
		return got
	}

	t.Run("link opens at the top", func(t *testing.T) {
		got := run(t, "click")
		ys, _ := got["scrolled"].([]any)
		found := false
		for _, y := range ys {
			if y.(float64) == 0 {
				found = true
			}
		}
		if !found {
			t.Errorf("clicked a link and scrollTo received %v, want a call with 0 — a link "+
				"must open the new page at the top, not at the old offset", ys)
		}
	})

	t.Run("offset belongs to the page being left", func(t *testing.T) {
		got := run(t, "click")
		reps, _ := got["replaces"].([]any)
		pushes, _ := got["pushes"].([]any)
		if len(reps) != 1 {
			t.Fatalf("history.replaceState called %d times, want 1 to stamp the entry being left", len(reps))
		}
		stamped := reps[0].(map[string]any)["state"].(map[string]any)["scrollY"].(float64)
		if stamped != 600 {
			t.Errorf("stamped scrollY=%v on the entry being left, want 600 (where the reader "+
				"actually was) — otherwise coming back restores the wrong position", stamped)
		}
		if reps[0].(map[string]any)["url"] != "http://localhost/view/library" {
			t.Errorf("replaceState stamped %v, want the URL of the page being left",
				reps[0].(map[string]any)["url"])
		}
		if len(pushes) != 1 {
			t.Fatalf("history.pushState called %d times, want 1", len(pushes))
		}
		fresh := pushes[0].(map[string]any)["state"].(map[string]any)["scrollY"].(float64)
		if fresh != 0 {
			t.Errorf("new entry recorded scrollY=%v, want 0 — the entry it replaces must not "+
				"inherit the previous page's offset", fresh)
		}
	})

	t.Run("popstate restores the entry's offset", func(t *testing.T) {
		got := run(t, "popstate")
		ys, _ := got["scrolled"].([]any)
		found := false
		for _, y := range ys {
			if y.(float64) == 123 {
				found = true
			}
		}
		if !found {
			t.Errorf("popstate with scrollY=123 and scrollTo received %v, want a call with 123 — "+
				"back/forward must restore the offset recorded for the entry", ys)
		}
	})
}

func contains(haystack []string, needle string) bool {
	return slices.Contains(haystack, needle)
}
