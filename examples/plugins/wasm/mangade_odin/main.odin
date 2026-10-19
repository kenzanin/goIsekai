// MangaDE (mangade.io) source plugin for goIsekai — Odin wasip1 reactor.
//
// Behavioural port of examples/plugins/lua/mangade/main.lua, which stays as the
// reference implementation: a successful port keeps its Lua original in
// examples/ next to the wasm build, and only app_data/plugins/<id>/ is swapped.
//
// The site is API-first, so there is no HTML parsing and no regex anywhere:
//
//	search   GET /api/comics?page=N&size=20[&name=Q][&genres%5B0%5D=ID]
//	detail   GET /api/comics/<id>/view    -> data: name/image/status/genres
//	chapters GET /api/comics/<id>/view    -> same response, data.news_chapters
//	pages    GET /api/chapters/<id>/view  -> data.chapter_images[]
//	genres   GET /api/genres?size=500
//
// Every reply is {success, message, data}; success == false is a failure even
// when the HTTP status is 200, which is why every call checks it explicitly.
//
// Two details are load-bearing and copied from the Lua source, not guessed:
//
//  1. Genre browse filters by NUMERIC id (&genres[0]=<id>), and the slug this
//     plugin exposes for a genre is that numeric id as a string.
//  2. chapter_number is a decimal-safe STRING ("17.5" as well as "59"), so
//     chapter_num stays f64 and the list is sorted newest-first explicitly —
//     the API's arrival order is not a contract.
package mangade_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:slice"
import "core:strconv"
import "core:strings"

kAPI :: "https://api.mangade.io"

// ─── wire types (ABI in) ─────────────────────────────────────────────────────

// Q is the short alias the host used to send before it settled on "query"; it
// is still accepted so an older caller cannot silently produce an empty search.
Search_Filter :: struct {
	Query:   string   `json:"query"`,
	Q:       string   `json:"q"`,
	Page:    int      `json:"page"`,
	Genres:  []string `json:"genres"`,
	Sort_By: string   `json:"sort_by"`,
}

// ─── wire types (ABI out) ────────────────────────────────────────────────────

Manga_Out :: struct {
	ID:        string   `json:"id"`,
	Title:     string   `json:"title"`,
	Cover_URL: string   `json:"cover_url"`,
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

// ─── wire types (site responses) ─────────────────────────────────────────────

Genre_Item :: struct {
	ID:   string `json:"id"`,
	Name: string `json:"name"`,
}

Search_Item :: struct {
	ID:    string `json:"id"`,
	Name:  string `json:"name"`,
	Image: string `json:"image"`,
}

Chapter_Item :: struct {
	ID:             string `json:"id"`,
	Name:           string `json:"name"`,
	Chapter_Number: string `json:"chapter_number"`,
}

// Comic_Data is /api/comics/<id>/view. It carries the detail fields AND the
// full chapter list, so detail and chapters decode the same response shape.
Comic_Data :: struct {
	ID            : string         `json:"id"`,
	Name          : string         `json:"name"`,
	Image         : string         `json:"image"`,
	Status        : string         `json:"status"`,
	Description   : string         `json:"description"`,
	Genres        : []Genre_Item   `json:"genres"`,
	News_Chapters : []Chapter_Item `json:"news_chapters"`,
}

Comic_Resp :: struct {
	Success: bool       `json:"success"`,
	Data:    Comic_Data `json:"data"`,
}

Search_Data :: struct {
	List: []Search_Item `json:"list"`,
}

Search_Resp :: struct {
	Success: bool        `json:"success"`,
	Data:    Search_Data `json:"data"`,
}

Page_Image :: struct {
	Image: string `json:"image"`,
}

Pages_Data :: struct {
	Chapter_Images: []Page_Image `json:"chapter_images"`,
}

Pages_Resp :: struct {
	Success: bool       `json:"success"`,
	Data:    Pages_Data `json:"data"`,
}

Genres_Data :: struct {
	Genres: []Genre_Item `json:"genres"`,
}

Genres_Resp :: struct {
	Success: bool        `json:"success"`,
	Data:    Genres_Data `json:"data"`,
}

// ─── helpers ─────────────────────────────────────────────────────────────────

url_encode :: proc (s: string) -> string { return abi.url_encode(s) }

// decode_id unwraps the JSON the host passes the single-string ABI: a bare
// quoted id ("46959") or an object ({"id":"46959"}) both end up as "46959".
decode_id :: proc (arg: string) -> string {
	v: string
	if json.unmarshal_string(arg, &v) != nil {
		v = strings.trim_space(arg)
	}
	return strings.trim_space(v)
}

// comic_view fetches /api/comics/<id>/view once and reports success, shared by
// detail and chapters so both cannot drift apart on the failure shape.
comic_view :: proc (id: string) -> (Comic_Data, bool) {
	body, ok := abi.get_body(strings.concatenate({kAPI, "/api/comics/", url_encode(id), "/view"}))
	if !ok {
		return {}, false
	}
	resp: Comic_Resp
	if json.unmarshal_string(body, &resp) != nil || !resp.Success {
		return {}, false
	}
	return resp.Data, true
}

// ─── search ──────────────────────────────────────────────────────────────────

search_impl :: proc (arg: string) -> string {
	f: Search_Filter
	if json.unmarshal_string(arg, &f) != nil {
		return "[]"
	}
	q := strings.trim_space(f.Query)
	if q == "" {
		q = strings.trim_space(f.Q)
	}
	// Genre browse works with no text query; a text-only search needs one.
	if q == "" && len(f.Genres) == 0 {
		return "[]"
	}
	page := f.Page
	if page < 1 {
		page = 1
	}

	sb := strings.builder_make()
	defer strings.builder_destroy(&sb)
	strings.write_string(&sb, kAPI)
	strings.write_string(&sb, "/api/comics?page=")
	strings.write_int(&sb, page)
	strings.write_string(&sb, "&size=20")
	if q != "" {
		strings.write_string(&sb, "&name=")
		strings.write_string(&sb, url_encode(q))
	}
	if len(f.Genres) > 0 {
		// First genre wins; the API filters by numeric id (see header).
		strings.write_string(&sb, "&genres%5B0%5D=")
		strings.write_string(&sb, url_encode(f.Genres[0]))
	}

	body, ok := abi.get_body(strings.to_string(sb))
	if !ok {
		return "[]"
	}
	resp: Search_Resp
	if json.unmarshal_string(body, &resp) != nil || !resp.Success {
		return "[]"
	}

	out := make([dynamic]Manga_Out, 0, len(resp.Data.List))
	defer delete(out)
	for it in resp.Data.List {
		append(&out, Manga_Out{
			ID        = it.ID,
			Title     = strings.trim_space(it.Name),
			Cover_URL = it.Image,
		})
	}
	return abi.to_json(out[:])
}

// ─── detail ──────────────────────────────────────────────────────────────────

detail_impl :: proc (arg: string) -> string {
	id := decode_id(arg)
	if id == "" {
		return "{}"
	}
	// Failure shape for an object export: bare {id}. The host never caches it,
	// so the next call retries; an [] here would not decode into a struct.
	d, ok := comic_view(id)
	if !ok {
		return abi.to_json(Manga_Out{ID = id})
	}

	genres := make([dynamic]string, 0, len(d.Genres))
	defer delete(genres)
	for g in d.Genres {
		n := strings.trim_space(g.Name)
		if n != "" {
			append(&genres, n)
		}
	}

	return abi.to_json(Manga_Out{
		ID        = id,
		Title     = strings.trim_space(d.Name),
		Cover_URL = d.Image,
		Desc      = d.Description,
		Status    = abi.host_str("text.normalize_status", d.Status),
		Genres    = genres[:],
	})
}

// ─── chapters ────────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	manga := decode_id(arg)
	if manga == "" {
		return "[]"
	}
	d, ok := comic_view(manga)
	if !ok {
		return "[]"
	}

	out := make([dynamic]Chapter_Out, 0, len(d.News_Chapters))
	defer delete(out)
	for ch in d.News_Chapters {
		if ch.ID == "" {
			continue
		}
		// A chapter whose number does not parse is dropped, like the Lua
		// source: emitting 0 would sort it past every real first chapter.
		num, parsed := strconv.parse_f64(strings.trim_space(ch.Chapter_Number))
		if !parsed {
			continue
		}
		append(&out, Chapter_Out{
			ID          = ch.ID,
			Manga_ID    = manga,
			Title       = strings.trim_space(ch.Name),
			Chapter_Num = num,
			URL         = strings.concatenate({"https://mangade.io/chapter/", ch.ID}),
		})
	}

	// The ABI wants newest-first; the API's arrival order is not a contract.
	slice.sort_by(out[:], proc(a, b: Chapter_Out) -> bool {
		return a.Chapter_Num > b.Chapter_Num
	})
	return abi.to_json(out[:])
}

// ─── pages ───────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	ch := decode_id(arg)
	if ch == "" {
		return "[]"
	}
	body, ok := abi.get_body(strings.concatenate({kAPI, "/api/chapters/", url_encode(ch), "/view"}))
	if !ok {
		return "[]"
	}
	resp: Pages_Resp
	if json.unmarshal_string(body, &resp) != nil || !resp.Success {
		return "[]"
	}

	out := make([dynamic]Page_Out, 0, len(resp.Data.Chapter_Images))
	defer delete(out)
	for img, i in resp.Data.Chapter_Images {
		if img.Image == "" {
			continue
		}
		// Index comes from the response position, so a dropped item can never
		// make the remaining pages shift.
		append(&out, Page_Out{Index = i, URL = img.Image})
	}
	return abi.to_json(out[:])
}

// ─── genres ──────────────────────────────────────────────────────────────────

genres_impl :: proc () -> string {
	body, ok := abi.get_body(strings.concatenate({kAPI, "/api/genres?size=500"}))
	if !ok {
		return "[]"
	}
	resp: Genres_Resp
	if json.unmarshal_string(body, &resp) != nil || !resp.Success {
		return "[]"
	}

	out := make([dynamic]Genre_Out, 0, len(resp.Data.Genres))
	defer delete(out)
	for g in resp.Data.Genres {
		if g.ID == "" || strings.trim_space(g.Name) == "" {
			continue
		}
		// The slug IS the numeric id: /api/comics filters by it. The host
		// persists this list in plugins.genres, so no in-module cache is kept.
		append(&out, Genre_Out{Name = g.Name, Slug = g.ID})
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
