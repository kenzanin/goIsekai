// Mangataro (mangataro.org) source plugin for goIsekai — Odin wasip1 reactor.
//
// Behavioural port of examples/plugins/lua/mangataro/main.lua, which stays the
// reference implementation: the Lua original is never deleted, only
// app_data/plugins/mangataro/ is swapped to the wasm build.
//
// WordPress (mangapeak theme). The Lua header claims chapters and pages need
// host.browser.evaluate — a scroll-poll harvest — but the plugin never calls it:
// both are plain JSON endpoints, so this port is pure HTTP too.
//
//   search    GET /?s=<q>  (or /tag/<slug>/ when a genre filter is set)
//   detail    GET /manga/<slug>
//   chapters  GET /auth/manga-chapters?manga_id=<numeric>&_t=<token>&_ts=<unix>
//   pages     GET /auth/chapter-content?chapter_id=<numeric>
//   genres    static list (the site's tag vocabulary)
//
// Three behaviours are load-bearing, copied from the Lua source rather than
// guessed:
//
// 1. Manga id is the BARE SLUG. Full URLs 404 the host's /view/manga route.
// 2. The chapter endpoint is protected by a client-side token:
//    `md5(unix_ts .. "mng_ch_" .. YYYYMMDDHH)[:16]`, where YYYYMMDDHH is the
//    current UTC hour. Reproducing it here avoids a browser round-trip. The hour
//    is derived from `text.date_to_iso` (which formats UTC) rather than from a
//    local date call, exactly as the Lua does.
// 3. Chapter id is the plain numeric string from the API (`"704814"`), which is
//    also the `chapter_id` the page endpoint wants. Reader URLs are accepted too
//    and reduced to their trailing digits.
//
// Deliberate deltas over the Lua original:
//
// - `released_at` is filled from the API's `date` field. It is a coarse relative
//   phrase ("2 years ago"), but that still beats the zero time Lua emits.
// - `first_reader_url` is NOT emitted: pkg/types.Manga has no such field, so the
//   host dropped it. Cross-checked against the A/B payload.
package mangataro_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:strings"
import "core:time"

kSite :: "https://mangataro.org"
kUA :: "Mozilla/5.0 (compatible; goIsekai-plugin/1.0)"
kHtmlHeaders :: `{"User-Agent":"Mozilla/5.0 (compatible; goIsekai-plugin/1.0)","Accept":"text/html,application/xhtml+xml"}`
kJsonHeaders :: `{"User-Agent":"Mozilla/5.0 (compatible; goIsekai-plugin/1.0)","Accept":"application/json, text/plain, */*","Accept-Language":"en-US,en;q=0.9","Origin":"https://mangataro.org","sec-fetch-dest":"empty","sec-fetch-mode":"cors","sec-fetch-site":"same-origin"}`

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

Chapter_Item :: struct {
	ID      : string `json:"id"`,
	Chapter : string `json:"chapter"`,
	Title   : string `json:"title"`,
	Date    : string `json:"date"`,
	URL     : string `json:"url"`,
}

Chapters_Resp :: struct {
	Success  : bool           `json:"success"`,
	Chapters : []Chapter_Item `json:"chapters"`,
}

Content_Resp :: struct {
	Images: []string `json:"images"`,
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

absolute :: proc (url: string) -> string {
	if url == "" || url == "null" {
		return ""
	}
	if strings.has_prefix(url, "http://") || strings.has_prefix(url, "https://") {
		return url
	}
	if strings.has_prefix(url, "/") {
		return strings.concatenate({kSite, url})
	}
	return strings.concatenate({kSite, "/manga/", url})
}

// slug_from_href keeps the bare slug out of "/manga/<slug>[/]".
slug_from_href :: proc (href: string) -> string {
	return abi.find_one(href, `/manga/([^/?#]+)/?\s*$`)
}

// trailing_digits drops everything before the last run of digits, which is the
// `chapter_id` the content endpoint expects (".../ch200-9355" -> "9355").
trailing_digits :: proc (id: string) -> string {
	v := abi.find_one(id, `([0-9]+)\s*$`)
	if v != "" {
		return v
	}
	return abi.find_one(id, `([0-9]+)`)
}

// chapter_token rebuilds the site's generateToken(): the first 16 hex chars of
// md5(ts .. "mng_ch_" .. YYYYMMDDHH), with YYYYMMDDHH the UTC hour.
chapter_token :: proc (ts: string) -> string {
	iso := abi.host_str("text.date_to_iso", ts)
	// date_to_iso formats UTC, so its digits are the UTC wall clock. Strip the
	// separators and keep YYYYMMDDHH.
	digits := abi.sub(iso, `\D`, "")
	hour := digits
	if len(hour) > 10 {
		hour = hour[:10]
	}
	sum := abi.host_str("crypto.md5_hex", strings.concatenate({ts, "mng_ch_", hour}))
	if len(sum) > 16 {
		return sum[:16]
	}
	return sum
}

// ─── search ─────────────────────────────────────────────────────────────────

search_impl :: proc (arg: string) -> string {
	f: Search_Filter
	if json.unmarshal_string(arg, &f) != nil {
		return "[]"
	}

	// A genre filter browses the WordPress tag archive, which reuses the same
	// card template as search results.
	url: string
	if len(f.Genres) > 0 {
		url = strings.concatenate({kSite, "/tag/", abi.url_encode(f.Genres[0]), "/"})
	} else {
		url = strings.concatenate({kSite, "/?s=", abi.url_encode(strings.trim_space(f.Query))})
	}

	body, ok := abi.http_body(url, kHtmlHeaders)
	if !ok {
		return "[]"
	}

	// Each card is an <a href="/manga/<slug>"> wrapping an <img> whose alt is the
	// title and whose src is the cover. The three lists are paired by position,
	// same as the Lua source.
	card :: "a[href*='/manga/']"
	hrefs := abi.find_list_attr(body, card, "href")
	titles := abi.find_list_attr(body, strings.concatenate({card, " img"}), "alt")
	covers := abi.find_list_attr(body, strings.concatenate({card, " img"}), "src")

	out := make([dynamic]Manga_Out, 0, len(hrefs))
	defer delete(out)
	for href, i in hrefs {
		slug := slug_from_href(href)
		if slug == "" {
			continue
		}
		title, cover: string
		if i < len(titles) {
			title = titles[i]
		}
		if i < len(covers) {
			cover = covers[i]
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

	body, ok := abi.http_body(absolute(slug), kHtmlHeaders)
	if !ok {
		// Failure shape for an object-returning export: bare {id}. The host does
		// not cache it, so the next call retries; an [] would not decode.
		return abi.to_json(Manga_Out{ID = slug})
	}

	// Genre links point at absolute /tag/ URLs.
	raw_genres := abi.find_list_text(body, `a[href*='/tag/']`)
	genres := make([dynamic]string, 0, len(raw_genres))
	defer delete(genres)
	for g in raw_genres {
		if n := strings.trim_space(g); n != "" {
			append(&genres, n)
		}
	}

	status_raw := abi.find_attr(body, `[data-status]`, "data-status")
	if status_raw == "" {
		status_raw = abi.find_text(body, `.comic-status, .status`)
	}

	desc := abi.find_attr(body, `meta[name='description']`, "content")
	if desc == "" {
		desc = abi.find_text(body, `.entry-content, .description-summary`)
	}

	return abi.to_json(Manga_Out {
		ID = slug,
		// Two <h1>s on the page (site title + manga title); the manga one carries
		// line-clamp-2.
		Title = strings.trim_space(abi.find_text(body, `h1.line-clamp-2`)),
		Cover_URL = abi.find_attr(body, `meta[property='og:image']`, "content"),
		Desc = desc,
		Status = abi.host_str("text.normalize_status", strings.trim_space(status_raw)),
		Genres = genres[:],
	})
}

// ─── chapters ───────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	slug := decode_id(arg)
	if slug == "" {
		return "[]"
	}

	// The AJAX endpoint keys off the numeric id, which only the detail page
	// carries.
	page, ok := abi.http_body(absolute(slug), kHtmlHeaders)
	if !ok {
		return "[]"
	}
	numeric_id := abi.find_attr(page, `[data-manga-id]`, "data-manga-id")
	if numeric_id == "" {
		return "[]"
	}

	// Wall clock: the token is timestamped, so a frozen or zero clock would make
	// every request fail. Verified live on wasi.
	sec := time.time_to_unix(time.now())
	ts := fmt.aprintf("%d", sec)
	token := chapter_token(ts)

	api := strings.concatenate({
		kSite, "/auth/manga-chapters?manga_id=", numeric_id,
		"&_t=", token, "&_ts=", ts,
	})
	// The endpoint checks Origin/Referer, so it needs its own header set.
	api_body, api_ok := abi.http_body(api, kJsonHeaders)
	if !api_ok {
		return "[]"
	}
	resp: Chapters_Resp
	if json.unmarshal_string(api_body, &resp) != nil || len(resp.Chapters) == 0 {
		return "[]"
	}

	out := make([dynamic]Chapter_Out, 0, len(resp.Chapters))
	defer delete(out)
	for ch in resp.Chapters {
		if ch.ID == "" {
			continue
		}
		append(&out, Chapter_Out {
			ID = ch.ID,
			Manga_ID = slug,
			Title = ch.Title,
			Chapter_Num = abi.chapter_num(ch.Chapter),
			Released_At = abi.host_str("text.date_to_iso", ch.Date),
			URL = ch.URL,
		})
	}
	return abi.to_json(out[:])
}

// ─── pages ──────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	// Accepts a bare id, an object with id/url, or a reader URL — same shapes the
	// Lua source tolerates.
	id := decode_id(arg)
	num := trailing_digits(id)
	if num == "" {
		return "[]"
	}

	// Chapter-content returns the full deterministic image list: no token, no
	// browser scroll-harvest.
	body, ok := abi.http_body(strings.concatenate({kSite, "/auth/chapter-content?chapter_id=", num}), kHtmlHeaders)
	if !ok {
		return "[]"
	}
	resp: Content_Resp
	if json.unmarshal_string(body, &resp) != nil || len(resp.Images) == 0 {
		return "[]"
	}

	out := make([dynamic]Page_Out, 0, len(resp.Images))
	defer delete(out)
	for url, i in resp.Images {
		if url == "" {
			continue
		}
		append(&out, Page_Out{Index = i, URL = url})
	}
	return abi.to_json(out[:])
}

// ─── genres (optional export) ───────────────────────────────────────────────

// The site's tag vocabulary. Kept static like the Lua source: the tag page lists
// categories and site-only groupings, so scraping it would advertise filters the
// archive URLs do not uniformly accept.
genres_impl :: proc () -> string {
	out := []Genre_Out{
		{Name = "Action", Slug = "action"},
		{Name = "Adaptation", Slug = "adaptation"},
		{Name = "Adventure", Slug = "adventure"},
		{Name = "Award Winning", Slug = "award-winning"},
		{Name = "Ecchi", Slug = "ecchi"},
		{Name = "Isekai", Slug = "isekai"},
		{Name = "Martial Arts", Slug = "martial-arts"},
		{Name = "Mecha", Slug = "mecha"},
		{Name = "Mythology", Slug = "mythology"},
		{Name = "Reincarnation", Slug = "reincarnation"},
		{Name = "Romance", Slug = "romance"},
		{Name = "Shounen", Slug = "shounen"},
		{Name = "Slice of Life", Slug = "slice-of-life"},
		{Name = "Team Sports", Slug = "team-sports"},
		{Name = "Video Game", Slug = "video-game"},
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
