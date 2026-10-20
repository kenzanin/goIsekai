// VioletManga (violetmanga.com) source plugin for goIsekai — Odin wasip1 reactor.
//
// Behavioural port of examples/plugins/lua/violetmanga/main.lua, which stays as
// the reference implementation: the Lua original is never deleted, only
// app_data/plugins/violetmanga/ is swapped.
//
// WordPress "bixbox/listupd" theme, server-rendered HTML throughout. Despite the
// Lua header mentioning host.browser.evaluate, that plugin never calls it — the
// reader HTML already carries the page list, so every endpoint is plain HTTP:
//
//	search   GET /?s=<q>[&paged=N]           -> .bsx cards
//	detail   GET /comics/<slug>/              -> h1.entry-title etc.
//	chapters GET /comics/<slug>/              -> same doc, #chapterlist a[href]
//	pages    GET <chapter-id>                 -> "images":[...] inside ts_reader.run()
//	genres   (none — the site exposes no stable genre index)
//
// Three behaviours are load-bearing and copied from the Lua source, not guessed:
//
//  1. The manga id is the BARE SLUG (`/comics/<slug>/`), not the full URL: a full
//     URL 404s the /view/manga route.
//  2. Chapter ids are RELATIVE PATHS (`/slug-chapter-N/`). The reader route
//     URL-encodes them, so storing the absolute URL breaks it.
//  3. The reader page hides the page list inside `ts_reader.run({...})`, not in
//     SSR markup, and escapes the slashes (`https:\/\/...`). The URLs therefore
//     come from the "images" array and are un-escaped after extraction.
//
// One deliberate addition over the Lua original: `chapter_num` is derived from
// the chapter slug. Lua emits no chapter_num at all, so every chapter sorts as
// 0; ids and titles are unaffected — only the sort key becomes real.
package violetmanga_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:strconv"
import "core:strings"

kSite :: "https://violetmanga.com"

// The Lua version sends only Accept; the UA is kept so a bare curl-shaped
// request never gets a challenge page.
kHeaders :: `{"Accept":"text/html,application/xhtml+xml","User-Agent":"Mozilla/5.0 (compatible; goIsekai-plugin/1.0)"}`

// ─── wire types (ABI in) ─────────────────────────────────────────────────────

// Q is the short alias the host used to send before it settled on "query".
Search_Filter :: struct {
	Query: string `json:"query"`,
	Q:     string `json:"q"`,
	Page:  int    `json:"page"`,
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
	URL:         string `json:"url"`,
}

Page_Out :: struct {
	Index: int    `json:"index"`,
	URL:   string `json:"url"`,
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// decode_id unwraps the JSON the host passes the single-string ABI: a bare
// quoted id ("my-slug") or an object ({"id":"my-slug"}) both end up as the slug.
decode_id :: proc (arg: string) -> string {
	v: string
	if json.unmarshal_string(arg, &v) != nil {
		v = strings.trim_space(arg)
	}
	return strings.trim_space(v)
}

// fetch loads a page through the host proxy. The Lua version returns
// host.html.parse(body) as a document handle; in wasm the html helpers are
// stateless, so the markup itself is the handle and html.parse would only
// re-validate it.
fetch :: proc (url: string) -> (string, bool) {
	return abi.http_body(url, kHeaders)
}

// slug_from_href pulls the bare slug out of "/comics/<slug>/".
slug_from_href :: proc (href: string) -> string {
	return abi.find_one(href, `/comics/([^/?#]+)`)
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
	if query == "" {
		return "[]"
	}
	page := f.Page
	if page < 1 {
		page = 1
	}

	sb := strings.builder_make()
	defer strings.builder_destroy(&sb)
	strings.write_string(&sb, kSite)
	strings.write_string(&sb, "/?s=")
	strings.write_string(&sb, abi.url_encode(query))
	if page > 1 {
		strings.write_string(&sb, "&paged=")
		strings.write_int(&sb, page)
	}

	body, ok := fetch(strings.to_string(sb))
	if !ok {
		return "[]"
	}

	// Cards are `.bsx > a`; the title lives in `.tt` and the cover in a nested
	// img. hrefs and covers are one-per-card, so they zip safely; titles can be
	// missing, hence the length guard.
	hrefs := abi.find_list_attr(body, ".bsx > a", "href")
	titles := abi.find_list_text(body, ".bsx .tt")
	covers := abi.find_list_attr(body, ".bsx > a img", "src")

	out := make([dynamic]Manga_Out, 0, len(hrefs))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)
	for href, i in hrefs {
		slug := slug_from_href(href)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		title := ""
		if i < len(titles) {
			title = strings.trim_space(titles[i])
		}
		cover := ""
		if i < len(covers) {
			cover = covers[i]
		}
		append(&out, Manga_Out{ID = slug, Title = title, Cover_URL = cover})
	}
	return abi.to_json(out[:])
}

// ─── detail ──────────────────────────────────────────────────────────────────

detail_impl :: proc (arg: string) -> string {
	slug := decode_id(arg)
	if slug == "" {
		return "{}"
	}

	body, ok := fetch(strings.concatenate({kSite, "/comics/", slug, "/"}))
	if !ok {
		// Failure shape for an object export: bare {id}. The host does not cache
		// it, so the next call retries; an [] here would not decode into a struct.
		return abi.to_json(Manga_Out{ID = slug})
	}

	genres := abi.find_list_text(body, ".genres-container .mgen a")
	clean := make([dynamic]string, 0, len(genres))
	defer delete(clean)
	for g in genres {
		if n := strings.trim_space(g); n != "" {
			append(&clean, n)
		}
	}

	// The site credits only the uploader (no mangaka field), but that uploader
	// line is what the Lua original exposes as `author`, so it is read the same
	// way rather than dropped.
	return abi.to_json(Manga_Out{
		ID        = slug,
		Title     = abi.find_text(body, "h1.entry-title"),
		Cover_URL = abi.find_attr(body, ".thumb img", "src"),
		Author    = abi.find_text(body, ".imptdt span.author i"),
		Desc      = abi.find_text(body, ".summary .entry-content"),
		Status    = abi.host_str("text.normalize_status", abi.find_text(body, ".info-half .status i")),
		Genres    = clean[:],
	})
}

// ─── chapters ────────────────────────────────────────────────────────────────

// chapter_path normalises a chapter href to the relative path the reader route
// expects: absolute URLs are cut to their path, relative ones pass through.
chapter_path :: proc (href: string) -> string {
	p := abi.find_one(href, `violetmanga\.com(/[^?#]*)`)
	if p == "" {
		return href
	}
	return p
}

chapters_impl :: proc (arg: string) -> string {
	slug := decode_id(arg)
	if slug == "" {
		return "[]"
	}

	body, ok := fetch(strings.concatenate({kSite, "/comics/", slug, "/"}))
	if !ok {
		return "[]"
	}

	// Free chapters are `a[href]`; coin-gated ones render as bare anchors with
	// no href, so the attribute selector skips them for free.
	hrefs := abi.find_list_attr(body, "#chapterlist a[href]", "href")

	out := make([dynamic]Chapter_Out, 0, len(hrefs))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)
	for href in hrefs {
		path := chapter_path(href)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true

		// The number drives both the title and the sort key. A slug that does
		// not carry one still becomes a chapter (title falls back to the path).
		num_raw := abi.find_one(path, `-chapter-(.*?)/?$`)
		title := ""
		num: f64
		if num_raw != "" {
			title = strings.concatenate({"Chapter ", num_raw})
			if n, parsed := strconv.parse_f64(strings.trim_space(num_raw)); parsed {
				num = n
			}
		}
		if title == "" {
			title = strings.trim_space(path)
		}

		append(&out, Chapter_Out{
			ID          = path,
			Manga_ID    = slug,
			Title       = title,
			Chapter_Num = num,
			URL         = path,
		})
	}
	return abi.to_json(out[:])
}

// ─── pages ───────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	chapter := decode_id(arg)
	if chapter == "" {
		return "[]"
	}
	url := chapter
	if !strings.has_prefix(url, "http://") && !strings.has_prefix(url, "https://") {
		url = strings.concatenate({kSite, chapter})
	}

	body, ok := fetch(url)
	if !ok {
		return "[]"
	}

	// ts_reader.run({"sources":[{"images":[...]}]}) — the array is the only
	// place the page URLs exist; (?s) because the config wraps across lines.
	images := abi.find_one(body, `(?s)"images"\s*:\s*\[(.*?)\]`)
	if images == "" {
		return "[]"
	}

	// Slashes are escaped in the config (`https:\/\/...`), hence `\\?` on each
	// separator. The pattern has no capture group, so find_all returns each
	// whole match rather than a row.
	raws := abi.find_all(images, `https?:\\?/\\?/[^"']+`)

	out := make([dynamic]Page_Out, 0, len(raws))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)
	for raw in raws {
		clean := abi.sub(raw, `\\/`, "/")
		if clean == "" || seen[clean] {
			continue
		}
		seen[clean] = true
		append(&out, Page_Out{Index = len(out), URL = clean})
	}
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
