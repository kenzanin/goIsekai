// Comizy (comizy.io) source plugin for goIsekai — Odin wasip1 reactor.
//
// Behavioural port of examples/plugins/lua/comizy/main.lua, which stays as the
// reference implementation: a successful port keeps its Lua original in
// examples/ next to the wasm build, and only app_data/plugins/<id>/ is swapped.
//
// comizy is a Next.js site with an API-first backend, so there is no HTML
// parsing and no regex:
//
//	version  GET https://comizy.io/api/version        -> {buildId}
//	search   GET https://api.comizy.io/titles/search?q=<q>&page=N&limit=24
//	genres   GET https://api.comizy.io/genres
//	detail   GET https://comizy.io/_next/data/<bid>/<slug>.json?slug=<slug>
//	chapters GET https://api.comizy.io/titles/<mangaId>/chapters
//	pages    GET https://comizy.io/_next/data/<bid>/<mSlug>/<cSlug>.json?slug=<mSlug>
//
// Three things are load-bearing and copied from the Lua source, not guessed:
//
//  1. buildId rotates per deploy and every /_next/data/ path needs the current
//     one, so it is fetched from /api/version and cached for the VM lifetime.
//
//  2. Neither id derives the other: the chapter endpoint needs the opaque
//     mangaId while the /_next/data/ paths need the slug. Manga.ID is therefore
//     the composite "<mangaId>|<slug>", and a chapter's ID is
//     "<mangaSlug>|<chapterSlug>" for GetPageList.
//
//  3. Chapter output mirrors the Lua source, which sets neither manga_id nor
//     url, but it now also carries chapter_num. The Lua source shipped 0
//     because the Odin port was written when reading progress was believed to
//     be keyed off chapter_num; it is not - progress is keyed off the chapter's
//     source id - and a source with no numbers cannot be ordered at all.
//
//  4. The site labels a split chapter "Chapter 12-2" and a decimal one
//     "Chapter 12.1". text.chapter_num reads the leading number only, so the
//     hyphen form collapsed to 12 and collided with its sibling. Fold the
//     hyphen into the decimal the dot form already uses.
package comizy_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:strings"

kAPI :: "https://api.comizy.io"
kWEB :: "https://comizy.io"
kHeaders :: `{"accept":"application/json"}`

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
	Author:    string   `json:"author,omitempty"`,
	Desc:      string   `json:"description,omitempty"`,
	Status:    string   `json:"status,omitempty"`,
	Genres:    []string `json:"genres,omitempty"`,
}

// Chapter_Out carries the same fields the Lua source produced, zero-valued
// where it left them out (see the header).
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

Version_Resp :: struct {
	Build_ID: string `json:"buildId"`,
}

// Search_Item is shared by the REST search and the /_next/data/ genre browse:
// the SSR payload carries the same {id,slug,name,cover} shape.
Search_Item :: struct {
	ID:    string `json:"id"`,
	Slug:  string `json:"slug"`,
	Name:  string `json:"name"`,
	Cover: string `json:"cover"`,
}

Search_Data :: struct {
	Items: []Search_Item `json:"items"`,
}

Search_Resp :: struct {
	Data: Search_Data `json:"data"`,
}

Genre_Browse_Props :: struct {
	Items: []Search_Item `json:"items"`,
}

Genre_Browse_Resp :: struct {
	Page_Props: Genre_Browse_Props `json:"pageProps"`,
}

Genre_Item :: struct {
	Name: string `json:"name"`,
	Slug: string `json:"slug"`,
}

Genres_Data :: struct {
	Items: []Genre_Item `json:"items"`,
}

Genres_Resp :: struct {
	Data: Genres_Data `json:"data"`,
}

Detail_Author :: struct {
	Name: string `json:"name"`,
}

Detail_Genre :: struct {
	Name: string `json:"name"`,
	Slug: string `json:"slug"`,
}

Initial_Manga :: struct {
	ID      : string         `json:"id"`,
	Name    : string         `json:"name"`,
	Slug    : string         `json:"slug"`,
	Cover   : string         `json:"cover"`,
	Status  : string         `json:"status"`,
	Summary : string         `json:"summary"`,
	Genres  : []Detail_Genre `json:"genres"`,
	Authors : []Detail_Author `json:"authors"`,
}

Detail_Props :: struct {
	Initial_Manga: Initial_Manga `json:"initialManga"`,
}

Detail_Resp :: struct {
	Page_Props: Detail_Props `json:"pageProps"`,
}

Chapter_Item :: struct {
	ID         : string `json:"id"`,
	Name       : string `json:"name"`,
	Slug       : string `json:"slug"`,
	Updated_At : string `json:"updated_at"`,
}

Chapters_Data :: struct {
	Chapters: []Chapter_Item `json:"chapters"`,
}

Chapters_Resp :: struct {
	Data: Chapters_Data `json:"data"`,
}

Pages_Chapter :: struct {
	Images: []string `json:"images"`,
}

Pages_Props :: struct {
	Initial_Chapter: Pages_Chapter `json:"initialChapter"`,
}

Pages_Resp :: struct {
	Page_Props: Pages_Props `json:"pageProps"`,
}

// ─── helpers ─────────────────────────────────────────────────────────────────

url_encode :: proc (s: string) -> string { return abi.url_encode(s) }

// decode_id unwraps the JSON the host passes the single-string ABI: a bare
// quoted id or an object ({"id":..}) both end up as the string.
decode_id :: proc (arg: string) -> string {
	v: string
	if json.unmarshal_string(arg, &v) != nil {
		v = strings.trim_space(arg)
	}
	return strings.trim_space(v)
}

// split_id cuts the composite "<a>|<b>" id both directions use. ok is false
// when there is no separator, which the Lua source treats as malformed.
split_id :: proc (id: string) -> (a, b: string, ok: bool) {
	i := strings.index_byte(id, '|')
	if i < 0 {
		return "", "", false
	}
	return id[:i], id[i + 1:], true
}

// build_id fetches the Next.js buildId, cached for the VM lifetime. The
// wasmHost keeps the module instantiated, so a package-level cache is per
// process; the host separately persists genres, which need no cache here.
cached_build_id: string
has_build_id: bool

build_id :: proc () -> string {
	if has_build_id {
		return cached_build_id
	}
	body, ok := abi.get_body(fmt.aprintf("%s/api/version", kWEB))
	if !ok {
		return ""
	}
	v: Version_Resp
	if json.unmarshal_string(body, &v) != nil || v.Build_ID == "" {
		return ""
	}
	cached_build_id = v.Build_ID
	has_build_id = true
	return cached_build_id
}

// date_to_iso normalises the site's timestamp; "" stays "" so the ABI omits
// released_at rather than emitting an epoch.
date_to_iso :: proc (s: string) -> string { return abi.date_to_iso(s) }

// comizy_chapter_num is the chapter number of a comizy chapter label. The site
// labels a split chapter "Chapter 12-2" and a decimal one "Chapter 12.1";
// text.chapter_num reads the leading number only, so the hyphen form collapsed
// to 12 and collided with its sibling. Fold the hyphen into the decimal the
// dot form already uses.
comizy_chapter_num :: proc (title: string) -> f64 {
	folded := abi.sub(title, `^(?i)(Chapter\s+\d+)\s*-\s*(\d+)\s*$`, "$1.$2")
	return abi.chapter_num(folded)
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
	page := f.Page
	if page < 1 {
		page = 1
	}

	items: []Search_Item
	if len(f.Genres) > 0 {
		// Genre browse goes through the SSR JSON: the REST search endpoint
		// silently ignores genre params, so this is the only genre path.
		bid := build_id()
		if bid == "" {
			return "[]"
		}
		g := url_encode(f.Genres[0])
		url := fmt.aprintf("%s/_next/data/%s/genres/%s.json?slug=%s", kWEB, bid, g, g)
		if page > 1 {
			url = fmt.aprintf("%s&page=%d", url, page)
		}
		body, ok := abi.get_body(url)
		if !ok {
			return "[]"
		}
		res: Genre_Browse_Resp
		if json.unmarshal_string(body, &res) != nil {
			return "[]"
		}
		items = res.Page_Props.Items
	} else {
		if q == "" {
			return "[]"
		}
		body, ok := abi.get_body(
			fmt.aprintf("%s/titles/search?q=%s&page=%d&limit=24", kAPI, url_encode(q), page),
		)
		if !ok {
			return "[]"
		}
		res: Search_Resp
		if json.unmarshal_string(body, &res) != nil {
			return "[]"
		}
		items = res.Data.Items
	}

	out := make([dynamic]Manga_Out, 0, len(items))
	defer delete(out)
	for it in items {
		if it.ID == "" || it.Slug == "" {
			continue
		}
		title := it.Name
		if title == "" {
			title = it.Slug
		}
		append(&out, Manga_Out{
			ID        = strings.concatenate({it.ID, "|", it.Slug}),
			Title     = title,
			Cover_URL = it.Cover,
		})
	}
	return abi.to_json(out[:])
}

// ─── genres ──────────────────────────────────────────────────────────────────

genres_impl :: proc () -> string {
	body, ok := abi.get_body(fmt.aprintf("%s/genres", kAPI))
	if !ok {
		return "[]"
	}
	res: Genres_Resp
	if json.unmarshal_string(body, &res) != nil {
		return "[]"
	}

	out := make([dynamic]Genre_Out, 0, len(res.Data.Items))
	defer delete(out)
	for g in res.Data.Items {
		if g.Slug == "" || g.Name == "" {
			continue
		}
		append(&out, Genre_Out{Name = g.Name, Slug = g.Slug})
	}
	return abi.to_json(out[:])
}

// ─── detail ──────────────────────────────────────────────────────────────────

detail_impl :: proc (arg: string) -> string {
	raw := decode_id(arg)
	if raw == "" {
		return "{}"
	}
	_, slug, has_slug := split_id(raw)
	if !has_slug {
		return abi.to_json(Manga_Out{ID = raw})
	}
	bid := build_id()
	if bid == "" {
		// Failure shape for an object export: bare {id}. The host does not
		// cache it, so the next call retries; an [] here would not decode.
		return abi.to_json(Manga_Out{ID = raw})
	}
	body, ok := abi.get_body(
		fmt.aprintf("%s/_next/data/%s/%s.json?slug=%s", kWEB, bid, slug, url_encode(slug)),
	)
	if !ok {
		return abi.to_json(Manga_Out{ID = raw})
	}
	res: Detail_Resp
	if json.unmarshal_string(body, &res) != nil {
		return abi.to_json(Manga_Out{ID = raw})
	}
	m := res.Page_Props.Initial_Manga
	if m.Name == "" && m.Slug == "" {
		return abi.to_json(Manga_Out{ID = raw})
	}

	genres := make([dynamic]string, 0, len(m.Genres))
	defer delete(genres)
	for g in m.Genres {
		n := g.Name
		if n == "" {
			n = g.Slug
		}
		if n != "" {
			append(&genres, n)
		}
	}

	author := ""
	if len(m.Authors) > 0 {
		author = m.Authors[0].Name
	}

	return abi.to_json(Manga_Out{
		ID        = raw,
		Title     = m.Name,
		Cover_URL = m.Cover,
		Author    = author,
		Desc      = m.Summary,
		Status    = abi.host_str("text.normalize_status", m.Status),
		Genres    = genres[:],
	})
}

// ─── chapters ────────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	raw := decode_id(arg)
	if raw == "" {
		return "[]"
	}
	mid, slug, ok := split_id(raw)
	if !ok {
		return "[]"
	}
	body, ok2 := abi.get_body(fmt.aprintf("%s/titles/%s/chapters", kAPI, url_encode(mid)))
	if !ok2 {
		return "[]"
	}
	res: Chapters_Resp
	if json.unmarshal_string(body, &res) != nil {
		return "[]"
	}

	out := make([dynamic]Chapter_Out, 0, len(res.Data.Chapters))
	defer delete(out)
	for c in res.Data.Chapters {
		if c.Slug == "" {
			continue
		}
		title := c.Name
		if title == "" {
			title = c.Slug
		}
		append(&out, Chapter_Out{
			ID          = strings.concatenate({slug, "|", c.Slug}),
			Title       = title,
			Chapter_Num = comizy_chapter_num(title),
			Released_At = date_to_iso(c.Updated_At),
		})
	}

	// The endpoint is already newest-first, which is the ABI order: the host
	// sorts its cached list by chapter_num descending and its reader resolves
	// prev/next assuming newest-first. Reversing here flipped the list.
	return abi.to_json(out[:])
}

// ─── pages ───────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	raw := decode_id(arg)
	if raw == "" {
		return "[]"
	}
	mslug, cslug, ok := split_id(raw)
	if !ok {
		return "[]"
	}
	bid := build_id()
	if bid == "" {
		return "[]"
	}
	body, ok2 := abi.get_body(
		fmt.aprintf(
			"%s/_next/data/%s/%s/%s.json?slug=%s",
			kWEB,
			bid,
			mslug,
			cslug,
			url_encode(mslug),
		),
	)
	if !ok2 {
		return "[]"
	}
	res: Pages_Resp
	if json.unmarshal_string(body, &res) != nil {
		return "[]"
	}

	imgs := res.Page_Props.Initial_Chapter.Images
	out := make([dynamic]Page_Out, 0, len(imgs))
	defer delete(out)
	for u in imgs {
		if u == "" {
			continue
		}
		// Index comes from the emitted position, so a dropped item cannot make
		// the remaining pages shift.
		append(&out, Page_Out{Index = len(out), URL = u})
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
