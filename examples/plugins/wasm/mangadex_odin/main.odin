#+feature dynamic-literals

// MangaDex (mangadex.org) source plugin for goIsekai — Odin wasip1 reactor.
//
// Behavioural port of examples/plugins/lua/mangadex/main.lua (+ util.lua), which
// stay as the reference implementation: a successful port keeps its Lua original
// in examples/ next to the wasm build, and only app_data/plugins/<id>/ is swapped.
//
// mangadex.org is a pure JSON API, so there is no HTML parsing and no regex:
//
//	search   GET /manga?limit=100&offset=N&title=<q>[&includedTags[]=TAG]
//	detail   GET /manga/<id>?includes[]=cover_art&includes[]=author&includes[]=artist
//	chapters GET /manga/<id>/feed?limit=500&offset=N
//	pages    GET /at-home/server/<chapter-id>
//	genres   GET /manga/tag
//
// Four details are load-bearing and copied from the Lua source, not guessed:
//
//  1. Search sweeps offset windows itself and stops on a WALL-CLOCK budget (8s
//     against the host's 15s invoke limit), returning partial results rather
//     than failing the whole search. core:time's monotonic clock works on wasi,
//     so time.tick_now/tick_since stand in for Lua's os.clock.
//
//  2. Genre sligs are tag UUIDs read from /manga/tag, never hardcoded: MangaDex
//     reissues tag ids and a stale list silently matches nothing.
//
//  3. The cover URL is the CDN's ".256.jpg" variant, and the image CDN answers
//     403 without a Referer — every request carries one.
//
//  4. Chapter ids are plain UUIDs and the reader page list needs a separate
//     /at-home/server call, which is why GetPageList takes the chapter id as-is.
//
// Chapter output keeps manga_id/url/volume_num because the ABI carries them and
// the Lua source filled them; unlike comizy, mangadex sets these from the start.
package mangadex_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:slice"
import "core:strconv"
import "core:strings"
import "core:time"

kAPI :: "https://api.mangadex.org"
kCDN :: "https://uploads.mangadex.org"
kLANG :: "en"
kHeaders :: `{"Referer":"https://uploads.mangadex.org/"}`

// ─── wire types (ABI in) ─────────────────────────────────────────────────────

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
	Volume_Num:  f64    `json:"volume_num,omitempty"`,
	Released_At: string `json:"released_at,omitempty"`,
	URL:         string `json:"url"`,
}

Page_Out :: struct {
	Index:   int               `json:"index"`,
	URL:     string            `json:"url"`,
	Headers: map[string]string `json:"headers,omitempty"`,
}

Genre_Out :: struct {
	Name: string `json:"name"`,
	Slug: string `json:"slug"`,
}

// ─── wire types (site responses) ─────────────────────────────────────────────

// Localisation maps are unordered, so every one is a plain string map.
Tag_Attrs :: struct {
	Name:  map[string]string `json:"name"`,
	Group: string            `json:"group"`,
}

Tag :: struct {
	ID:         string    `json:"id"`,
	Attributes: Tag_Attrs `json:"attributes"`,
}

// Cover_Attrs is the only relationship attribute used; a bare relationship
// (e.g. "author") simply decodes to a zero value.
Cover_Attrs :: struct {
	FileName: string `json:"fileName"`,
}

Rel :: struct {
	Type:       string      `json:"type"`,
	Attributes: Cover_Attrs `json:"attributes"`,
}

Manga_Attrs :: struct {
	Title:       map[string]string   `json:"title"`,
	Alt_Titles:  []map[string]string `json:"altTitles"`,
	Description: map[string]string   `json:"description"`,
	Status:      string              `json:"status"`,
	Tags:        []Tag               `json:"tags"`,
}

Manga :: struct {
	ID:            string      `json:"id"`,
	Attributes:    Manga_Attrs `json:"attributes"`,
	Relationships: []Rel       `json:"relationships"`,
}

// Search and detail answer different envelopes: a list carries total+data[],
// a single lookup carries just data{}.
Manga_List_Resp :: struct {
	Total: int     `json:"total"`,
	Data:  []Manga `json:"data"`,
}

// /manga/tag answers the same {total,data[]} envelope but with tag objects.
Tag_List_Resp :: struct {
	Total: int   `json:"total"`,
	Data:  []Tag `json:"data"`,
}

Manga_One_Resp :: struct {
	Data: Manga `json:"data"`,
}

Chapter_Attrs :: struct {
	Chapter:    string `json:"chapter"`,
	Volume:     string `json:"volume"`,
	Title:      string `json:"title"`,
	External:   string `json:"externalUrl"`,
	Publish_At: string `json:"publishAt"`,
}

Chapter :: struct {
	ID:         string        `json:"id"`,
	Attributes: Chapter_Attrs `json:"attributes"`,
}

Feed_Resp :: struct {
	Total: int       `json:"total"`,
	Data:  []Chapter `json:"data"`,
}

At_Home_Chapter :: struct {
	Hash: string   `json:"hash"`,
	Data: []string `json:"data"`,
}

At_Home_Resp :: struct {
	Base_URL: string          `json:"baseUrl"`,
	Chapter:  At_Home_Chapter `json:"chapter"`,
}

// ─── helpers ─────────────────────────────────────────────────────────────────

url_encode :: proc (s: string) -> string { return abi.url_encode(s) }

// decode_id unwraps the JSON the host passes the single-string ABI.
decode_id :: proc (arg: string) -> string {
	v: string
	if json.unmarshal_string(arg, &v) != nil {
		v = strings.trim_space(arg)
	}
	return strings.trim_space(v)
}

// first_locale prefers the English entry, then any non-empty one. The maps are
// unordered, so "first" means "first found", exactly like the Lua source.
first_locale :: proc (m: map[string]string) -> string {
	if len(m) == 0 {
		return ""
	}
	if v, ok := m[kLANG]; ok {
		return v
	}
	for _, v in m {
		if v != "" {
			return v
		}
	}
	return ""
}

// first_title walks title["en"], every altTitles["en"], then any locale of
// either: most alt titles carry no English at all, so each map is probed.
first_title :: proc (a: Manga_Attrs) -> string {
	if v, ok := a.Title[kLANG]; ok {
		return v
	}
	for at in a.Alt_Titles {
		if v, ok := at[kLANG]; ok {
			return v
		}
	}
	for at in a.Alt_Titles {
		if v := first_locale(at); v != "" {
			return v
		}
	}
	return first_locale(a.Title)
}

cover_url :: proc (md: Manga) -> string {
	for r in md.Relationships {
		if r.Type == "cover_art" && r.Attributes.FileName != "" {
			return strings.concatenate({kCDN, "/covers/", md.ID, "/", r.Attributes.FileName, ".256.jpg"})
		}
	}
	return ""
}

// number converts the API's chapter/volume strings, which arrive as null for
// oneshots and as "17.5" for decimals; anything unparseable stays 0.
number :: proc (s: string) -> f64 {
	v, ok := strconv.parse_f64(strings.trim_space(s))
	if !ok {
		return 0
	}
	return v
}

date_to_iso :: proc (s: string) -> string { return abi.date_to_iso(s) }

// manga_genres collects the tag names into a caller-owned array. The caller
// must keep it alive until AFTER the reply is marshalled: `return
// abi.to_json(x)` evaluates the marshal first and runs defers afterwards, so a
// `defer delete(genres)` in the same scope is safe — but a slice handed back
// from a helper whose own defer already ran is not (that was an out-of-bounds
// trap until this was split out).
manga_genres :: proc (md: Manga, out: ^[dynamic]string) {
	for t in md.Attributes.Tags {
		if n := first_locale(t.Attributes.Name); n != "" {
			append(out, n)
		}
	}
}

// normalize_status mirrors the Lua source, which falls back to "unknown"
// rather than emitting an empty status.
normalize_status :: proc (raw: string) -> string {
	s := abi.host_str("text.normalize_status", raw)
	if s == "" {
		return "unknown"
	}
	return s
}

// manga_out builds the ABI record. genres is borrowed, not owned.
manga_out :: proc (md: Manga, genres: []string) -> Manga_Out {
	return Manga_Out{
		ID        = md.ID,
		Title     = first_title(md.Attributes),
		Cover_URL = cover_url(md),
		Desc      = first_locale(md.Attributes.Description),
		Status    = normalize_status(md.Attributes.Status),
		Genres    = genres,
	}
}

// content_rating_params: the API hides ratings unless each is named explicitly.
kContentRating :: "contentRating[]=safe&contentRating[]=suggestive&contentRating[]=erotica"

// ─── genres ──────────────────────────────────────────────────────────────────

genres_impl :: proc () -> string {
	body, ok := abi.http_body(strings.concatenate({kAPI, "/manga/tag"}), kHeaders)
	if !ok {
		return "[]"
	}
	resp: Tag_List_Resp
	if json.unmarshal_string(body, &resp) != nil {
		return "[]"
	}

	out := make([dynamic]Genre_Out, 0, len(resp.Data))
	defer delete(out)
	for t in resp.Data {
		a := t.Attributes
		name := a.Name[kLANG]
		if t.ID == "" || name == "" {
			continue
		}
		if a.Group == "genre" || a.Group == "theme" {
			append(&out, Genre_Out{Name = name, Slug = t.ID})
		}
	}
	slice.sort_by(out[:], proc(a, b: Genre_Out) -> bool {
		return a.Name < b.Name
	})
	return abi.to_json(out[:])
}

// ─── search ──────────────────────────────────────────────────────────────────

// search_budget is how long the offset sweep may run. The host invoke budget is
// 15s, so stopping at 8s returns partial results instead of failing outright.
SEARCH_BUDGET :: 8.0

search_impl :: proc (arg: string) -> string {
	f: Search_Filter
	if json.unmarshal_string(arg, &f) != nil {
		return "[]"
	}
	title := strings.trim_space(f.Query)
	if title == "" {
		title = strings.trim_space(f.Q)
	}
	// MangaDex API /manga?title= does exact-ish phrase matching that the
	// website /search does not — a 4-word phrase (e.g. "Isekai
	// Shoukansareta kedo") yields 0 API hits while the site shows matches.
	// When the full phrase finds nothing, retry with progressively shorter
	// prefixes (drop the last word each retry) until results come back or
	// only one word remains. Mirrors the Lua reference, which falls back
	// gracefully.
	searchWords := strings.split(title, " ") if title != "" else nil
	
	out := make([dynamic]Manga_Out, 0)
	defer delete(out)
	// Every manga in the reply borrows its genre array from here, and all of
	// them stay alive until this proc returns — i.e. after the marshal.
	keep := make([dynamic][dynamic]string, 0)
	defer {
		for g in keep {
			delete(g)
		}
		delete(keep)
	}
	
	maxWords := len(searchWords)
	// searchQuery is reassigned each pass — must be declared before the loop
	// because Odin for-loops do not accept `var` declarations inside.
	searchQuery: []string
	for w := maxWords; w >= 1; w -= 1 {
		searchQuery = nil
		if w <= len(searchWords) {
			searchQuery = searchWords[:w]
		}
	
		offset := 0
		total := 999_999
		start := time.tick_now()
		hadResult := false
		searchedOnce := false
	
		for offset < total {
			if time.duration_seconds(time.tick_since(start)) > SEARCH_BUDGET {
				break
			}
	
			sb := strings.builder_make()
			strings.write_string(&sb, kAPI)
			strings.write_string(&sb, "/manga?limit=100&offset=")
			strings.write_int(&sb, offset)
			strings.write_string(&sb, "&includes[]=cover_art&includes[]=author")
			strings.write_string(&sb, "&availableTranslatedLanguage[]=en")
			if len(searchQuery) == 0 {
				strings.write_string(&sb, "&order[followedCount]=desc")
			} else {
				strings.write_string(&sb, "&order[relevance]=desc&title=")
				strings.write_string(&sb, url_encode(strings.join(searchQuery, " ")))
			}
			for g in f.Genres {
				strings.write_string(&sb, "&includedTags[]=")
				strings.write_string(&sb, url_encode(g))
			}
			strings.write_string(&sb, "&")
			strings.write_string(&sb, kContentRating)
	
			body, ok := abi.http_body(strings.to_string(sb), kHeaders)
			strings.builder_destroy(&sb)
			if !ok {
				break
			}
			resp: Manga_List_Resp
			if json.unmarshal_string(body, &resp) != nil {
				break
			}
	
			searchedOnce = true
			if resp.Total > 0 {
				total = resp.Total
			}
			n := len(resp.Data)
			if n == 0 {
				break
			}
			hadResult = true
			for md in resp.Data {
				genres := make([dynamic]string, 0, len(md.Attributes.Tags))
				manga_genres(md, &genres)
				append(&out, manga_out(md, genres[:]))
				append(&keep, genres)
			}
	
			offset += n
			if n < 100 {
				break
			}
			if len(out) >= 2000 {
				break
			}
		}
	
		// Got results, or no query to fall back on further. Stop.
		if hadResult || !searchedOnce || len(searchQuery) == 0 {
			break
		}
	}
	return abi.to_json(out[:])
}
// ─── detail ──────────────────────────────────────────────────────────────────

detail_impl :: proc (arg: string) -> string {
	id := decode_id(arg)
	if id == "" {
		return abi.to_json(Manga_Out{ID = ""})
	}
	body, ok := abi.http_body(
		strings.concatenate({
			kAPI,
			"/manga/",
			url_encode(id),
			"?includes[]=cover_art&includes[]=author&includes[]=artist",
		}),
		kHeaders,
	)
	if !ok {
		// A failed lookup still returns an object: an empty table encodes [] and
		// that cannot decode into a detail record.
		return abi.to_json(Manga_Out{ID = id})
	}
	resp: Manga_One_Resp
	if json.unmarshal_string(body, &resp) != nil || resp.Data.ID == "" {
		return abi.to_json(Manga_Out{ID = id})
	}
	// Object export, not a list: the detail reply must be a JSON object. The
	// marshal runs before this defer, so genres is still alive when it does.
	genres := make([dynamic]string, 0, len(resp.Data.Attributes.Tags))
	defer delete(genres)
	manga_genres(resp.Data, &genres)
	return abi.to_json(manga_out(resp.Data, genres[:]))
}

// ─── chapters ────────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	manga := decode_id(arg)
	if manga == "" {
		return "[]"
	}

	out := make([dynamic]Chapter_Out, 0)
	defer delete(out)

	offset := 0
	total := 999_999
	for offset < total {
		sb := strings.builder_make()
		strings.write_string(&sb, kAPI)
		strings.write_string(&sb, "/manga/")
		strings.write_string(&sb, url_encode(manga))
		strings.write_string(&sb, "/feed?limit=500&offset=")
		strings.write_int(&sb, offset)
		strings.write_string(&sb, "&translatedLanguage[]=en")
		strings.write_string(&sb, "&order[volume]=asc&order[chapter]=asc")
		strings.write_string(&sb, "&includes[]=scanlation_group&includeEmptyPages=0&")
		strings.write_string(&sb, kContentRating)

		body, ok := abi.http_body(strings.to_string(sb), kHeaders)
		strings.builder_destroy(&sb)
		if !ok {
			break
		}
		resp: Feed_Resp
		if json.unmarshal_string(body, &resp) != nil {
			break
		}

		if resp.Total > 0 {
			total = resp.Total
		}
		n := len(resp.Data)
		if n == 0 {
			break
		}

		for cd in resp.Data {
			a := cd.Attributes
			// External chapters link off-site and have no reader pages.
			if a.External != "" {
				continue
			}
			has_num := strings.trim_space(a.Chapter) != ""
			has_title := strings.trim_space(a.Title) != ""

			ch_title := strings.concatenate({"Chapter ", a.Chapter})
			if has_title {
				ch_title = a.Title
			}
			if !has_num && !has_title {
				ch_title = "Oneshot"
			}

			ch := Chapter_Out{
				ID          = cd.ID,
				Manga_ID    = manga,
				Title       = ch_title,
				Chapter_Num = number(a.Chapter),
				Volume_Num  = number(a.Volume),
				URL         = strings.concatenate({"https://mangadex.org/chapter/", cd.ID}),
			}
			// An unparseable date is left out so the host keeps its zero value
			// instead of failing the whole record.
			ch.Released_At = date_to_iso(a.Publish_At)
			append(&out, ch)
		}

		offset += n
		if n < 500 {
			break
		}
	}

	// ABI contract: newest-first. The API sorts ascending.
	slice.reverse(out[:])
	return abi.to_json(out[:])
}

// ─── pages ───────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	chapter := decode_id(arg)
	if chapter == "" {
		return "[]"
	}
	body, ok := abi.http_body(
		strings.concatenate({kAPI, "/at-home/server/", url_encode(chapter)}),
		kHeaders,
	)
	if !ok {
		return "[]"
	}
	resp: At_Home_Resp
	if json.unmarshal_string(body, &resp) != nil {
		return "[]"
	}
	if resp.Base_URL == "" || resp.Chapter.Hash == "" {
		return "[]"
	}

	out := make([dynamic]Page_Out, 0, len(resp.Chapter.Data))
	defer delete(out)
	for file in resp.Chapter.Data {
		if file == "" {
			continue
		}
		append(
			&out,
			Page_Out {
				Index = len(out),
				URL = strings.concatenate({resp.Base_URL, "/data/", resp.Chapter.Hash, "/", file}),
				// The CDN rejects requests without a Referer.
				Headers = map[string]string{"Referer" = "https://uploads.mangadex.org/"},
			},
		)
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
