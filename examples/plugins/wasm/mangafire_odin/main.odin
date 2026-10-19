// MangaFire (mangafire.to) source plugin for goIsekai — Odin wasip1 reactor.
//
// Behavioural port of examples/plugins/lua/mangafire/main.lua, which stays as
// the reference implementation. A successful port keeps its Lua original in
// examples/ next to the wasm build; only app_data/plugins/<id>/ is swapped.
//
// JSON API, no HTML scraping. Every request is VRF-signed: the query string is
// built from a key-sorted parameter set and the host computes the signature
// over that SAME set (host.crypto.vrf_sign), so the two must never drift.
// Three things in the Lua source are load-bearing and copied verbatim:
//
//  1. `keyword` is omitted entirely when the query is empty. An empty value
//     changes the signed parameter set and the API answers 403.
//  2. Array params are sent as repeated raw `genres_in[]` while the signature
//     covers the INDEXED form (`genres_in[0]`, `genres_in[1]`).
//  3. Search stops at a wall-clock budget rather than a page count: a bare
//     genre browse matches thousands of titles and would blow the 15 s invoke
//     deadline, so partial results are returned instead of failing.
//
// genre ids come from the site's own filter panel and are stable; the API
// exposes no endpoint that lists them, so the table is carried here.

package mangafire_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:slice"
import "core:strings"
import "core:time"

kAPI :: "https://mangafire.to/api"
kReferer :: "https://mangafire.to/"
kHeaders :: `{"Referer":"https://mangafire.to/"}`

// Search must return before the host's 15 s invoke deadline; 8 s leaves room
// for the first request's latency on a cold connection.
SEARCH_BUDGET :: 8.0

// ─── VRF signing ─────────────────────────────────────────────────────────────
// The three stages are the same tables the JS plugin uses. iv is a byte; the
// key/tbl pairs are base64 and decoded host-side.

VRF_STAGES_JSON :: `[{"iv":90,"key":"0Ec58JOY3uBzJK9m3zqIOpdlF7UFiax9DmA=","tbl":"yINlmUNho8VYJT+ibTIP+9ESiULpVEtMOoD6U6lRE0R/xwXo/Xp9NrUgC4cw/Lmo33vUyjUE40kUoEWIr/fxfNNcq2s79ShQ5NhNrFnJ4hXPwOu/SuXzIbuTQKGFvfm08E9jvCfqAtoDqvQq3dVWPQFmJjgvkISBeXY3BgANR+yVnjGbcxZ47d6kLNfZPIayTq3/YGySb1KuVZodWp/WGNAO5pfMcpaK53Hhs0allBszaMaxuouOwdxbwgxIw6YunSsXjI05Yi0j9j4eHKfSXR8Ifo/Od+8iamRfCXTyvm7NGRGYdcQ0ywcK/u6RXhrbcCm4t2eCtrDgQVecJGkQ+A=="},{"iv":53,"key":"AAdjb1iPY8CiDmq9H34tKTBF8a3oDQ==","tbl":"IUFltCxD3Oc2cwCgkJffthaOg9cgPUb0LgW6H/VtfcF0kc5F25t+aWj6JH9VOhOaY0rAFdUxlDnl5BLNvwEJvQtP5qcw7vdb/K+chnbwnspSHT8mz5lqwz41TezG0hkO06FTjJZhsyNuFLDpD2ZZxQj/QIRcF90zpmQ7Byu483WsQqUE0C342HL+JXngRB6fRzxRyVTaKu83h7UYTJ0QMt6ixFh6S3F8gqkKwrGTL3jHNBsD45UnifK8+RGtishQV2K3rujLKEkiZxpr2dYcudFW4oFsDKhad3CLBvuyTqsCo4B7mL5IKQ1vXo/MOOvq1I1d8ar9X6Ttu5KF4fZgiA=="},{"iv":186,"key":"DELOJgPsVaCcblDtTGMdHzM=","tbl":"NQHlu1/wVO5EmkwQymF810qqY2xG1k2obcas4Z9mCsPEIFl9pRIjFxbJ7ybMHbBckT5Ton85E0FOeHezbh/mjlEYpmpnlXOS8dgrqeq2KfxImTh1YK9y0PeMNhzA1OQzSY9brYOJq/l2QnE/hwOeZIhPixVSKIUlDb5vLcH6RWKxkIEMuP0bDwIqQ71AJJaEaMJL7A6YtyIwoRT+L5v4aZzodN/0+3nOGsfblFjgxSfPzVDjNFeNl5P26+kEC/8AHgdrpAbt3hHz3HrRN1Y6e+JHgF7ncFWnoF0y3THL1S71WgWGCa6KtSzTCCG58n68nTyj2T3Sshk7utqCtMi/ZQ=="}]`

// Vrf_Params keeps TWO views of the same call: `sign` is what the signature
// covers, `wire` is what goes on the query string. They differ for array
// params, which is exactly the mismatch rule 2 above is about.
Vrf_Params :: struct {
	sign: map[string]string,
	wire: map[string]string,
}

vrf_params_make :: proc () -> Vrf_Params {
	return Vrf_Params{sign = make(map[string]string), wire = make(map[string]string)}
}

vrf_params_destroy :: proc (p: ^Vrf_Params) {
	delete(p.sign)
	delete(p.wire)
}

// vrf_set adds a scalar: the wire key is URL-encoded like the value.
vrf_set :: proc (p: ^Vrf_Params, k, v: string) {
	p.sign[k] = v
	p.wire[k] = url_encode(k)
}

// vrf_set_array adds items under a "name[]" key. The site signs the indexed
// spelling but expects the repeated "name[]" spelling on the wire, and the
// wire key is emitted raw (not URL-encoded) as the Lua source does.
vrf_set_array :: proc (p: ^Vrf_Params, name: string, items: []string) {
	base := strings.trim_suffix(name, "[]")
	for item, i in items {
		indexed := fmt.tprintf("%s[%d]", base, i)
		p.sign[indexed] = item
		p.wire[indexed] = name
	}
}

vrf_sign :: proc (path: string, sign: map[string]string) -> string {
	raw_params, err := json.marshal(sign)
	if err != nil {
		return ""
	}
	raw := abi.host_args("crypto.vrf_sign", {path, string(raw_params), VRF_STAGES_JSON})
	if raw == "" || raw == "null" {
		return ""
	}
	out: string
	if json.unmarshal_string(raw, &out) != nil {
		return ""
	}
	return out
}

// vrf_url writes `/<path>?<sorted params>&vrf=<signature>` into sb.
//
// sb is OWNED BY THE CALLER on purpose. strings.to_string(sb) does NOT copy the
// buffer — it aliases it — so returning that string while a
// `defer strings.builder_destroy(&sb)` is armed hands the caller a dangling
// slice. That failure is silent: the URL looks fine in a debug print yet the
// fetch fails, and the endpoint answers [] with no error anywhere.
vrf_url :: proc (sb: ^strings.Builder, path: string, p: ^Vrf_Params) {
	keys := make([dynamic]string, 0, len(p.sign), context.temp_allocator)
	for k in p.sign {
		append(&keys, k)
	}
	slice.sort(keys[:])

	strings.write_string(sb, kAPI)
	strings.write_string(sb, path)
	strings.write_string(sb, "?")
	for k in keys {
		strings.write_string(sb, p.wire[k])
		strings.write_string(sb, "=")
		strings.write_string(sb, url_encode(p.sign[k]))
		strings.write_string(sb, "&")
	}
	strings.write_string(sb, "vrf=")
	strings.write_string(sb, vrf_sign(path, p.sign))
}

// ─── host helpers ────────────────────────────────────────────────────────────

url_encode :: proc (s: string) -> string { return abi.url_encode(s) }

sanitize_title :: proc (s: string) -> string { return abi.sanitize_title(s) }

// normalize_status mirrors helpers.lua: empty input yields "unknown" rather
// than the host helper's treatment of an unknown word.
normalize_status :: proc (raw: string) -> string {
	if raw == "" {
		return "unknown"
	}
	return abi.host_str("text.normalize_status", raw)
}

// decode_id accepts the host's bare id ("00935") or an object {"id":"00935"}.
decode_id :: proc (arg: string) -> string {
	s := strings.trim_space(arg)
	if s == "" {
		return ""
	}
	bare: string
	if json.unmarshal_string(s, &bare) == nil {
		return strings.trim_space(bare)
	}
	obj: struct {
		ID: string `json:"id"`,
	}
	if json.unmarshal_string(s, &obj) == nil {
		return strings.trim_space(obj.ID)
	}
	return strings.trim_space(strings.trim(s, `"`))
}

// ─── wire types (API in) ─────────────────────────────────────────────────────
//
// EVERY field here must start with a capital letter. Odin's json unmarshal
// silently skips unexported fields, so a lowercase field decodes to its zero
// value with no error at all — the classic "the API returned data but I got an
// empty slice" failure. The json tags carry the wire spelling.

Poster :: struct {
	Small:  string `json:"small"`,
	Medium: string `json:"medium"`,
	Large:  string `json:"large"`,
}

Titles_Item :: struct {
	ID:     int    `json:"id"`,
	Hid:    string `json:"hid"`,
	Title:  string `json:"title"`,
	Status: string `json:"status"`,
	Poster: Poster `json:"poster"`,
}

// Meta is camelCase on the wire (lastPage, hasNext) even though most of the
// payload is snake_case — verified against the live API, not assumed.
Meta :: struct {
	Total:    int  `json:"total"`,
	PerPage:  int  `json:"perPage"`,
	Page:     int  `json:"page"`,
	LastPage: int  `json:"lastPage"`,
	HasNext:  bool `json:"hasNext"`,
	HasPrev:  bool `json:"hasPrev"`,
}

Titles_Resp :: struct {
	Items: []Titles_Item `json:"items"`,
	Meta:  Meta          `json:"meta"`,
}

Named :: struct {
	ID:    int    `json:"id"`,
	Title: string `json:"title"`,
}

Detail_Data :: struct {
	Hid:          string  `json:"hid"`,
	Title:        string  `json:"title"`,
	Status:       string  `json:"status"`,
	SynopsisHTML: string  `json:"synopsisHtml"`,
	Poster:       Poster  `json:"poster"`,
	Genres:       []Named `json:"genres"`,
	Authors:      []Named `json:"authors"`,
}

Detail_Resp :: struct {
	Data: Detail_Data `json:"data"`,
}

Chapter_Item :: struct {
	ID:        int    `json:"id"`,
	Number:    f64    `json:"number"`,
	Name:      string `json:"name"`,
	CreatedAt: i64    `json:"createdAt"`,
}

Chapters_Resp :: struct {
	Items: []Chapter_Item `json:"items"`,
	Meta:  Meta           `json:"meta"`,
}

Page_Item :: struct {
	URL: string `json:"url"`,
}

Pages_Data :: struct {
	Pages: []Page_Item `json:"pages"`,
}

Pages_Resp :: struct {
	Data: Pages_Data `json:"data"`,
}

// ─── wire types (ABI out) ────────────────────────────────────────────────────

Search_Filter :: struct {
	Query:  string   `json:"query"`,
	Q:      string   `json:"q"`,
	Page:   int      `json:"page"`,
	Genres: []string `json:"genres"`,
}

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

// Page_Headers is a concrete struct rather than map[string]string: the only
// header a reader page ever needs is the Referer, and a map per page is an
// allocation for nothing.
Page_Headers :: struct {
	Referer: string `json:"Referer"`,
}

Page_Out :: struct {
	Index:   int          `json:"index"`,
	URL:     string       `json:"url"`,
	Headers: Page_Headers `json:"headers"`,
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

	p := vrf_params_make()
	defer vrf_params_destroy(&p)
	vrf_set(&p, "limit", "50")
	// Rule 1: an empty keyword must be absent, not empty — the signature is
	// checked against the exact parameter set.
	if query != "" {
		vrf_set(&p, "keyword", query)
	}
	if len(f.Genres) > 0 {
		vrf_set_array(&p, "genres_in[]", f.Genres)
		vrf_set(&p, "genres_mode", "and")
	}

	out := make([dynamic]Manga_Out, 0, 50)
	defer delete(out)

	start := time.tick_now()
	page := 1
	for {
		if time.duration_seconds(time.tick_since(start)) > SEARCH_BUDGET {
			break
		}
		vrf_set(&p, "page", fmt.tprintf("%d", page))
		sb := strings.builder_make()
		vrf_url(&sb, "/titles", &p)
		url := strings.to_string(sb)
		body, ok := abi.http_body(url, kHeaders)
		strings.builder_destroy(&sb)
		if !ok {
			break
		}
		resp: Titles_Resp
		if json.unmarshal_string(body, &resp) != nil || len(resp.Items) == 0 {
			break
		}
		for it in resp.Items {
			append(&out, Manga_Out{
				ID        = it.Hid,
				Title     = sanitize_title(it.Title),
				Cover_URL = it.Poster.Medium,
			})
		}
		last := resp.Meta.LastPage
		if last < 1 {
			last = 1
		}
		if !resp.Meta.HasNext || page >= last || len(out) >= 2000 {
			break
		}
		page += 1
	}
	return abi.to_json(out[:])
}

// ─── detail ──────────────────────────────────────────────────────────────────

detail_impl :: proc (arg: string) -> string {
	hid := decode_id(arg)
	if hid == "" {
		return "null"
	}

	p := vrf_params_make()
	defer vrf_params_destroy(&p)
	sb := strings.builder_make()
	vrf_url(&sb, fmt.tprintf("/titles/%s", hid), &p)
	url := strings.to_string(sb)
	body, ok := abi.http_body(url, kHeaders)
	strings.builder_destroy(&sb)
	if !ok {
		return "null"
	}
	resp: Detail_Resp
	if json.unmarshal_string(body, &resp) != nil {
		return "null"
	}
	d := resp.Data
	if d.Hid == "" && d.Title == "" {
		return "null"
	}

	genres := make([dynamic]string, 0, len(d.Genres), context.temp_allocator)
	for g in d.Genres {
		append(&genres, g.Title)
	}
	authors := make([dynamic]string, 0, len(d.Authors), context.temp_allocator)
	for a in d.Authors {
		append(&authors, a.Title)
	}

	id := d.Hid
	if id == "" {
		id = hid
	}
	return abi.to_json(Manga_Out{
		ID        = id,
		Title     = sanitize_title(d.Title),
		Desc      = abi.host_str("text.strip_html", d.SynopsisHTML),
		Cover_URL = d.Poster.Medium,
		Status    = normalize_status(d.Status),
		Genres    = genres[:],
		Author    = strings.join(authors[:], ", "),
	})
}

// ─── chapters ────────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	hid := decode_id(arg)
	if hid == "" {
		return "[]"
	}

	out := make([dynamic]Chapter_Out, 0, 200)
	defer delete(out)

	for page in 1 ..= 3 {
		p := vrf_params_make()
		vrf_set(&p, "language", "en")
		vrf_set(&p, "limit", "200")
		vrf_set(&p, "order", "desc")
		vrf_set(&p, "page", fmt.tprintf("%d", page))
		vrf_set(&p, "sort", "number")
		sb := strings.builder_make()
		vrf_url(&sb, fmt.tprintf("/titles/%s/chapters", hid), &p)
		url := strings.to_string(sb)
		body, ok := abi.http_body(url, kHeaders)
		strings.builder_destroy(&sb)
		vrf_params_destroy(&p)
		if !ok {
			break
		}
		resp: Chapters_Resp
		if json.unmarshal_string(body, &resp) != nil || len(resp.Items) == 0 {
			break
		}
		for c in resp.Items {
			entry := Chapter_Out{
				ID          = fmt.tprintf("%d", c.ID),
				Manga_ID    = hid,
				Chapter_Num = c.Number,
				Title       = c.Name,
			}
			// createdAt is unix seconds; a missing one stays absent rather
			// than becoming the 1970 timestamp.
			if c.CreatedAt > 0 {
				entry.Released_At = abi.host_str("text.date_to_iso", fmt.tprintf("%d", c.CreatedAt))
			}
			append(&out, entry)
		}
		last := resp.Meta.LastPage
		if last < 1 {
			last = 1
		}
		if page >= last || !resp.Meta.HasNext {
			break
		}
	}

	// Newest-first ABI order. stable: chapters sharing a number (37.1 vs 37.1)
	// keep the API's order instead of shuffling between runs.
	slice.stable_sort_by(out[:], proc (a, b: Chapter_Out) -> bool { return a.Chapter_Num > b.Chapter_Num })
	return abi.to_json(out[:])
}

// ─── pages ───────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	chapter_id := decode_id(arg)
	if chapter_id == "" {
		return "[]"
	}

	p := vrf_params_make()
	defer vrf_params_destroy(&p)
	sb := strings.builder_make()
	vrf_url(&sb, fmt.tprintf("/chapters/%s", chapter_id), &p)
	url := strings.to_string(sb)
	body, ok := abi.http_body(url, kHeaders)
	strings.builder_destroy(&sb)
	if !ok {
		return "[]"
	}
	resp: Pages_Resp
	if json.unmarshal_string(body, &resp) != nil || len(resp.Data.Pages) == 0 {
		return "[]"
	}

	out := make([dynamic]Page_Out, 0, len(resp.Data.Pages))
	defer delete(out)
	for pg, i in resp.Data.Pages {
		append(&out, Page_Out{
			Index   = i,
			URL     = pg.URL,
			Headers = Page_Headers{Referer = kReferer},
		})
	}
	return abi.to_json(out[:])
}

// ─── genres ──────────────────────────────────────────────────────────────────
// Seeded server-side and stable; the API lists no genre endpoint, so this
// table is carried here and only needs revisiting if the site adds a genre.

GENRES := []Genre_Out{
	{Name = "Action", Slug = "1"},
	{Name = "Adult", Slug = "268929"},
	{Name = "Adventure", Slug = "78"},
	{Name = "Avant Garde", Slug = "3"},
	{Name = "Boys Love", Slug = "4"},
	{Name = "Comedy", Slug = "5"},
	{Name = "Crime", Slug = "268921"},
	{Name = "Demons", Slug = "77"},
	{Name = "Drama", Slug = "6"},
	{Name = "Ecchi", Slug = "7"},
	{Name = "Fantasy", Slug = "79"},
	{Name = "Girls Love", Slug = "9"},
	{Name = "Gourmet", Slug = "10"},
	{Name = "Harem", Slug = "11"},
	{Name = "Hentai", Slug = "268930"},
	{Name = "Historical", Slug = "268922"},
	{Name = "Horror", Slug = "530"},
	{Name = "Isekai", Slug = "13"},
	{Name = "Iyashikei", Slug = "531"},
	{Name = "Josei", Slug = "15"},
	{Name = "Kids", Slug = "532"},
	{Name = "Magic", Slug = "539"},
	{Name = "Magical Girls", Slug = "268923"},
	{Name = "Mahou Shoujo", Slug = "533"},
	{Name = "Martial Arts", Slug = "534"},
	{Name = "Mature", Slug = "268931"},
	{Name = "Mecha", Slug = "19"},
	{Name = "Medical", Slug = "268924"},
	{Name = "Military", Slug = "535"},
	{Name = "Music", Slug = "21"},
	{Name = "Mystery", Slug = "22"},
	{Name = "Parody", Slug = "23"},
	{Name = "Philosophical", Slug = "268925"},
	{Name = "Psychological", Slug = "536"},
	{Name = "Reverse Harem", Slug = "25"},
	{Name = "Romance", Slug = "26"},
	{Name = "School", Slug = "73"},
	{Name = "Sci-Fi", Slug = "28"},
	{Name = "Seinen", Slug = "537"},
	{Name = "Shoujo", Slug = "30"},
	{Name = "Shounen", Slug = "31"},
	{Name = "Slice of Life", Slug = "538"},
	{Name = "Smut", Slug = "268932"},
	{Name = "Space", Slug = "33"},
	{Name = "Sports", Slug = "34"},
	{Name = "Super Power", Slug = "75"},
	{Name = "Superhero", Slug = "268926"},
	{Name = "Supernatural", Slug = "76"},
	{Name = "Suspense", Slug = "37"},
	{Name = "Thriller", Slug = "38"},
	{Name = "Tragedy", Slug = "268927"},
	{Name = "Vampire", Slug = "39"},
	{Name = "Wuxia", Slug = "268928"},
}

genres_impl :: proc () -> string {
	return abi.to_json(GENRES)
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
