// Package templates provides Lua-based template rendering for the embedded
// template tree.
package templates

import (
	"embed"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed layouts views partials
var templatesFS embed.FS

// dateLayouts are tried in order by formatDate.
var dateLayouts = []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"}

// Engine wraps LuaEngine for template rendering.
type Engine struct {
	lua *LuaEngine
}

// New creates a LuaEngine from the embedded templates and returns an Engine wrapper.
func New(devMode bool) (*Engine, error) {
	luaEng, err := NewLuaEngine(templatesFS, devMode)
	if err != nil {
		return nil, err
	}
	return &Engine{lua: luaEng}, nil
}

// Render executes the named Lua template and writes the result to w.
// The name is the full path (e.g. "views/library") — no .jet suffix needed.
func (e *Engine) Render(w io.Writer, name string, data any) error {
	m := toDataMap(data)
	return e.lua.Render(w, name, m)
}

// RenderPartial renders the named Lua template without the full page layout.
func (e *Engine) RenderPartial(w io.Writer, name string, data any) error {
	m := toDataMap(data)
	return e.lua.RenderPartial(w, name, m)
}

// toDataMap ensures data is a map[string]any; nil or non-map values become an empty map.
func toDataMap(data any) map[string]any {
	if data == nil {
		return map[string]any{}
	}
	if m, ok := data.(map[string]any); ok {
		return m
	}
	return map[string]any{}
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
