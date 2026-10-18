// Atsumaru (atsu.moe) source plugin for goIsekai — Odin wasip1 reactor.
//
// 1:1 behavioural port of examples/plugins/lua/atsumaru/main.lua, which is the
// reference implementation for this source. Site API:
//   search   GET /collections/manga/documents/search  (Typesense)
//   chapters GET /api/manga/allChapters?mangaId=<id>  -> {chapters:[...]}
//   detail   GET /api/manga/page?id=<id>              -> {mangaPage:{...}}
//   pages    GET /api/read/chapter?mangaId=<id>&chapterId=<id>
//
// Three details are load-bearing and were taken from the Lua plugin, not guessed:
//
//  1. The search needs the site's own filter, not just q=:
//       hidden:!=true && views:>0 && medium:!=[`Novel`]
//     Without it the index returns drafts and novel entries.
//
//  2. /api/read/chapter needs BOTH ids (chapterId alone answers 400 ZodError)
//     while the host passes GetPageList a single string, so Chapter.ID is the
//     composite "<mangaId>:<chapterId>" and pages_impl splits it again.
//
//  3. Image paths arrive prefixed ("/static/posters/x.avif" from search) and
//     bare ("posters/x.avif" from detail). Both must become
//     cdn.atsu.moe/static/<path>; the site origin does not serve them at all.
//
// Network and text work go through abi/ (host_call): http_body plus the host's
// text.url_encode / text.strip_html / text.chapter_num / text.normalize_status,
// so no logic here duplicates what every other plugin kind already gets.
package atsumaru_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:slice"
import "core:strings"

kSite :: "https://atsu.moe"
kCDN :: "https://cdn.atsu.moe"
kJSONHeaders :: `{"accept":"application/json"}`

// image_url resolves an asset path against the CDN. The site origin answers
// 410 Gone for /static/*, and the two payloads disagree on the prefix, so the
// prefix is stripped here and re-added in one canonical place.
image_url :: proc (path: string) -> string {
	p := strings.trim_space(path)
	switch {
	case p == "":
		return ""
	case strings.has_prefix(p, "http://"), strings.has_prefix(p, "https://"):
		return p
	case strings.has_prefix(p, "//"):
		return strings.concatenate({"https:", p})
	}
	p = strings.trim_prefix(p, "/static/")
	p = strings.trim_prefix(p, "static/")
	return strings.concatenate({kCDN, "/static/", p})
}

url_encode :: proc (s: string) -> string {
	return abi.host_str("text.url_encode", s)
}

// host_num reads a numeric answer from a host helper (text.chapter_num).
host_num :: proc (fn: string, arg: string) -> f64 {
	raw := abi.host_args(fn, {arg})
	if raw == "" {
		return 0
	}
	v: f64
	if json.unmarshal(transmute([]u8)raw, &v) != nil {
		return 0
	}
	return v
}

// parse_time converts a unix value to RFC3339. The site reports createdAt in
// milliseconds while the host helper wants seconds, hence the /1000 at the call
// site. "" stays "" and the ABI then omits released_at.
parse_time :: proc (seconds: string) -> string {
	if seconds == "" {
		return ""
	}
	return abi.host_str("text.date_to_iso", seconds)
}

// ─── wire types (ABI in) ─────────────────────────────────────────────────────

Search_Filter :: struct {
	Query:   string   `json:"query"`,
	Page:    int      `json:"page"`,
	Genres:  []string `json:"genres"`,
	Sort_By: string   `json:"sort_by"`,
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

Ts_Doc :: struct {
	ID:           string `json:"id"`,
	Title:        string `json:"title"`,
	English_Title: string `json:"englishTitle"`,
	Poster:       string `json:"poster"`,
	Poster_Small: string `json:"posterSmall"`,
}

Ts_Hit :: struct {
	Document: Ts_Doc `json:"document"`,
}

Ts_Resp :: struct {
	Hits: []Ts_Hit `json:"hits"`,
}

Poster :: struct {
	Image:        string `json:"image"`,
	ID:           string `json:"id"`,
	Small_Image:  string `json:"smallImage"`,
	Medium_Image: string `json:"mediumImage"`,
}

Genre_Item :: struct {
	ID:   string `json:"id"`,
	Name: string `json:"name"`,
}

Author_Item :: struct {
	Name: string `json:"name"`,
}

Manga_Page :: struct {
	ID:             string       `json:"id"`,
	Title:          string       `json:"title"`,
	English_Title:  string       `json:"englishTitle"`,
	Synopsis:       string       `json:"synopsis"`,
	Status:         string       `json:"status"`,
	Genres:         []Genre_Item `json:"genres"`,
	Authors:        []Author_Item `json:"authors"`,
	Poster:         Poster       `json:"poster"`,
}

Page_Resp :: struct {
	Manga_Page: Manga_Page `json:"mangaPage"`,
}

All_Chapter :: struct {
	ID:         string `json:"id"`,
	Title:      string `json:"title"`,
	Number:     f64    `json:"number"`,
	Index:      int    `json:"index"`,
	Page_Count: int    `json:"pageCount"`,
	Created_At: i64    `json:"createdAt"`,
}

All_Resp :: struct {
	Chapters: []All_Chapter `json:"chapters"`,
}

Read_Page :: struct {
	Image:  string `json:"image"`,
	Number: int    `json:"number"`,
}

Read_Chapter :: struct {
	ID:    string      `json:"id"`,
	Title: string      `json:"title"`,
	Pages: []Read_Page `json:"pages"`,
}

Read_Resp :: struct {
	Read_Chapter: Read_Chapter `json:"readChapter"`,
}

// ─── search ──────────────────────────────────────────────────────────────────

search_impl :: proc (arg: string) -> string {
	f: Search_Filter
	if json.unmarshal_string(arg, &f) != nil {
		return "[]"
	}

	q := strings.trim_space(f.Query)
	if q == "" {
		q = "*" // Typesense "match everything": browse mode
	}
	page := f.Page
	if page < 1 {
		page = 1
	}

	// The site's own filter: drop drafts, view-less entries and novel rows.
	filter := "hidden:!=true && views:>0 && medium:!=[`Novel`]"
	if len(f.Genres) > 0 && f.Genres[0] != "" {
		// genres_impl yields the numeric genre ids, which this API filters on
		// directly (genreIds:= vs the rejected `genres` field name).
		filter = strings.concatenate({
			filter, " && genreIds:=[`", strings.trim_space(f.Genres[0]), "`]",
		})
	}

	url := strings.concatenate({
		kSite, "/collections/manga/documents/search",
		"?q=", url_encode(q),
		"&filter_by=", url_encode(filter),
		"&query_by=", url_encode("title,englishTitle,otherNames,authors"),
		"&page=", fmt.aprintf("%d", page),
		"&per_page=20",
	})

	body, ok := abi.http_body(url, kJSONHeaders)
	if !ok {
		return "[]"
	}
	resp: Ts_Resp
	if json.unmarshal_string(body, &resp) != nil {
		return "[]"
	}

	out := make([dynamic]Manga_Out, 0, len(resp.Hits))
	defer delete(out)
	for h in resp.Hits {
		d := h.Document
		if d.ID == "" {
			continue
		}
		title := strings.trim_space(d.Title)
		if title == "" {
			title = strings.trim_space(d.English_Title)
		}
		cover := d.Poster
		if cover == "" {
			cover = d.Poster_Small
		}
		// Status is deliberately absent: the search index does not carry it, and
		// the detail call fills it in. Emitting "" would look like "unknown".
		append(&out, Manga_Out{
			ID        = d.ID,
			Title     = title,
			Cover_URL = image_url(cover),
		})
	}
	return abi.to_json(out[:])
}

// decode_id unwraps the JSON string the host passes as the argument.
decode_id :: proc (arg: string) -> string {
	v: string
	if json.unmarshal_string(arg, &v) != nil {
		v = strings.trim_space(arg)
	}
	return v
}

// ─── detail ──────────────────────────────────────────────────────────────────

detail_impl :: proc (arg: string) -> string {
	id := strings.trim_space(decode_id(arg))
	if id == "" {
		return "{}"
	}

	body, ok := abi.http_body(
		strings.concatenate({kSite, "/api/manga/page?id=", url_encode(id)}),
		kJSONHeaders,
	)
	if !ok {
		// Never "{}" without an id: the host reads a missing id as an empty
		// series, while an id with no fields keeps the entry addressable.
		return abi.to_json(Manga_Out{ID = id})
	}
	resp: Page_Resp
	if json.unmarshal_string(body, &resp) != nil {
		return abi.to_json(Manga_Out{ID = id})
	}
	mp := resp.Manga_Page

	cover := mp.Poster.Small_Image
	if cover == "" {
		cover = mp.Poster.Image
	}
	if cover == "" {
		cover = mp.Poster.Medium_Image
	}
	if cover == "" {
		cover = mp.Poster.ID
	}

	// Every author is joined, like the Lua reference: sources routinely credit
	// writer and artist separately and dropping either loses information.
	author := ""
	for a in mp.Authors {
		n := strings.trim_space(a.Name)
		if n == "" {
			continue
		}
		if author == "" {
			author = n
		} else {
			author = strings.concatenate({author, ", ", n})
		}
	}

	genres := make([dynamic]string, 0, len(mp.Genres))
	defer delete(genres)
	for g in mp.Genres {
		n := strings.trim_space(g.Name)
		if n != "" {
			append(&genres, n)
		}
	}

	title := strings.trim_space(mp.Title)
	if title == "" {
		title = strings.trim_space(mp.English_Title)
	}

	return abi.to_json(Manga_Out{
		ID        = id,
		Title     = title,
		Cover_URL = image_url(cover),
		Author    = author,
		// synopsis carries inline HTML; the host helper strips it.
		Desc      = abi.host_str("text.strip_html", mp.Synopsis),
		Status    = abi.host_str("text.normalize_status", mp.Status),
		Genres    = genres[:],
	})
}

// ─── chapters ────────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	manga := strings.trim_space(decode_id(arg))
	if manga == "" {
		return "[]"
	}

	body, ok := abi.http_body(
		strings.concatenate({kSite, "/api/manga/allChapters?mangaId=", url_encode(manga)}),
		kJSONHeaders,
	)
	if !ok {
		return "[]"
	}
	resp: All_Resp
	if json.unmarshal_string(body, &resp) != nil {
		return "[]"
	}

	page_url := strings.concatenate({kSite, "/manga/", manga})
	out := make([dynamic]Chapter_Out, 0, len(resp.Chapters))
	defer delete(out)
	for c in resp.Chapters {
		if c.ID == "" {
			continue
		}
		title := strings.trim_space(c.Title)
		if title == "" {
			title = fmt.aprintf("Chapter %v", c.Number)
		}
		// chapter_num comes from the label (text.chapter_num), which handles
		// "Chapter 12.5" and sub-chapters the raw number field flattens.
		num := host_num("text.chapter_num", title)
		if num == 0 {
			num = c.Number
		}
		rel := ""
		if c.Created_At > 0 {
			rel = parse_time(fmt.aprintf("%d", c.Created_At / 1000))
		}
		append(&out, Chapter_Out{
			ID          = strings.concatenate({manga, ":", c.ID}),
			Manga_ID    = manga,
			Title       = title,
			Chapter_Num = num,
			Released_At = rel,
			URL         = page_url,
		})
	}

	// The ABI wants newest-first; resuming "continue reading" from the oldest
	// chapter is the symptom when a source hands them back ascending.
	slice.sort_by(out[:], proc(a, b: Chapter_Out) -> bool {
		return a.Chapter_Num > b.Chapter_Num
	})

	return abi.to_json(out[:])
}

// ─── pages ───────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	raw := strings.trim_space(decode_id(arg))
	i := strings.index_byte(raw, ':')
	if i <= 0 || i + 1 >= len(raw) {
		return "[]"
	}
	manga := raw[:i]
	chapter := raw[i + 1:]

	body, ok := abi.http_body(
		strings.concatenate({
			kSite,
			"/api/read/chapter?mangaId=", url_encode(manga),
			"&chapterId=", url_encode(chapter),
		}),
		kJSONHeaders,
	)
	if !ok {
		return "[]"
	}
	resp: Read_Resp
	if json.unmarshal_string(body, &resp) != nil {
		return "[]"
	}

	out := make([dynamic]Page_Out, 0, len(resp.Read_Chapter.Pages))
	defer delete(out)
	for p in resp.Read_Chapter.Pages {
		u := image_url(p.Image)
		if u == "" {
			continue
		}
		append(&out, Page_Out{Index = len(out), URL = u})
	}
	return abi.to_json(out[:])
}

// ─── genres ──────────────────────────────────────────────────────────────────
//
// Same table as the Lua reference: these are the numeric ids the Typesense
// filter accepts (genreIds:=[...]). Adult rows from the site's own
// availableFilters are left out on purpose — it would put them in front of
// every user of the genre picker.

GENRES := [?]Genre_Out{
	{Name = "Action", Slug = "39"},
	{Name = "Adventure", Slug = "37"},
	{Name = "Comedy", Slug = "6"},
	{Name = "Drama", Slug = "31"},
	{Name = "Fantasy", Slug = "36"},
	{Name = "Historical", Slug = "45"},
	{Name = "Horror", Slug = "44"},
	{Name = "Martial Arts", Slug = "29"},
	{Name = "Mystery", Slug = "32"},
	{Name = "Psychological", Slug = "18"},
	{Name = "Romance", Slug = "9"},
	{Name = "Sci-Fi", Slug = "1"},
	{Name = "Slice of Life", Slug = "7"},
	{Name = "Supernatural", Slug = "22"},
	{Name = "Thriller", Slug = "19"},
	{Name = "Tragedy", Slug = "5"},
}

genres_impl :: proc () -> string {
	return abi.to_json(GENRES[:])
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
