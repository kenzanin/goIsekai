// MangaKatana (mangakatana.com) source plugin for goIsekai — Odin wasip1 reactor.
//
// Behavioural port of examples/plugins/lua/mangakatana/main.lua, which stays the
// reference implementation: the Lua original is never deleted, only
// app_data/plugins/mangakatana/ is swapped to the wasm build.
//
//   search    GET /?search=<q>&search_by=book_name   (page N: /page/N?search=...)
//             GET /genre/<slug>[/page/N]             (genre filter)
//   detail    GET /manga/<slug>
//   chapters  GET /manga/<slug>            (scraped from the chapter table)
//   pages     GET /manga/<slug>/<cXXX>     (URLs lifted out of a JS array)
//   genres    static list (27 entries, the site's nav vocabulary)
//
// Ids follow the Lua contract exactly, because the host pastes them into URL
// paths and a bare path segment cannot hold a slash:
//
//   manga id    "<slug>"                e.g. "naruto.1205"
//   chapter id  "<slug>:<cXXX>"         e.g. "naruto.1205:c700"
//
// Load-bearing behaviours, verified against the live site:
//
// 1. The reader page ships NO usable image URLs in its markup — every <img>
//    carries data-src="#" and JavaScript copies the real URLs into them. The
//    real list lives in a JS array literal (`var thzq=[...]`). We take the array
//    with the most http URLs on the page and ignore its name, so a rename of the
//    obfuscated variable cannot break us.
// 2. Search lists stay index-aligned: every card has one title link and one
//    cover <img> (results without a cover use /imgs/no-cover.png).
// 3. `#related` is nested inside `#single_book` and repeats the search-card
//    markup, so every detail lookup is scoped to stay off it.
//
// Deliberate delta over the Lua original:
//
// - The description is read as the `<p>` inside `div.summary` with `<br>` turned
//   into newlines. The Lua reaches for `html.xpath_list_text(.../p/text())`, and
//   no xpath helper is exposed to wasm; both forms agree on the live markup
//   (sampled pages carry zero `<br>` in that paragraph).
package mangakatana_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:strconv"
import "core:strings"

kSite :: "https://mangakatana.com"
kHeaders :: `{"User-Agent":"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36","Accept":"text/html,application/xhtml+xml"}`

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

// ─── id / URL contract ──────────────────────────────────────────────────────

decode_id :: proc (arg: string) -> string {
	v: string
	if json.unmarshal_string(arg, &v) != nil {
		v = strings.trim_space(arg)
	}
	return strings.trim_space(v)
}

is_absolute :: proc (url: string) -> bool {
	return strings.has_prefix(url, "http://") || strings.has_prefix(url, "https://")
}

// slug_of reduces a full or partial manga URL to the bare slug; a bare slug is
// returned unchanged.
slug_of :: proc (id: string) -> string {
	if v := abi.find_one(id, `/manga/([^/?#]+)`); v != "" {
		return v
	}
	return id
}

// chapter_url turns "slug:cXXX" back into the chapter's absolute URL. Absolute
// input passes through so a stub host can be targeted in tests.
chapter_url :: proc (id: string) -> string {
	if is_absolute(id) {
		return id
	}
	parts := strings.split_n(id, ":", 2)
	defer delete(parts)
	if len(parts) < 2 {
		return id
	}
	return strings.concatenate({kSite, "/manga/", parts[0], "/", parts[1]})
}

// ─── search ─────────────────────────────────────────────────────────────────

search_impl :: proc (arg: string) -> string {
	f: Search_Filter
	if json.unmarshal_string(arg, &f) != nil {
		return "[]"
	}
	page := f.Page
	if page < 1 {
		page = 1
	}

	url: string
	if len(f.Genres) > 0 {
		// Genre archive; page N is /page/N past the slug.
		base := strings.concatenate({kSite, "/genre/", abi.url_encode(strings.trim_space(f.Genres[0]))})
		if page > 1 {
			url = strings.concatenate({base, "/page/", fmt.aprintf("%d", page)})
		} else {
			url = base
		}
	} else {
		q := abi.url_encode(strings.trim_space(f.Query))
		if page > 1 {
			url = strings.concatenate({kSite, "/page/", fmt.aprintf("%d", page), "?search=", q, "&search_by=book_name"})
		} else {
			url = strings.concatenate({kSite, "/?search=", q, "&search_by=book_name"})
		}
	}

	body, ok := abi.http_body(url, kHeaders)
	if !ok {
		return "[]"
	}

	// Every card carries a title link and one cover <img>, so the three lists are
	// index-aligned — the same assumption the Lua makes.
	hrefs := abi.find_list_attr(body, `#book_list .item h3.title a`, "href")
	titles := abi.find_list_text(body, `#book_list .item h3.title a`)
	covers := abi.find_list_attr(body, `#book_list .item .media img`, "src")

	out := make([dynamic]Manga_Out, 0, len(hrefs))
	defer delete(out)
	for href, i in hrefs {
		title, cover: string
		if i < len(titles) {
			title = titles[i]
		}
		if i < len(covers) {
			cover = covers[i]
		}
		append(&out, Manga_Out{
			ID = slug_of(href),
			Title = abi.sanitize_title(title),
			Cover_URL = strings.trim_space(cover),
		})
	}
	return abi.to_json(out[:])
}

// ─── detail ─────────────────────────────────────────────────────────────────

detail_impl :: proc (arg: string) -> string {
	slug := slug_of(decode_id(arg))
	if slug == "" {
		return "{}"
	}

	body, ok := abi.http_body(strings.concatenate({kSite, "/manga/", slug}), kHeaders)
	if !ok {
		// Failure shape for an object-returning export: bare {id}. The host does
		// not cache it, so the next call retries; an [] would not decode.
		return abi.to_json(Manga_Out{ID = slug})
	}

	// Genre anchors live in `div.genres` inside #single_book.
	raw := abi.find_list_text(body, `#single_book .genres a`)
	genres := make([dynamic]string, 0, len(raw))
	defer delete(genres)
	for g in raw {
		if n := strings.trim_space(g); n != "" {
			append(&genres, n)
		}
	}

	// The description is a single <p>; <br> tags are its line breaks.
	desc: string
	if pblock := abi.find_one(body, `(?s)class="summary">.*?<p>(.*?)</p>`); pblock != "" {
		with_breaks := abi.sub(pblock, `(?i)<br\s*/?>`, "\n")
		desc = strings.trim_space(abi.host_str("text.strip_html", with_breaks))
	}

	return abi.to_json(Manga_Out{
		ID = slug,
		Title = abi.sanitize_title(abi.find_text(body, `h1.heading`)),
		Author = strings.trim_space(abi.find_text(body, `#single_book .authors a.author`)),
		Desc = desc,
		Cover_URL = strings.trim_space(abi.find_attr(body, `#single_book .cover img`, "src")),
		Status = abi.host_str("text.normalize_status", strings.trim_space(abi.find_text(body, `#single_book div.value.status`))),
		Genres = genres[:],
	})
}

// ─── chapters ───────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	slug := slug_of(decode_id(arg))
	if slug == "" {
		return "[]"
	}

	body, ok := abi.http_body(strings.concatenate({kSite, "/manga/", slug}), kHeaders)
	if !ok {
		return "[]"
	}

	// The table is newest-first already, which is the ABI convention.
	selector :: `div.chapters table tbody tr td div.chapter a`
	urls := abi.find_list_attr(body, selector, "href")
	titles := abi.find_list_text(body, selector)

	out := make([dynamic]Chapter_Out, 0, len(urls))
	defer delete(out)
	for url, i in urls {
		// .../c247, .../c230.5 and .../c0-v2 all keep their trailing segment.
		tail := abi.find_one(url, `/([^/]+)$`)
		if tail == "" {
			tail = url
		}
		title: string
		if i < len(titles) {
			title = titles[i]
		}
		// The number is the tail of the href, not the title text.
		num: f64 = 0
		if raw := abi.find_one(url, `/c([\d.]+)`); raw != "" {
			if v, parsed := strconv.parse_f64(raw); parsed {
				num = v
			}
		}
		append(&out, Chapter_Out{
			ID = strings.concatenate({slug, ":", tail}),
			Manga_ID = slug,
			Title = abi.sanitize_title(title),
			Chapter_Num = num,
			URL = url,
		})
	}
	return abi.to_json(out[:])
}

// ─── pages ──────────────────────────────────────────────────────────────────

// page_urls lifts the image list out of the reader page's JS. The array variable
// is deliberately obfuscated and renamed periodically, so we take whichever
// bracket literal on the page holds the most http URLs — same strategy as the
// Lua, which keeps the rename from breaking the plugin.
page_urls :: proc (markup: string) -> []string {
	arrays := abi.find_all(markup, `(?s)\[(.*?)\]`)
	best: []string
	for a in arrays {
		urls := abi.find_all(a, `'(https?://[^']+)'`)
		if len(urls) > len(best) {
			best = urls
		}
	}
	return best
}

pages_impl :: proc (arg: string) -> string {
	url := chapter_url(decode_id(arg))
	if url == "" {
		return "[]"
	}

	body, ok := abi.http_body(url, kHeaders)
	if !ok {
		return "[]"
	}

	urls := page_urls(body)
	out := make([dynamic]Page_Out, 0, len(urls))
	defer delete(out)
	for u, i in urls {
		// The Lua source numbers pages from 1 (`ipairs` index), not from 0, so the
		// port keeps that to stay byte-identical with the reference.
		append(&out, Page_Out{Index = i + 1, URL = u})
	}
	return abi.to_json(out[:])
}

// ─── genres (optional export) ───────────────────────────────────────────────

// Slugs as they appear in /genre/<slug>, taken from the site nav.
genres_impl :: proc () -> string {
	out := []Genre_Out{
		{Name = "Action", Slug = "action"},
		{Name = "Adventure", Slug = "adventure"},
		{Name = "Comedy", Slug = "comedy"},
		{Name = "Drama", Slug = "drama"},
		{Name = "Fantasy", Slug = "fantasy"},
		{Name = "Harem", Slug = "harem"},
		{Name = "Historical", Slug = "historical"},
		{Name = "Horror", Slug = "horror"},
		{Name = "Isekai", Slug = "isekai"},
		{Name = "Josei", Slug = "josei"},
		{Name = "Martial Arts", Slug = "martial-arts"},
		{Name = "Mecha", Slug = "mecha"},
		{Name = "Mystery", Slug = "mystery"},
		{Name = "Psychological", Slug = "psychological"},
		{Name = "Reincarnation", Slug = "reincarnation"},
		{Name = "Romance", Slug = "romance"},
		{Name = "School Life", Slug = "school-life"},
		{Name = "Sci-Fi", Slug = "sci-fi"},
		{Name = "Seinen", Slug = "seinen"},
		{Name = "Shoujo", Slug = "shoujo"},
		{Name = "Shounen", Slug = "shounen"},
		{Name = "Slice of Life", Slug = "slice-of-life"},
		{Name = "Sports", Slug = "sports"},
		{Name = "Super Power", Slug = "super-power"},
		{Name = "Supernatural", Slug = "supernatural"},
		{Name = "Tragedy", Slug = "tragedy"},
	}
	return abi.to_json(out)
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
