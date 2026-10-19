// MangaUno (manga.uno) source plugin for goIsekai — Odin wasip1 reactor.
//
// 1:1 port of ../manga_uno_cpp/main.cpp (and ../manga_uno/main.go).
//
// manga.uno is a Next.js site with a JSON API:
//   search:   GET /api/search/advanced?title=...&genres=...&limit=24
//   genres:   GET /api/genres
//   detail:   GET /api/manga/<slug>       -> {"manga":{...},"chapters":[...]}
//   chapters: GET /api/manga/<slug>       -> top-level "chapters" (same response)
//   pages:    GET /api/chapter/<id>       -> {"chapter":{...},"pages":[...]}
//
// ID scheme: Manga.ID = slug; Chapter.ID = the site's numeric chapter id, so
// GetPageList can feed it straight back to /api/chapter/<id>.
//
// Covers and pages are served from the CDN origin (xz7.fstr-cdn.com), NOT
// manga.uno: manga.uno/covers/*.webp answers an HTML shell (text/html), which
// is what made every cover and reader image fail to load.
//
// Reuses abi/wasm_abi.odin (host ABI + HTTP/JSON helpers) and the host's own
// text.* helpers for url_encode / date_to_iso, so no text logic is duplicated.
package manga_uno_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:strconv"
import "core:strings"

kSite :: "https://manga.uno"
kAPI :: "https://manga.uno/api"
kJSONHeaders :: `{"Accept":"application/json","Referer":"https://manga.uno/"}`
kCDN :: "https://xz7.fstr-cdn.com"

// Resolve a path the way the site itself does: against the CDN origin, never
// manga.uno (which serves an HTML shell for /covers/*.webp).
cdn :: proc (raw: string) -> string {
	p := strings.trim_space(raw)
	switch {
	case p == "":
		return ""
	case strings.has_prefix(p, "http://"), strings.has_prefix(p, "https://"):
		return p
	case strings.has_prefix(p, "//"):
		return strings.concatenate({"https:", p})
	case p[0] == '/':
		return strings.concatenate({kCDN, p})
	}
	return strings.concatenate({kCDN, "/", p})
}

url_encode :: proc (s: string) -> string { return abi.url_encode(s) }

// parse_time converts to RFC3339 in UTC via the host; "" stays "".
parse_time :: proc (s: string) -> string {
	if s == "" {
		return ""
	}
	return abi.host_str("text.date_to_iso", s)
}

// normalize_status mirrors util.h's substring matching so this port behaves
// exactly like the C++ twin it was ported from.
normalize_status :: proc (raw: string) -> string {
	l := strings.to_lower(raw)
	switch {
	case strings.contains(l, "ongoing"), strings.contains(l, "on going"), strings.contains(l, "updating"):
		return "Ongoing"
	case strings.contains(l, "complete"), strings.contains(l, "finished"):
		return "Completed"
	case strings.contains(l, "hiatus"), strings.contains(l, "hold"), strings.contains(l, "pause"):
		return "Hiatus"
	case strings.contains(l, "cancel"), strings.contains(l, "drop"):
		return "Cancelled"
	}
	return ""
}

// ─── wire types (ABI in) ─────────────────────────────────────────────────────

Search_Filter :: struct {
	Query:  string   `json:"query"`,
	Page:   int      `json:"page"`,
	Genres: []string `json:"genres"`,
}

// ─── wire types (ABI out) ────────────────────────────────────────────────────

Manga_Out :: struct {
	ID:        string `json:"id"`,
	Title:     string `json:"title"`,
	Cover_URL: string `json:"cover_url"`,
	Author:    string `json:"author,omitempty"`,
	Desc:      string `json:"description,omitempty"`,
	Status:    string `json:"status,omitempty"`,
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

// ─── wire types (site responses) ─────────────────────────────────────────────

Search_Item :: struct {
	Slug:   string `json:"slug"`,
	Title:  string `json:"title"`,
	Cover:  string `json:"cover"`,
	Status: string `json:"status"`,
}

Search_Resp :: struct {
	Data: []Search_Item `json:"data"`,
}

Detail_Item :: struct {
	Slug:     string `json:"slug"`,
	Title:    string `json:"title"`,
	Synopsis: string `json:"synopsis"`,
	Status:   string `json:"status"`,
	Cover:    string `json:"cover"`,
	Author:   string `json:"author"`,
}

Detail_Resp :: struct {
	Manga: Detail_Item `json:"manga"`,
}

Chapter_Item :: struct {
	ID:             i64    `json:"id"`,
	Chapter_Number: string `json:"chapter_number"`,
	Title:          string `json:"title"`,
	Published_At:   string `json:"published_at"`,
	Created_At:     string `json:"created_at"`,
}

Chapters_Resp :: struct {
	Chapters: []Chapter_Item `json:"chapters"`,
}

Page_Chapter :: struct {
	ID:         i64 `json:"id"`,
	Manga_ID:   i64 `json:"manga_id"`,
	Page_Count: int `json:"page_count"`,
}

Pages_Resp :: struct {
	Pages:   []string     `json:"pages"`,
	Chapter: Page_Chapter `json:"chapter"`,
}

Genres_Resp :: struct {
	Genres: []string `json:"genres"`,
}

// ─── search ──────────────────────────────────────────────────────────────────

search_impl :: proc (arg: string) -> string {
	f: Search_Filter
	if json.unmarshal_string(arg, &f) != nil {
		return "[]"
	}

	url := strings.concatenate({kAPI, "/search/advanced?limit=24"})
	if f.Query != "" {
		url = strings.concatenate({url, "&title=", url_encode(f.Query)})
	}
	if len(f.Genres) > 0 && f.Genres[0] != "" {
		url = strings.concatenate({url, "&genres=", url_encode(f.Genres[0])})
	}

	body, ok := abi.http_body(url, kJSONHeaders)
	if !ok {
		return "[]"
	}
	resp: Search_Resp
	if json.unmarshal_string(body, &resp) != nil {
		return "[]"
	}

	out := make([dynamic]Manga_Out, 0, len(resp.Data))
	defer delete(out)
	for it in resp.Data {
		append(&out, Manga_Out {
			ID        = strings.trim_space(it.Slug),
			Title     = strings.trim_space(it.Title),
			Cover_URL = cdn(it.Cover),
			Status    = normalize_status(it.Status),
		})
	}
	return abi.to_json(out[:])
}

// decode_id unwraps the JSON string literal the host passes as every argument.
decode_id :: proc (arg: string) -> string {
	v: string
	if json.unmarshal_string(arg, &v) != nil {
		return ""
	}
	return v
}

// ─── detail ──────────────────────────────────────────────────────────────────

detail_impl :: proc (arg: string) -> string {
	slug := strings.trim_space(decode_id(arg))
	if slug == "" {
		return "{}"
	}

	body, ok := abi.http_body(
		strings.concatenate({kAPI, "/manga/", url_encode(slug)}),
		kJSONHeaders,
	)
	if !ok {
		return "{}"
	}
	resp: Detail_Resp
	if json.unmarshal_string(body, &resp) != nil {
		return "{}"
	}
	m0 := resp.Manga

	id := strings.trim_space(m0.Slug)
	if id == "" {
		id = slug
	}
	// cover_url is what the detail page's hero image reads; without it the image
	// is blank even though search shows a cover.
	return abi.to_json(Manga_Out {
		ID        = id,
		Title     = strings.trim_space(m0.Title),
		Desc      = strings.trim_space(m0.Synopsis),
		Status    = normalize_status(m0.Status),
		Cover_URL = cdn(m0.Cover),
		Author    = strings.trim_space(m0.Author),
	})
}

// ─── chapters ────────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	slug := strings.trim_space(decode_id(arg))
	if slug == "" {
		return "[]"
	}

	body, ok := abi.http_body(
		strings.concatenate({kAPI, "/manga/", url_encode(slug)}),
		kJSONHeaders,
	)
	if !ok {
		return "[]"
	}
	// The chapter list rides along on the detail response — /search/advanced
	// returns *other manga*, which is how this used to emit one fake chapter.
	d: Chapters_Resp
	if json.unmarshal_string(body, &d) != nil {
		return "[]"
	}

	page_url := strings.concatenate({kSite, "/", slug})
	out := make([dynamic]Chapter_Out, 0, len(d.Chapters))
	defer delete(out)
	for c in d.Chapters {
		number := strings.trim_space(c.Chapter_Number)
		title := strings.trim_space(c.Title)
		if title == "" {
			title = strings.concatenate({"Chapter ", number})
		}
		rel := c.Published_At
		if rel == "" {
			rel = c.Created_At
		}
		num, _ := strconv.parse_f64(number)
		append(&out, Chapter_Out {
			ID          = fmt.aprintf("%d", c.ID),
			Manga_ID    = slug,
			Title       = title,
			Chapter_Num = num,
			Released_At = parse_time(rel),
			URL         = page_url,
		})
	}
	return abi.to_json(out[:])
}

// ─── pages ───────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	ch_id := strings.trim_space(decode_id(arg))
	if ch_id == "" {
		return "[]"
	}

	body, ok := abi.http_body(
		strings.concatenate({kAPI, "/chapter/", url_encode(ch_id)}),
		kJSONHeaders,
	)
	if !ok {
		return "[]"
	}
	resp: Pages_Resp
	if json.unmarshal_string(body, &resp) != nil {
		return "[]"
	}

	out := make([dynamic]Page_Out, 0, len(resp.Pages))
	defer delete(out)
	// Page URLs sit at the TOP level of the response, not under "chapter".
	for p in resp.Pages {
		if p == "" {
			continue
		}
		append(&out, Page_Out{Index = len(out), URL = p})
	}
	if len(out) > 0 {
		return abi.to_json(out[:])
	}

	// pages omitted (uncached): those URLs all resolve to
	// /pages/<manga_id>/<chapter_id>/<n>.webp on the CDN, so synthesize them.
	// NOTE: Chapter must stay a value, not ^Page_Chapter — Odin's json.unmarshal
	// rejects pointer fields with Unsupported_Type_Error, which silently made
	// this whole function return [].
	c := resp.Chapter
	if c.Manga_ID > 0 && c.ID > 0 && c.Page_Count > 0 {
		for i in 1 ..= c.Page_Count {
			append(&out, Page_Out {
				Index = len(out),
				URL = fmt.aprintf(
					"%s/pages/%d/%d/%d.webp",
					kCDN,
					c.Manga_ID,
					c.ID,
					i,
				),
			})
		}
	}
	return abi.to_json(out[:])
}

// ─── genres ──────────────────────────────────────────────────────────────────

genres_impl :: proc () -> string {
	body, ok := abi.http_body(strings.concatenate({kAPI, "/genres"}), kJSONHeaders)
	if !ok {
		return "[]"
	}
	resp: Genres_Resp
	if json.unmarshal_string(body, &resp) != nil {
		return "[]"
	}

	out := make([dynamic]Genre_Out, 0, len(resp.Genres))
	defer delete(out)
	for n in resp.Genres {
		if n == "" {
			continue
		}
		append(&out, Genre_Out{Name = n, Slug = n})
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

@(export)
GetGenres :: proc "c" (ptr: u32, length: u32) -> u64 {
	context = runtime.default_context()
	return abi.reply_json(genres_impl())
}
