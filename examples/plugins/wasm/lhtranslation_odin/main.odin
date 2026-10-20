// LHTranslation (lhtranslation.net) source plugin for goIsekai — Odin wasip1 reactor.
//
// Behavioural port of examples/plugins/lua/lhtranslation/main.lua, which stays
// the reference implementation: the Lua original is never deleted, only
// app_data/plugins/lhtranslation/ is swapped to the wasm build.
//
// WordPress + Madara theme. Every read is a scrape; there is no JSON API.
//
//   search    GET /?s=<q>&post_type=wp-manga[&paged=N]
//             GET /manga-genre/<slug>/[page/N/]        (genre filter)
//   detail    GET /manga/<slug>/
//   chapters  POST /manga/<slug>/ajax/chapters/        (X-Requested-With)
//   pages     GET /manga/<slug>/<chapter-slug>/
//   genres    static list (the site's Madara genre vocabulary)
//
// Load-bearing details, verified against the live site rather than assumed:
//
// 1. The detail page carries NO `li.wp-manga-chapter` markup at all — Madara
//    loads the list through an AJAX POST. Scraping the detail page for chapters
//    silently yields zero, so the POST is mandatory.
// 2. `img[data-src]` values on the page start with tabs and newlines; the URL is
//    only usable after trimming.
// 3. The detail page has two `<h1>`-ish blocks (site title and manga title); the
//    manga one lives inside `div.post-title`, so that block is scoped first.
//
// Deliberate delta over the Lua original:
//
// - The genre slug is derived as `lower(name)` with spaces hyphenated, instead of
//   URL-encoding the display name. The Lua sends "Gender Bender" as
//   `Gender%20Bender`, which the site answers with **404** — verified live, while
//   `gender-bender` returns 200. Deriving the slug fixes that filter and leaves
//   the other 45 unchanged (`Action` and `action` both resolve).
package lhtranslation_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:strings"

kSite :: "https://lhtranslation.net"
kAjaxHeaders :: `{"X-Requested-With":"xmlhttprequest"}`

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

// ─── helpers ────────────────────────────────────────────────────────────────

// decode_id unwraps the single-string ABI payload: a bare quoted id or an
// object with an `id` field both reduce to the bare value.
decode_id :: proc (arg: string) -> string {
	v: string
	if json.unmarshal_string(arg, &v) != nil {
		v = strings.trim_space(arg)
	}
	return strings.trim_space(v)
}

// genre_slug is the /manga-genre/<slug>/ segment: the display name lowercased,
// with spaces turned into hyphens. The site 404s the raw space-encoded name.
genre_slug :: proc (name: string) -> string {
	lower := strings.to_lower(strings.trim_space(name))
	return abi.sub(lower, `\s+`, "-")
}

// first_of returns the first non-empty capture, which is how the Lua source
// expresses `data-src` else `src` fallbacks.
first_of :: proc (subject: string, patterns: ..string) -> string {
	for p in patterns {
		if v := abi.find_one(subject, p); v != "" {
			return v
		}
	}
	return ""
}

// ─── search ─────────────────────────────────────────────────────────────────

search_impl :: proc (arg: string) -> string {
	f: Search_Filter
	if json.unmarshal_string(arg, &f) != nil {
		return "[]"
	}
	// The ABI sends page 0 when it means the first page.
	page := f.Page
	if page < 1 {
		page = 1
	}

	url: string
	if len(f.Genres) > 0 {
		// Genre archive; page N is /page/N/ past the slug.
		base := strings.concatenate({kSite, "/manga-genre/", abi.url_encode(genre_slug(f.Genres[0])), "/"})
		if page > 1 {
			url = strings.concatenate({base, "page/", fmt.aprintf("%d", page), "/"})
		} else {
			url = base
		}
	} else {
		base := strings.concatenate({kSite, "/?s=", abi.url_encode(strings.trim_space(f.Query)), "&post_type=wp-manga"})
		if page > 1 {
			url = strings.concatenate({base, "&paged=", fmt.aprintf("%d", page)})
		} else {
			url = base
		}
	}

	body, ok := abi.get_body(url)
	if !ok {
		return "[]"
	}

	// Each card is an <a href=".../manga/<slug>/" title="..."> whose block also
	// holds the lazyloaded cover. Walk the anchors so the three fields stay
	// paired; a naive global scan of data-src would drift out of step.
	//
	// No `^` anchor here: RE2 (the host's regex engine) only matches `^` at the
	// start of the whole subject, so an anchored pattern finds exactly one card.
	pairs := abi.find_all_pairs(body, `(?s)<a\s+href="([^"]*/manga/[^"]+)"\s+title="([^"]*)"[^>]*>(.*?)</a>`)
	out := make([dynamic]Manga_Out, 0, len(pairs))
	defer delete(out)
	for row in pairs {
		if len(row) < 3 {
			continue
		}
		href, title, inner := row[0], row[1], row[2]
		slug := abi.find_one(href, `/manga/([^/]+)/$`)
		if slug == "" {
			continue
		}
		cover := first_of(inner, `data-src="([^"]+)"`, `\ssrc="([^"]+)"`)
		// Madara's placeholder is a "dflazy" blank; treat it as no cover.
		if strings.contains(cover, "dflazy") {
			cover = ""
		}
		append(&out, Manga_Out {
			ID = slug,
			Title = abi.host_str("text.unescape", strings.trim_space(title)),
			Cover_URL = strings.trim_space(cover),
		})
	}
	return abi.to_json(out[:])
}

// ─── detail ─────────────────────────────────────────────────────────────────

detail_impl :: proc (arg: string) -> string {
	slug := decode_id(arg)
	if slug == "" {
		return "{}"
	}

	body, ok := abi.get_body(strings.concatenate({kSite, "/manga/", slug, "/"}))
	if !ok {
		// Failure shape for an object-returning export: bare {id}. The host does
		// not cache it, so the next call retries; an [] would not decode.
		return abi.to_json(Manga_Out{ID = slug})
	}

	// Cover lives in div.summary_image, lazyloaded through data-src.
	cover_block := abi.find_one(body, `(?s)class="summary_image".*?</a>`)
	cover := first_of(cover_block, `data-src="([^"]+)"`, `\ssrc="([^"]+)"`)

	// Two title-ish blocks exist on the page; scope to div.post-title first.
	tblock := abi.find_one(body, `(?s)class="post-title[^"]*">(.*?)</div>`)
	title := abi.host_str("text.unescape", strings.trim_space(
		abi.find_one(tblock, `(?s)<h[1-6][^>]*>(.*?)</h[1-6]>`),
	))

	// Author: anchor list inside div.author-content.
	author := abi.sub(
		abi.host_str("text.strip_html", abi.find_one(body, `(?s)class="author-content">(.*?)</div>`)),
		`,`, ", ",
	)

	// Genres: the anchors inside div.genres-content, comma-separated on the page.
	gblock := abi.find_one(body, `(?s)class="genres-content">(.*?)</div>`)
	raw_genres := abi.find_all(gblock, `<a[^>]*>([^<]+)</a>`)
	genres := make([dynamic]string, 0, len(raw_genres))
	defer delete(genres)
	for g in raw_genres {
		if n := strings.trim_space(g); n != "" {
			append(&genres, n)
		}
	}

	// Status: the "Status" heading's following summary-content (the block also
	// carries Release, which precedes it).
	sblock := abi.find_one(body, `(?s)class="post-status">(.*?)$`)
	status := abi.host_str("text.normalize_status", strings.trim_space(
		abi.find_one(sblock, `(?s)Status\s*</h5>.*?class="summary-content">\s*([^<]+)`),
	))

	// Description: summary__content, cut at the "read more" span so the toggle
	// markup and anything after it stay out.
	dblock := abi.find_one(body, `(?s)class="summary__content[^"]*">(.*?)<span\s+class="[^"]*content-readmore"`)
	if dblock == "" {
		dblock = abi.find_one(body, `(?s)class="summary__content[^"]*">(.*?)</div>`)
	}
	desc := abi.sub(abi.host_str("text.strip_html", dblock), `\s+`, " ")

	return abi.to_json(Manga_Out {
		ID = slug,
		Title = title,
		Cover_URL = strings.trim_space(cover),
		Author = strings.trim_space(author),
		Desc = strings.trim_space(desc),
		Status = status,
		Genres = genres[:],
	})
}

// ─── chapters ───────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	slug := decode_id(arg)
	if slug == "" {
		return "[]"
	}

	// Mandatory AJAX POST: the detail page has no chapter list in its markup.
	body, ok := abi.http_post_body(
		strings.concatenate({kSite, "/manga/", slug, "/ajax/chapters/"}),
		"",
		kAjaxHeaders,
	)
	if !ok {
		return "[]"
	}

	items := abi.find_all(body, `(?s)<li\s+class="wp-manga-chapter[^>]*>(.*?)</li>`)
	out := make([dynamic]Chapter_Out, 0, len(items))
	defer delete(out)
	for li in items {
		href := abi.find_one(li, `href="([^"]+)"`)
		if href == "" {
			continue
		}
		label := strings.trim_space(abi.find_one(li, `>([^<]*[Cc]hapter[^<]*)<`))
		if label == "" {
			continue
		}
		// Chapter slug out of ".../manga/<slug>/<chapter-slug>/".
		cid := abi.find_one(href, `/manga/[^/]+/([^/]+)/$`)

		raw_date := first_of(
			li,
			`chapter-release-date[^>]*>\s*<i[^>]*>([^<]+)</i>`,
			`chapter-release-date[^>]*>\s*<a[^>]*>([^<]+)</a>`,
		)

		// The id is "slug:chapter-slug" so it round-trips into get_page_list,
		// which only has to swap the colon for a slash.
		id_part := cid
		if id_part == "" {
			id_part = label
		}
		append(&out, Chapter_Out {
			ID = strings.concatenate({slug, ":", id_part}),
			Manga_ID = slug,
			Title = label,
			Chapter_Num = abi.chapter_num(label),
			Released_At = abi.date_to_iso(strings.trim_space(raw_date)),
			URL = href,
		})
	}
	return abi.to_json(out[:])
}

// ─── pages ──────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	// Chapter id is "slug:chapter-slug"; the page URL is the same with a slash.
	// replace_all returns (output, count) in Odin, so the count is discarded.
	path, _ := strings.replace_all(decode_id(arg), ":", "/")
	if path == "" {
		return "[]"
	}

	body, ok := abi.get_body(strings.concatenate({kSite, "/manga/", path, "/"}))
	if !ok {
		return "[]"
	}

	// Every page image carries the wp-manga-chapter-img class. Take the whole tag
	// and pull data-src (else src); the value needs trimming — on the live site it
	// is indented with tabs and newlines.
	tags := abi.find_all(body, `(?s)<img[^>]*wp-manga-chapter-img[^>]*>`)
	out := make([dynamic]Page_Out, 0, len(tags))
	defer delete(out)
	for tag in tags {
		src := first_of(tag, `data-src="([^"]+)"`, `\ssrc="([^"]+)"`)
		if src = strings.trim_space(src); src == "" {
			continue
		}
		append(&out, Page_Out{Index = len(out), URL = src})
	}
	return abi.to_json(out[:])
}

// ─── genres (optional export) ───────────────────────────────────────────────

// The site's Madara genre vocabulary. Kept static like the Lua source, but the
// slug is derived (lower, hyphenated) instead of echoing the display name —
// "Gender Bender" as a space-encoded name is a 404 on the live site.
genres_impl :: proc () -> string {
	names := []string{
		"Action", "Adult", "Adventure", "Anime", "Comedy", "Comic", "Cooking",
		"Drama", "Ecchi", "Fantasy", "Game", "Gender Bender", "Harem",
		"Historical", "Horror", "Isekai", "Josei", "LGBTQ", "Love", "Magic",
		"Mecha", "Medical", "Music", "Mystery", "Parody", "Philosophical",
		"Political", "Post-apocalypse", "Psychological", "Romance", "School Life",
		"Sci-Fi", "Seinen", "Shoujo", "Shounen", "Slice of Life", "Smut", "Sports",
		"Super Power", "Supernatural", "Survival", "Tragedy", "War", "Wuxia",
		"Yaoi", "Yuri",
	}
	out := make([]Genre_Out, len(names))
	for n, i in names {
		out[i] = Genre_Out{Name = n, Slug = genre_slug(n)}
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
