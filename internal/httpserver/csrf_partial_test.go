package httpserver

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const tmplRoot = "../../internal/templates"

// A partial that renders CSRF-protected forms must be handed the token by its
// caller. Three partials receive an explicit literal table instead of the
// view's own data, and when the caller left csrf_token out every form in them
// silently posted an empty token — masked in the browser only because the
// Alpine interceptor adds the header from the page meta tag.
func TestEveryCSRFPartialIsCalledWithAToken(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(tmplRoot, "partials"))
	if err != nil {
		t.Fatalf("read partials: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		body, err := os.ReadFile(filepath.Join(tmplRoot, "partials", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(body), "csrfInput") {
			continue
		}
		if !calledWithToken(t, name) {
			t.Errorf("partials/%s renders csrfInput forms but no caller passes csrf_token; "+
				"every one of those forms will post an empty token", name)
		}
	}
}

// calledWithToken reports whether any view invokes the partial with a
// csrf_token field in the table literal. The call name is whatever local the
// view bound the require to (detailChapters, detailAlt, pluginCards), so the
// binding is parsed rather than guessed.
func calledWithToken(t *testing.T, partial string) bool {
	t.Helper()
	dir := filepath.Join(tmplRoot, "views")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read views: %v", err)
	}
	want := "partials." + strings.TrimSuffix(partial, ".lua")
	bind := regexp.MustCompile(`local\s+(\w+)\s*=\s*require\("` + regexp.QuoteMeta(want) + `"`)
	for _, e := range entries {
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		src := string(body)
		for _, alias := range bind.FindAllStringSubmatch(src, -1) {
			if tableLiteralHasToken(src, alias[1]) {
				return true
			}
		}
	}
	return false
}

// tableLiteralHasToken looks at the {...} literal passed to call and reports
// whether it carries a csrf_token field.
func tableLiteralHasToken(src, call string) bool {
	for idx := 0; ; {
		at := strings.Index(src[idx:], call+"({")
		if at < 0 {
			return false
		}
		at += idx
		tail := src[at:]
		if end := strings.Index(tail, "})"); end > 0 && strings.Contains(tail[:end], "csrf_token") {
			return true
		}
		idx = at + len(call)
	}
}
