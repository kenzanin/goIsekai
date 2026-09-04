// Package templates wires CloudyKit/jet to the embedded template tree and
// exposes the small set of global helpers the views rely on.
package templates

import (
	"embed"
	"io"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/CloudyKit/jet/v6"
)

//go:embed layouts views partials
var templatesFS embed.FS

// dateLayouts are tried in order by formatDate.
var dateLayouts = []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"}

// Engine is a jet.Set preloaded with the embedded templates and globals.
type Engine struct {
	Set *jet.Set
}

// New loads every embedded template into an InMemLoader and registers the
// global view helpers. jet.NewInMemLoader is required because jet's
// NewOSFileSystemLoader cannot read from an embed.FS.
func New(devMode bool) (*Engine, error) {
	loader := jet.NewInMemLoader()
	err := fs.WalkDir(templatesFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, err := templatesFS.ReadFile(path)
		if err != nil {
			return err
		}
		// Store at the embed's relative slash path (e.g. "layouts/base.jet")
		// so extends/include strings in templates resolve against it.
		loader.Set(path, string(content))
		return nil
	})
	if err != nil {
		return nil, err
	}

	opts := []jet.Option{jet.DevelopmentMode(false)}
	if devMode {
		opts = []jet.Option{jet.InDevelopmentMode()} // hot-reload: reparse on every Execute.
	}
	set := jet.NewSet(loader, opts...)

	// Global helpers, registered so they are callable from every template.
	set.AddGlobal("formatDate", formatDate)
	set.AddGlobal("formatChapterNum", formatChapterNum)
	set.AddGlobal("getInitials", getInitials)
	set.AddGlobal("add", add)
	set.AddGlobal("ratioAt", ratioAt)
	set.AddGlobal("statAt", statAt)
	set.AddGlobal("formatBytes", formatBytes)
	set.AddGlobal("pageWindow", pageWindow)
	set.AddGlobal("pageURL", pageURL)
	set.AddGlobal("pagination", newPagination)

	return &Engine{Set: set}, nil
}

// Render executes the named embedded template (e.g. "views/library.jet") and
// writes the result to w.
func (e *Engine) Render(w io.Writer, name string, vars jet.VarMap, data any) error {
	tmpl, err := e.Set.GetTemplate(name)
	if err != nil {
		return err
	}
	// ponytail: jet errors on any missing field/key access, so guarantee the
	// keys nav.jet reads (.active) are always present. A view that supplies its
	// own "active" value wins; otherwise the tab highlight is simply off.
	if data == nil {
		data = map[string]any{}
	}
	if m, ok := data.(map[string]any); ok {
		if _, exists := m["active"]; !exists {
			m["active"] = ""
		}
	}
	return tmpl.Execute(w, vars, data)
}

// formatDate formats an ISO-8601 timestamp as "Jan 2, 2006". Empty input or a
// value that does not parse returns "—".
func formatDate(ts string) string {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return "—"
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t.Format("Jan 2, 2006")
		}
	}
	return "—"
}

// formatChapterNum trims a float chapter number to its meaningful digits
// (5.0 -> "5", 5.5 -> "5.5"). nil/empty/unparseable -> "—".
func formatChapterNum(n any) string {
	switch v := n.(type) {
	case nil:
		return "—"
	case int:
		return strconv.Itoa(v)
	case float64:
		return chapterFloat(v)
	case float32:
		return chapterFloat(float64(v))
	case string:
		if strings.TrimSpace(v) == "" {
			return "—"
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return v
		}
		return chapterFloat(f)
	default:
		return "—"
	}
}

// chapterFloat formats a float without trailing zeros. FormatFloat with -1
// precision already drops them (5.0 -> "5", 5.50 -> "5.5"), so no manual trim
// is needed.
func chapterFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// ratioAt looks up a plugin's thumb ratio by id, returning 0 when unknown —
// jet v6 has no map index() at runtime, so the map lookup lives here.
func ratioAt(ratios map[string]float64, pluginID string) float64 {
	return ratios[pluginID]
}

// statAt looks up a manga's library stats by id, returning nil when missing —
// jet v6 has no map index() at runtime, so the map lookup lives here.
func statAt(stats map[string]map[string]any, mangaID string) map[string]any {
	return stats[mangaID]
}

// getInitials returns the uppercase first letter of the first two words of
// title (e.g. "Dark Hunter" -> "DH").
func getInitials(title string) string {
	parts := strings.Fields(title)
	initials := make([]byte, 0, 2)
	for i := 0; i < 2 && i < len(parts); i++ {
		r := []rune(parts[i])
		if len(r) == 0 {
			continue
		}
		u := strings.ToUpper(string(r[0]))
		if len(u) > 0 {
			initials = append(initials, u[0])
		}
	}
	return string(initials)
}

// add sums two ints — used in templates for loop offsets, e.g.
// {{range $i, $x := .Items}}{{add $i 1}}{{end}}.
func add(a, b int) int {
	return a + b
}

// formatBytes renders a byte count in human-readable units (B, KB, MB, GB).
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit && exp < 3; m /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(n)/float64(div), 'f', 1, 64) + " " + string("KMG"[exp]) + "B"
}

// Pagination carries the state the pagination.jet partial renders: the URL
// base (path + any fixed query prefix), the page query-param name, the current
// page, and the total page count. Extra holds alternating key/value pairs to
// preserve on every generated link (e.g. q and pluginID on the search page).
type Pagination struct {
	Base    string
	Param   string
	Current int
	Total   int
	Extra   []string
}

// URL returns the href for the given page number.
func (p Pagination) URL(page int) string {
	q := make([]string, 0, len(p.Extra)/2+1)
	for i := 0; i+1 < len(p.Extra); i += 2 {
		q = append(q, p.Extra[i]+"="+p.Extra[i+1])
	}
	q = append(q, p.Param+"="+strconv.Itoa(page))
	sep := "?"
	if strings.Contains(p.Base, "?") {
		sep = "&"
	}
	return p.Base + sep + strings.Join(q, "&")
}

// newPagination builds a Pagination for templates. Extra alternates key,
// value, ... pairs that every page link must preserve.
func newPagination(base, param string, current, total int, extra ...string) Pagination {
	return Pagination{Base: base, Param: param, Current: current, Total: total, Extra: extra}
}

// pageURL is a global helper so Jet templates can build page URLs from a
// local Pagination variable: {{ pageURL(p, pageNum) }}.
func pageURL(p Pagination, page int) string {
	return p.URL(page)
}

// pageWindow returns the page numbers to render for numbered pagination.
// Gaps between the always-included first/last/current-neighbour pages are
// represented by 0, which the template renders as an ellipsis. Result is
// deduped and ascending. E.g. current=5,total=100 -> [1 0 4 5 6 0 100].
func pageWindow(current, total int) []int {
	if total <= 0 {
		return nil
	}
	if total <= 7 {
		pages := make([]int, total)
		for i := range pages {
			pages[i] = i + 1
		}
		return pages
	}
	seen := map[int]bool{1: true, total: true}
	for _, p := range []int{current - 1, current, current + 1} {
		if p >= 1 && p <= total {
			seen[p] = true
		}
	}
	sorted := make([]int, 0, len(seen))
	for p := range seen {
		sorted = append(sorted, p)
	}
	sort.Ints(sorted)
	out := make([]int, 0, len(sorted)+2)
	prev := sorted[0]
	out = append(out, prev)
	for _, p := range sorted[1:] {
		if p != prev+1 {
			out = append(out, 0)
		}
		out = append(out, p)
		prev = p
	}
	return out
}
