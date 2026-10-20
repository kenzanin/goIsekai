// MangaRead (mangaread.org) source plugin for goIsekai — Odin wasip1 reactor.
//
// Behavioural port of examples/plugins/lua/mangaread/main.lua, which stays the
// reference implementation: the Lua original is never deleted, only
// app_data/plugins/mangaread/ is swapped to the wasm build.
//
// WordPress + Madara theme. Every read is a scrape.
//
//   search    GET /?s=<q>&post_type=wp-manga
//             GET /genres/<slug>/[page/N/]            (genre filter)
//   detail    GET /manga/<slug>/
//   chapters  POST /manga/<slug>/ajax/chapters/      (X-Requested-With)
//   pages     GET /manga/<slug>/<chapter-slug>/
//   genres    static list (the site's nav vocabulary)
//
// Same Madara traps already documented for lhtranslation, all re-verified here:
// the detail page carries no `li.wp-manga-chapter` markup (the list only exists
// behind the AJAX POST), and `img[data-src]` values can start with tabs and
// newlines, so every URL needs trimming before use.
//
// Search is parsed per card rather than by scanning a window around each slug:
// the first occurrence of a slug is the `og:url` in <head>, which made the old
// window miss the card entirely (item #1 lost its cover) and far-apart markup
// let the fallback regex steal a neighbouring card's cover. The card block is
// taken up to the `post-title` close, and the cover always precedes it inside
// the same card.
//
// Deliberate deltas over the Lua original:
//
// - `manga_id` is filled on every chapter (Lua emits "").
// - The chapter dedupe key is the chapter id, and the fallback branch fills
//   manga_id and released_at too, matching the primary path.
package mangaread_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:strings"

kSite :: "https://www.mangaread.org"
kAjaxHeaders :: `{"X-Requested-With":"xmlhttprequest","Content-Type":"application/x-www-form-urlencoded"}`

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

decode_id :: proc (arg: string) -> string {
	v: string
	if json.unmarshal_string(arg, &v) != nil {
		v = strings.trim_space(arg)
	}
	return strings.trim_space(v)
}

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
	page := f.Page
	if page < 1 {
		page = 1
	}

	url: string
	if len(f.Genres) > 0 {
		// Madara genre archives page as /genres/<slug>/page/N/, not ?page=N.
		base := strings.concatenate({kSite, "/genres/", abi.url_encode(strings.trim_space(f.Genres[0])), "/"})
		if page > 1 {
			url = strings.concatenate({base, "page/", fmt.aprintf("%d", page), "/"})
		} else {
			url = base
		}
	} else {
		// The admin-ajax.php search endpoint returns 0 bytes; the server-rendered
		// GET is the working path.
		url = strings.concatenate({kSite, "/?s=", abi.url_encode(strings.trim_space(f.Query)), "&post_type=wp-manga"})
	}

	body, ok := abi.get_body(url)
	if !ok {
		return "[]"
	}

	cards := abi.find_all(body, `(?s)<div[^>]*class="[^"]*c-tabs-item__content.*?post-title.*?</h3>`)
	out := make([dynamic]Manga_Out, 0, len(cards))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)
	for card in cards {
		href := abi.find_one(card, `href="([^"]*/manga/[^"/]+/)"`)
		if href == "" {
			continue
		}
		slug := first_of(href, `/manga/([^/]+)/`, `/manga/([^/]+)$`)
		if slug == "" || slug in seen {
			continue
		}
		seen[slug] = true

		// The cover lives in the same card, before the title. Prefer data-src
		// (lazyload) over src, and never take the srcset width descriptors.
		cover := first_of(card, `data-src="[\t\n\s]*(https?://[^"]+)"`, `\ssrc="(https?://[^"]+)"`)
		append(&out, Manga_Out {
			ID = slug,
			Title = abi.host_str("text.unescape", strings.trim_space(abi.find_one(card, `<h3[^>]*><a[^>]*>([^<]+)</a>`))),
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

	cover_block := abi.find_one(body, `(?s)class="summary_image".*?</a>`)
	cover := first_of(cover_block, `data-src="([^"]+)"`, `src="([^"]+)"`)

	tblock := abi.find_one(body, `(?s)class="post-title[^"]*">(.*?)</div>`)
	title := abi.host_str("text.unescape", strings.trim_space(
		abi.find_one(tblock, `(?s)<h[1-6][^>]*>(.*?)</h[1-6]>`),
	))

	author := abi.sub(
		abi.host_str("text.strip_html", abi.find_one(body, `(?s)class="author-content">(.*?)</div>`)),
		`,`, ", ",
	)

	gblock := abi.find_one(body, `(?s)class="genres-content">(.*?)</div>`)
	raw_genres := abi.find_all(gblock, `<a[^>]*>([^<]+)</a>`)
	genres := make([dynamic]string, 0, len(raw_genres))
	defer delete(genres)
	for g in raw_genres {
		if n := strings.trim_space(g); n != "" {
			append(&genres, n)
		}
	}

	// Status: the "Status" heading's summary-content. The post-status block also
	// carries Release, which precedes it.
	sblock := abi.find_one(body, `(?s)class="post-status">(.*?)$`)
	status := abi.host_str("text.normalize_status", strings.trim_space(
		abi.find_one(sblock, `(?s)Status\s*</h5>.*?class="summary-content">\s*([^<]+)`),
	))

	// Description: cut at the "read more" span so the toggle markup stays out.
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

	items := abi.find_all(body, `(?s)<li[^>]*class="wp-manga-chapter[^"]*"[^>]*>(.*?)</li>`)
	out := make([dynamic]Chapter_Out, 0, len(items))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)

	for li in items {
		href := abi.find_one(li, `href="([^"]+)"`)
		if href == "" {
			continue
		}
		label := strings.trim_space(abi.find_one(li, `>([^<]*[Cc]hapter[^<]*)<`))
		if label == "" {
			continue
		}
		cid := first_of(href, `/manga/[^/]+/([^/]+)/?$`, `/([^/]+)/?$`)
		if cid == "" || cid in seen {
			continue
		}
		seen[cid] = true
		raw_date := abi.find_one(li, `(?s)chapter-release-date[^>]*>.*?<i>([^<]+)</i>`)
		append(&out, Chapter_Out {
			ID = strings.concatenate({slug, ":", cid}),
			Manga_ID = slug,
			Title = label,
			Chapter_Num = abi.chapter_num(label),
			Released_At = abi.date_to_iso(strings.trim_space(raw_date)),
			URL = href,
		})
	}

	// Fallback for pages whose AJAX payload uses a different wrapper: pull any
	// /chapter- link straight out of the body.
	if len(out) == 0 {
		pairs := abi.find_all_pairs(body, `<a[^>]*href="([^"]*chapter-[^"]+)"[^>]*>\s*([^<]+)\s*</a>`)
		for row in pairs {
			if len(row) < 2 {
				continue
			}
			href, label := row[0], strings.trim_space(row[1])
			cid := abi.find_one(href, `/([^/]+)/?$`)
			if cid == "" || cid in seen {
				continue
			}
			seen[cid] = true
			append(&out, Chapter_Out {
				ID = strings.concatenate({slug, ":", cid}),
				Manga_ID = slug,
				Title = label,
				Chapter_Num = abi.chapter_num(label),
				URL = href,
			})
		}
	}
	return abi.to_json(out[:])
}

// ─── pages ──────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	// Chapter id is "slug:chapter-slug"; the page URL is the same with a slash.
	path, _ := strings.replace_all(decode_id(arg), ":", "/")
	if path == "" {
		return "[]"
	}

	body, ok := abi.get_body(strings.concatenate({kSite, "/manga/", path, "/"}))
	if !ok {
		return "[]"
	}

	tags := abi.find_all(body, `(?s)<img[^>]*wp-manga-chapter-img[^>]*>`)
	out := make([dynamic]Page_Out, 0, len(tags))
	defer delete(out)
	for tag in tags {
		// data-src may carry tabs and newlines before the URL.
		src := first_of(tag,
			`data-src="[\t\n\s]*(https?://[^"]+)"`,
			`data-src="(https?://[^"]+)"`,
			`src="[\t\n\s]*(https?://[^"]+)"`,
			`src="(https?://[^"]+)"`,
		)
		if src = strings.trim_space(src); src == "" {
			continue
		}
		// The Lua numbers pages from 0 (`index = #pages` before appending).
		append(&out, Page_Out{Index = len(out), URL = src})
	}
	return abi.to_json(out[:])
}

// ─── genres (optional export) ───────────────────────────────────────────────

// Slugs as they appear in /genres/<slug>, taken from the site navigation.
genres_impl :: proc () -> string {
	out := []Genre_Out{
		{Name = "Action", Slug = "action"},
		{Name = "Adventure", Slug = "adventure"},
		{Name = "Comedy", Slug = "comedy"},
		{Name = "Comic", Slug = "comic"},
		{Name = "Cooking", Slug = "cooking"},
		{Name = "Doujinshi", Slug = "doujinshi"},
		{Name = "Drama", Slug = "drama"},
		{Name = "Ecchi", Slug = "ecchi"},
		{Name = "Fantasy", Slug = "fantasy"},
		{Name = "Gender Bender", Slug = "gender-bender"},
		{Name = "Harem", Slug = "harem"},
		{Name = "Historical", Slug = "historical"},
		{Name = "Horror", Slug = "horror"},
		{Name = "Isekai", Slug = "isekai"},
		{Name = "Josei", Slug = "josei"},
		{Name = "Manhua", Slug = "manhua"},
		{Name = "Manhwa", Slug = "manhwa"},
		{Name = "Manga", Slug = "manga"},
		{Name = "Martial Arts", Slug = "martial-arts"},
		{Name = "Mecha", Slug = "mecha"},
		{Name = "Mystery", Slug = "mystery"},
		{Name = "One shot", Slug = "one-shot"},
		{Name = "Psychological", Slug = "psychological"},
		{Name = "Romance", Slug = "romance"},
		{Name = "School Life", Slug = "school-life"},
		{Name = "Sci-fi", Slug = "sci-fi"},
		{Name = "Seinen", Slug = "seinen"},
		{Name = "Shoujo", Slug = "shoujo"},
		{Name = "Shoujo Ai", Slug = "shoujo-ai"},
		{Name = "Shounen", Slug = "shounen"},
		{Name = "Shounen Ai", Slug = "shounen-ai"},
		{Name = "Slice of Life", Slug = "slice-of-life"},
		{Name = "Smut", Slug = "smut"},
		{Name = "Sports", Slug = "sports"},
		{Name = "Supernatural", Slug = "supernatural"},
		{Name = "Tragedy", Slug = "tragedy"},
		{Name = "Webtoon", Slug = "webtoon"},
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
