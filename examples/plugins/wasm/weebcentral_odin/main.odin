// Weebcentral (weebcentral.com) source plugin for goIsekai — Odin wasip1 reactor.
//
// Behavioural port of examples/plugins/lua/weebcentral/main.lua, which stays as
// the reference implementation: a successful port keeps its Lua original in
// examples/ next to the wasm build, and only app_data/plugins/<id>/ is swapped.
//
// Server-rendered HTML + HTMX, no JSON API. Every interactive load is a GET
// returning an HTML fragment:
//
//	search   GET /search/data?text=<q>&limit=24&offset=N[&included_tag=T]
//	detail   GET /series/<ulid>/auto          (302 to the canonical slug)
//	chapters GET /series/<ulid>/full-chapter-list   (fragment, newest-first)
//	pages    GET /chapters/<ulid>             (doc carries max_page)
//	         GET /chapters/<ulid>/images?...current_page=N  (one batch per page)
//	genres   GET /search                      (id="tag-<name>" checkboxes)
//
// Four behaviours are load-bearing and copied from the Lua source, not guessed:
//
//  1. `hrefs` carries TWO anchors per card (picture + title) and `imgs` TWO
//     <img> per card (a desktop and a mobile variant of the same cover), so
//     covers are matched to their card by the ULID the cover URL carries —
//     never by position. A positional zip put every cover on the previous
//     card's title.
//  2. The clean title lives in `a.line-clamp-1`; the picture anchor's text is
//     badge noise ("Official..."). Titles never come from the picture anchor.
//  3. `/full-chapter-list` is newest-first; the reader wants ascending, so the
//     list is reversed on the way out.
//  4. The reader page is paginated: the chapter doc only carries `max_page`,
//     and `/images?current_page=N` returns one batch. All batches are walked.
//
// One deliberate addition over the Lua original: `chapter_num` is derived from
// the row title with the host's text.chapter_num, which the Lua version leaves
// unset. Chapter IDs are unaffected — only the sort key becomes real.

package weebcentral_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:slice"
import "core:strings"

kSite :: "https://weebcentral.com"
kHeaders :: `{"Accept":"text/html,application/xhtml+xml","Referer":"https://weebcentral.com/"}`

// ─── host helper wrappers ────────────────────────────────────────────────────
// In wasm the html helpers are stateless: html.parse only validates the markup
// and hands the same string back, so every find_* takes the markup as its first
// argument and no document handle is passed around.

find_text :: proc (markup, css: string) -> string { return abi.find_text(markup, css) }

find_attr :: proc (markup, css, attr: string) -> string { return abi.find_attr(markup, css, attr) }

find_list_text :: proc (markup, css: string) -> []string { return abi.find_list_text(markup, css) }

find_list_attr :: proc (markup, css, attr: string) -> []string { return abi.find_list_attr(markup, css, attr) }

// find_one returns the first capture of a pattern, "" when it does not match.
find_one :: proc (subject, pattern: string) -> string { return abi.find_one(subject, pattern) }

// find_all returns the first capture of every match, in document order.
find_all :: proc (subject, pattern: string) -> []string { return abi.find_all(subject, pattern) }

// sub replaces every match of pattern, mirroring Lua's string.gsub use here.
sub :: proc (subject, pattern, repl: string) -> string { return abi.sub(subject, pattern, repl) }

url_encode :: proc (s: string) -> string { return abi.url_encode(s) }

normalize_status :: proc (raw: string) -> string {
	return abi.host_str("text.normalize_status", raw)
}

chapter_num :: proc (title: string) -> f64 { return abi.chapter_num(title) }

// ─── wire types (ABI in) ─────────────────────────────────────────────────────

Search_Filter :: struct {
	Query:  string   `json:"query"`,
	Q:      string   `json:"q"`,
	Page:   int      `json:"page"`,
	Genres: []string `json:"genres"`,
}

// ─── wire types (ABI out) ────────────────────────────────────────────────────

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
}

Page_Out :: struct {
	Index: int    `json:"index"`,
	URL:   string `json:"url"`,
}

Genre_Out :: struct {
	Name: string `json:"name"`,
	Slug: string `json:"slug"`,
}

// ─── search ──────────────────────────────────────────────────────────────────

search_impl :: proc (arg: string) -> string {
	f: Search_Filter
	if json.unmarshal_string(arg, &f) != nil {
		return "[]"
	}
	query := strings.trim_space(f.Query)
	if query == "" {
		query = strings.trim_space(f.Q)
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 24

	sb := strings.builder_make()
	defer strings.builder_destroy(&sb)
	strings.write_string(&sb, kSite)
	strings.write_string(&sb, "/search/data?text=")
	strings.write_string(&sb, url_encode(query))
	fmt.sbprintf(&sb, "&limit=24&offset=%d", offset)
	if len(f.Genres) > 0 {
		// /search/data accepts repeated included_tag params; first genre wins.
		strings.write_string(&sb, "&included_tag=")
		strings.write_string(&sb, url_encode(f.Genres[0]))
	}

	body, ok := abi.http_body(strings.to_string(sb), kHeaders)
	if !ok {
		return "[]"
	}

	hrefs := find_list_attr(body, "article.bg-base-300 a[href*='/series/']", "href")
	imgs := find_list_attr(body, "article.bg-base-300 img", "src")
	ttl_hrefs := find_list_attr(body, "article.bg-base-300 a.line-clamp-1", "href")
	ttl_texts := find_list_text(body, "article.bg-base-300 a.line-clamp-1")

	// clean maps ULID -> the title rendered in the dedicated title anchor.
	clean := make(map[string]string)
	defer delete(clean)
	for th, j in ttl_hrefs {
		u := find_one(th, `/series/([^/?#]+)`)
		if u == "" || j >= len(ttl_texts) {
			continue
		}
		t := strings.trim_space(ttl_texts[j])
		if t != "" {
			clean[u] = t
		}
	}

	// Every card renders TWO <img> (a desktop and a mobile variant of the same
	// cover), so covers must NOT be zipped with the card list by position:
	// that put every cover on the PREVIOUS card's title. The cover URL
	// carries the series ULID, so a cover is matched to its card by identity.
	out := make([dynamic]Manga_Out, 0, len(hrefs))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)
	for href in hrefs {
		ulid := find_one(href, `/series/([^/?#]+)`)
		if ulid == "" || seen[ulid] {
			continue
		}
		seen[ulid] = true
		cover := cover_for(imgs, ulid)
		title, ok_t := clean[ulid]
		if !ok_t || title == "" {
			title = ulid
		}
		append(&out, Manga_Out{ID = ulid, Title = title, Cover_URL = cover})
	}
	return abi.to_json(out[:])
}

// cover_for returns the cover image belonging to one series, matched by the
// ULID the cover URL carries. Identity beats position here: the page renders
// more than one <img> per card, so a positional zip misplaces every cover.
//
// ponytail: the ULID in the cover URL is a property of the CDN's URL scheme.
// If it ever goes away the cover comes back empty and the title stays right —
// upgrade path: fall back to one img per picture anchor.
cover_for :: proc (imgs: []string, ulid: string) -> string {
	for src in imgs {
		if strings.contains(src, ulid) {
			return src
		}
	}
	return ""
}

// ─── detail ──────────────────────────────────────────────────────────────────

detail_impl :: proc (arg: string) -> string {
	ulid := strings.trim_space(strings.trim(arg, `"`))
	if ulid == "" {
		return "{}"
	}

	url := strings.concatenate({kSite, "/series/", ulid, "/auto"})
	body, ok := abi.http_body(url, kHeaders)
	if !ok {
		return abi.to_json(Manga_Out{ID = ulid, Title = "", Cover_URL = ""})
	}

	out := Manga_Out{ID = ulid}
	out.Title = find_text(body, "h1")
	out.Cover_URL = find_attr(body, "picture img", "src")
	out.Genres = find_list_text(body, "a[href*='included_tag']")

	// The info block is a list of <li> whose text is "<label>: <value>".
	lis := find_list_text(body, "li")
	status_raw := ""
	for li in lis {
		if out.Author == "" && strings.has_prefix(li, "Author") {
			out.Author = strip_label(li, "Author")
		}
		if status_raw == "" && strings.has_prefix(li, "Status") {
			status_raw = strip_label(li, "Status")
		}
	}
	out.Status = normalize_status(status_raw)

	out.Desc = find_text(body, "p.whitespace-pre-wrap")
	if out.Desc == "" {
		out.Desc = find_attr(body, "meta[name='description']", "content")
	}
	return abi.to_json(out)
}

// strip_label turns "Author(s): Someone" into "Someone" for the given label,
// dropping everything up to and including the first colon.
strip_label :: proc (line, label: string) -> string {
	rest := strings.trim_prefix(line, label)
	if i := strings.index_byte(rest, ':'); i >= 0 {
		rest = rest[i + 1:]
	}
	return strings.trim_space(rest)
}

// ─── chapters ────────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	ulid := strings.trim_space(strings.trim(arg, `"`))
	if ulid == "" {
		return "[]"
	}

	url := strings.concatenate({kSite, "/series/", ulid, "/full-chapter-list"})
	body, ok := abi.http_body(url, kHeaders)
	if !ok {
		return "[]"
	}

	hrefs := find_list_attr(body, "a[href*='/chapters/']", "href")
	row_texts := find_list_text(body, "a[href*='/chapters/']")
	times := find_list_attr(body, "a[href*='/chapters/'] time", "datetime")
	if len(times) == 0 {
		times = find_list_attr(body, "time", "datetime")
	}

	out := make([dynamic]Chapter_Out, 0, len(hrefs))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)
	for href, i in hrefs {
		chid := find_one(href, `/chapters/([^/?#]+)`)
		if chid == "" || seen[chid] {
			continue
		}
		seen[chid] = true

		raw := ""
		if i < len(row_texts) {
			raw = row_texts[i]
		}
		title := clean_chapter_title(raw)
		if title == "" {
			title = chid
		}
		released := ""
		if i < len(times) {
			released = times[i]
		}
		append(&out, Chapter_Out{
			ID          = chid,
			Manga_ID    = ulid,
			Title       = title,
			Chapter_Num = chapter_num(title),
			Released_At = released,
		})
	}

	// The fragment is newest-first; the reader wants ascending.
	n := len(out)
	for i in 0 ..< n / 2 {
		out[i], out[n - 1 - i] = out[n - 1 - i], out[i]
	}
	return abi.to_json(out[:])
}

// clean_chapter_title drops the "Last Read" marker and the trailing date the
// row text carries after the chapter label.
//
// (?s) is load-bearing: the row text wraps across newlines, and Lua's `.`
// matched them while RE2's does not. Without it the pattern silently matches
// nothing and the raw multi-line row leaks into the title.
clean_chapter_title :: proc (raw: string) -> string {
	t := strings.trim_space(raw)
	t = sub(t, `(?s)\s*Last Read.*$`, "")
	t = sub(t, `(?s)\s*\d{4}-\d{2}-\d{2}.*$`, "")
	return strings.trim_space(t)
}

// ─── pages ───────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	chid := strings.trim_space(strings.trim(arg, `"`))
	if chid == "" {
		return "[]"
	}

	url := strings.concatenate({kSite, "/chapters/", chid})
	body, ok := abi.http_body(url, kHeaders)
	if !ok {
		return "[]"
	}

	// The reader doc carries the batch count and an EMPTY image container.
	max_page := 1
	if mp := find_one(body, `max_page:\s*parseInt\(\s*['"](\d+)`); mp != "" {
		if n, conv_ok := parse_int(mp); conv_ok {
			max_page = n
		}
	} else if mp := find_one(body, `max_page[\s=]+(\d+)`); mp != "" {
		if n, conv_ok := parse_int(mp); conv_ok {
			max_page = n
		}
	}
	if max_page < 1 {
		max_page = 1
	}

	out := make([dynamic]Page_Out, 0, 32)
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)
	for page in 1 ..= max_page {
		sb := strings.builder_make()
		strings.write_string(&sb, kSite)
		strings.write_string(&sb, "/chapters/")
		strings.write_string(&sb, chid)
		fmt.sbprintf(&sb, "/images?is_prev=False&reading_style=long_strip&current_page=%d", page)
		fbody, fok := abi.http_body(strings.to_string(sb), kHeaders)
		strings.builder_destroy(&sb)
		if !fok {
			continue
		}
		for u in find_list_attr(fbody, "#chapter-images img", "src") {
			if u == "" || seen[u] {
				continue
			}
			seen[u] = true
			append(&out, Page_Out{Index = len(out), URL = u})
		}
	}
	return abi.to_json(out[:])
}

// parse_int is a tiny wrapper so pages_impl stays readable; the digits are
// already constrained by the regex that produced them.
parse_int :: proc (s: string) -> (int, bool) {
	n := 0
	if s == "" {
		return 0, false
	}
	for c in s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n * 10 + int(c - '0')
	}
	return n, true
}

// ─── genres ──────────────────────────────────────────────────────────────────

// Live /search markup carries checkbox ids with a badge prefix:
// <input id="tag-g-Action">. The g- prefix is stripped, and the hidden value
// mirrors (id="tag-g-Action-value") are skipped. name == slug == tag text.
genres_impl :: proc () -> string {
	body, ok := abi.http_body(strings.concatenate({kSite, "/search"}), kHeaders)
	if !ok {
		return "[]"
	}

	out := make([dynamic]Genre_Out, 0, 48)
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)
	for tag in find_all(body, `id="tag-([^"]+)"`) {
		if strings.has_suffix(tag, "-value") {
			continue
		}
		name := strings.trim_prefix(tag, "g-")
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		append(&out, Genre_Out{Name = name, Slug = name})
	}
	slice.sort_by(out[:], proc (a, b: Genre_Out) -> bool { return a.Name < b.Name })
	return abi.to_json(out[:])
}

// ─── exports ─────────────────────────────────────────────────────────────────

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
