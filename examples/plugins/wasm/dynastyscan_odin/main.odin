// DynastyScan (dynasty-scans.com) source plugin for goIsekai — Odin wasip1 reactor.
//
// Behavioural port of examples/plugins/lua/dynastyscan/main.lua, which stays the
// reference implementation: the Lua original is never deleted, only
// app_data/plugins/dynastyscan/ is swapped to the wasm build.
//
// Ruby-on-Rails site. Almost everything is JSON, which the Lua source already
// exploited: the series document `/series/<slug>.json` carries the detail fields
// AND the chapter list ("taggings"), and `/chapters/<permalink>.json` carries the
// page list. Only search and the genre index are HTML.
//
//   search    GET /search?q=<q>[&page=N]         -> <dd> result blocks
//             GET /tags/<slug>/[?page=N]         -> tag archive, same blocks
//   detail    GET /series/<slug>.json            -> name/cover/description/tags
//   chapters  GET /series/<slug>.json            -> taggings[]
//   pages     GET /chapters/<permalink>.json     -> pages[]
//   genres    GET /tags                          -> tag index links
//
// Three behaviours are load-bearing, copied from the Lua source rather than
// guessed:
//
// 1. Manga id is the BARE SLUG (`/series/<slug>`), not the full URL.
// 2. Chapter id is the `permalink` from `taggings` (e.g. `if_you_could_see_love_ch01`)
//    and its url is `/chapters/<permalink>` — the JSON endpoint path, not the
//    reader path.
// 3. Genre tags are the `type == "General"` entries of `data.tags`; `Author` and
//    `Status` share the same array, and the top-level "type" field is the
//    container kind (Series/Anthology/Doujin), not the reading status.
//
// Deliberate deltas over the Lua original:
//
// - `manga_id` is filled on every chapter (Lua leaves it empty).
// - `released_at` is emitted from `taggings[].released_on` (Lua drops it). The
//   host feeds it into the chapter row, so a real value beats a zero time.
// - `alt_title` is NOT emitted: nothing in the host decodes that field (it is not
//   in pkg/types and no Go code reads `"alt_title"`), so the Lua line was dead
//   weight. Cross-source alt titles come from the alt_titles table instead.
package dynastyscan_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:slice"
import "core:fmt"
import "core:strings"

kSite :: "https://dynasty-scans.com"
kHeaders :: `{"Accept":"text/html,application/xhtml+xml","User-Agent":"Mozilla/5.0 (compatible; goIsekai-plugin/1.0)"}`
kJsonHeaders :: `{"Accept":"application/json","User-Agent":"Mozilla/5.0 (compatible; goIsekai-plugin/1.0)"}`

// ─── wire types (ABI in) ────────────────────────────────────────────────────

Search_Filter :: struct {
	Query:  string   `json:"query"`,
	Page:   int      `json:"page"`,
	Genres: []string `json:"genres"`,
}

// ─── wire types (ABI out) ───────────────────────────────────────────────────

Manga_Out :: struct {
	ID:        string   `json:"id"`,
	Title:     string   `json:"title"`,
	Cover_URL: string   `json:"cover_url"`,
	Author:    string   `json:"author,omitempty"`,
	Desc:      string   `json:"description,omitempty"`,
	Status:    string   `json:"status,omitempty"`,
	Genres:    []string `json:"genres,omitempty"`,
}

Chapter_Out :: struct {
	ID:          string `json:"id"`,
	Manga_ID:    string `json:"manga_id"`,
	Title:       string `json:"title"`,
	Chapter_Num: f64    `json:"chapter_num"`,
	Released_At: string `json:"released_at,omitempty"`,
	URL:         string `json:"url"`,
}

Page_Out :: struct {
	Index: int    `json:"index"`,
	URL:   string `json:"url"`,
}

Genre_Out :: struct {
	Name: string `json:"name"`,
	Slug: string `json:"slug"`,
}

// ─── wire types (site JSON) ─────────────────────────────────────────────────

// Tag_Item covers all three types the site mixes into one array: Author,
// General (the genre set) and Status.
Tag_Item :: struct {
	Type:      string `json:"type"`,
	Name:      string `json:"name"`,
	Permalink: string `json:"permalink"`,
}

Tagging :: struct {
	Title       : string `json:"title"`,
	Permalink   : string `json:"permalink"`,
	Released_On : string `json:"released_on"`,
}

// Series is `/series/<slug>.json`. One response serves both detail and chapters,
// and `aliases` is deliberately absent: the site types it as either a string or
// an array, and a type mismatch would fail the whole decode for a field nothing
// consumes.
Series :: struct {
	Name        : string     `json:"name"`,
	Cover       : string     `json:"cover"`,
	Description : string     `json:"description"`,
	Tags        : []Tag_Item `json:"tags"`,
	Taggings    : []Tagging  `json:"taggings"`,
}

Chapter_Page :: struct {
	URL: string `json:"url"`,
}

Chapter_JSON :: struct {
	Pages: []Chapter_Page `json:"pages"`,
}

// ─── helpers ────────────────────────────────────────────────────────────────

// decode_id unwraps the JSON the host passes for a single-string ABI: a bare
// quoted id ("my-slug") or an object ({"id":"my-slug"}) both end up as the slug.
decode_id :: proc (arg: string) -> string {
	v: string
	if json.unmarshal_string(arg, &v) != nil {
		v = strings.trim_space(arg)
	}
	return strings.trim_space(v)
}

// absolute turns the site's root-relative media paths into full URLs.
absolute :: proc (path: string) -> string {
	if path == "" || path == "null" {
		return ""
	}
	if strings.has_prefix(path, "http") {
		return path
	}
	return strings.concatenate({kSite, path})
}

fetch_series :: proc (slug: string) -> (Series, bool) {
	body, ok := abi.http_body(strings.concatenate({kSite, "/series/", abi.url_encode(slug), ".json"}), kJsonHeaders)
	if !ok {
		return Series{}, false
	}
	s: Series
	if json.unmarshal_string(body, &s) != nil {
		return Series{}, false
	}
	return s, true
}

// ─── search ─────────────────────────────────────────────────────────────────

search_impl :: proc (arg: string) -> string {
	f: Search_Filter
	if json.unmarshal_string(arg, &f) != nil {
		return "[]"
	}
	if f.Page < 1 {
		f.Page = 1
	}

	// A genre filter goes to the tag archive (/tags/<slug>/, page as a query
	// parameter) instead of the search endpoint, exactly like the Lua source.
	url: string
	if len(f.Genres) > 0 {
		url = strings.concatenate({kSite, "/tags/", abi.url_encode(f.Genres[0]), "/"})
		if f.Page > 1 {
			url = fmt.aprintf("%s?page=%d", url, f.Page)
		}
	} else {
		url = strings.concatenate({kSite, "/search?q=", abi.url_encode(strings.trim_space(f.Query))})
		if f.Page > 1 {
			url = fmt.aprintf("%s&page=%d", url, f.Page)
		}
	}

	body, ok := abi.http_body(url, kHeaders)
	if !ok {
		return "[]"
	}

	// Each result is one <dd> block. Matching whole blocks keeps the title paired
	// with the right slug — titles repeat across a page.
	//
	// find_all, not find_all_pairs: this pattern has ONE capture group, and the
	// host renders a single-value row as a bare string (wasmRegexRow), so a
	// [][]string decode would fail and hand back nothing. find_all reads []string.
	blocks := abi.find_all(body, `(?s)<dd>(.*?)</dd>`)
	out := make([dynamic]Manga_Out, 0, len(blocks))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)

	for block in blocks {
		slug := abi.find_one(block, `href="/series/([a-z0-9_]+)"`)
		if slug == "" || seen[slug] {
			continue
		}
		title := strings.trim_space(abi.host_str("text.unescape", abi.find_one(block, `class="name"[^>]*>([^<]+)</a>`)))
		if title == "" {
			continue
		}
		seen[slug] = true
		// The result card carries no cover image at all — the only <img> on the
		// search page is the site logo — so the cover has to come from the
		// series JSON, the same response the detail call reads.
		cover := ""
		if s, ok := fetch_series(slug); ok {
			cover = absolute(s.Cover)
		}
		append(&out, Manga_Out{ID = slug, Title = title, Cover_URL = cover})
	}
	return abi.to_json(out[:])
}

// ─── detail ─────────────────────────────────────────────────────────────────

detail_impl :: proc (arg: string) -> string {
	slug := decode_id(arg)
	if slug == "" {
		return "{}"
	}

	s, ok := fetch_series(slug)
	if !ok {
		// Failure shape for an object-returning export: bare {id}. The host does
		// not cache it, so the next call retries; an [] would not decode.
		return abi.to_json(Manga_Out{ID = slug})
	}

	genres := make([dynamic]string, 0, 8)
	defer delete(genres)
	author := ""
	status := ""

	for tag in s.Tags {
		switch tag.Type {
		case "Author":
			if author == "" {
				author = tag.Name
			} else if tag.Name != "" {
				author = strings.concatenate({author, ", ", tag.Name})
			}
		case "General":
			if tag.Name != "" {
				append(&genres, tag.Name)
			}
		case "Status":
			// First Status tag wins, same as the Lua source.
			if status == "" {
				status = abi.host_str("text.normalize_status", tag.Name)
			}
		}
	}

	// The site ships the description as HTML; flatten it to one line.
	desc := strings.trim_space(abi.sub(abi.host_str("text.strip_html", s.Description), `\s+`, " "))

	return abi.to_json(Manga_Out {
		ID = slug,
		Title = s.Name,
		Cover_URL = absolute(s.Cover),
		Author = author,
		Desc = desc,
		Status = status,
		Genres = genres[:],
	})
}

// ─── chapters ───────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	slug := decode_id(arg)
	if slug == "" {
		return "[]"
	}

	s, ok := fetch_series(slug)
	if !ok {
		return "[]"
	}

	out := make([dynamic]Chapter_Out, 0, len(s.Taggings))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)

	for t in s.Taggings {
		if t.Permalink == "" || seen[t.Permalink] {
			continue
		}
		seen[t.Permalink] = true
		label := strings.trim_space(t.Title)
		append(&out, Chapter_Out {
			ID = t.Permalink,
			Manga_ID = slug,
			Title = label,
			Chapter_Num = abi.chapter_num(label),
			Released_At = abi.host_str("text.date_to_iso", t.Released_On),
			URL = strings.concatenate({kSite, "/chapters/", t.Permalink}),
		})
	}

	// "taggings" is already newest-first (it is the site's own release feed), but
	// the ABI requires the order explicitly. stable: equal chapter numbers (the
	// extras that carry no number) must keep their feed order.
	slice.stable_sort_by(out[:], proc (a, b: Chapter_Out) -> bool {
		return a.Chapter_Num > b.Chapter_Num
	})
	return abi.to_json(out[:])
}

// ─── pages ──────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	permalink := decode_id(arg)
	if permalink == "" {
		return "[]"
	}

	body, ok := abi.http_body(strings.concatenate({kSite, "/chapters/", abi.url_encode(permalink), ".json"}), kJsonHeaders)
	if !ok {
		return "[]"
	}
	data: Chapter_JSON
	if json.unmarshal_string(body, &data) != nil {
		return "[]"
	}

	out := make([dynamic]Page_Out, 0, len(data.Pages))
	defer delete(out)
	for p in data.Pages {
		url := absolute(p.URL)
		if url == "" {
			continue
		}
		append(&out, Page_Out{Index = len(out), URL = url})
	}
	return abi.to_json(out[:])
}

// ─── genres (optional export) ───────────────────────────────────────────────

// Read from the site's own /tags index rather than a hardcoded list, so the
// genres cannot drift from what the archive URLs accept. `/tags.json` exists but
// keys its payload by letter ({"A": [...]}), and map iteration has no order, so
// the HTML index — which is already alphabetical — is the better source.
genres_impl :: proc () -> string {
	body, ok := abi.http_body(strings.concatenate({kSite, "/tags"}), kHeaders)
	if !ok {
		return "[]"
	}

	rows := abi.find_all_pairs(body, `href="/tags/([a-z0-9_%-]+)"[^>]*>([^<]+)<`)
	out := make([dynamic]Genre_Out, 0, len(rows))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)

	for row in rows {
		if len(row) < 2 {
			continue
		}
		slug := row[0]
		name := strings.trim_space(abi.host_str("text.unescape", row[1]))
		if slug == "" || name == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		append(&out, Genre_Out{Name = name, Slug = slug})
	}
	return abi.to_json(out[:])
}

// ─── exports ────────────────────────────────────────────────────────────────

@(export)
Search :: proc "c" (ptr: u32, length: u32) -> u64 {
	context = runtime.default_context()
	return abi.reply_json(search_impl(abi.read_string(ptr, length)))
}

@(export)
GetMangaDetail :: proc "c" (ptr: u32, length: u32) -> u64 {
	context = runtime.default_context()
	return abi.reply_json(detail_impl(abi.read_string(ptr, length)))
}

@(export)
GetChapterList :: proc "c" (ptr: u32, length: u32) -> u64 {
	context = runtime.default_context()
	return abi.reply_json(chapters_impl(abi.read_string(ptr, length)))
}

@(export)
GetPageList :: proc "c" (ptr: u32, length: u32) -> u64 {
	context = runtime.default_context()
	return abi.reply_json(pages_impl(abi.read_string(ptr, length)))
}

@(export)
GetGenres :: proc "c" (ptr: u32, length: u32) -> u64 {
	context = runtime.default_context()
	return abi.reply_json(genres_impl())
}
