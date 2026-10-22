// Comicasura (comicasura.net) source plugin for goIsekai — Odin wasip1 reactor.
//
// This source has no content API: everything is scraped from server-rendered
// HTML (Laravel). Endpoints, established from comicasura.har + comicasura2.har
// and then re-checked live:
//   search   GET /advanced-search?name=<q>&page=<n>
//   detail   GET /manga/<slug>            (h1, og:image, Status, Overview, genres)
//   chapters GET /manga/<slug>            (same page: chapter links)
//   pages    GET /manga/<slug>/<chapter>  (reader <img> tags)
//   genres   GET /genres
//
// Four behaviours are load-bearing:
//
//  1. Image hosts are hotlink-protected: the same URL answers 403 without a
//     Referer and 200 with one. Nothing to do here — the host proxy supplies
//     it — but it is why a bare curl looks like the source is broken.
//
//  2. The chapter list only carries a `title` attribute on its first and last
//     entry; the other 60+ links are bare anchors. The label is therefore
//     derived from the id ("chapter-28-1" -> "Chapter 28.1"), which matches
//     the rendered label exactly.
//
//  3. Cover thumbs are missing for roughly a quarter of the catalogue (they
//     404 on the CDN). Search therefore takes its slug list from the anchors
//     and attaches a cover only where one resolves — never by zipping the two
//     lists, which would shift every cover onto the wrong title.
//
//  4. GetPageList receives one string but this source needs manga + chapter,
//     so Chapter.ID is the composite "<slug>:<chapter-id>".
//
// Network, text and markup work all go through abi/ (host_call): http_body and
// the host's regex.find_all / text.strip_html / text.chapter_num helpers, so
// there is no hand-rolled HTML or text logic here.
package comicasura_odin

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:slice"
import "core:strings"

// The site origin, and the Referer derived from it, come from plugin.json so a
// site move is one edit there rather than three strings kept in sync by hand.
// Procs rather than package constants: the host dispatcher only answers after
// _initialize has run, so a host_call in a global initializer would be too early.
site :: proc () -> string {
	s := abi.host_str("plugin.param", "site_url")
	if s == "" {
		return "https://comicasura.net"
	}
	return s
}

// headers derives the Referer from the same origin.
headers :: proc () -> string {
	return strings.concatenate({`{"Accept":"text/html,application/xhtml+xml","Referer":"`, site(), `/"}`})
}

url_encode :: proc (s: string) -> string { return abi.url_encode(s) }

// find_all returns every match of a single-group pattern, in document order.
find_all :: proc (subject, pattern: string) -> []string { return abi.find_all(subject, pattern) }

// find_all_pairs returns every match of a two-group pattern.
find_all_pairs :: proc (subject, pattern: string) -> [][]string { return abi.find_all_pairs(subject, pattern) }

// find_one returns the first group of the first match, "" when absent.
find_one :: proc (subject, pattern: string) -> string { return abi.find_one(subject, pattern) }

// basename_slug pulls "<slug>" out of ".../thumb/<slug>.webp".
basename_slug :: proc (url: string) -> string {
	i := strings.last_index_byte(url, '/')
	rest := url
	if i >= 0 {
		rest = url[i + 1:]
	}
	if d := strings.last_index_byte(rest, '.'); d > 0 {
		rest = rest[:d]
	}
	return rest
}

// chapter_label turns "chapter-28-1" into "Chapter 28.1", matching the label
// the site renders for the same chapter.
chapter_label :: proc (chapter_id: string) -> string {
	rest := strings.trim_prefix(chapter_id, "chapter-")
	num, _ := strings.replace_all(rest, "-", ".")
	return strings.concatenate({"Chapter ", num})
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

// ─── search ──────────────────────────────────────────────────────────────────

// Search returns slugs from the result anchors, then attaches the cover whose
// thumb filename equals that slug. Matching on the filename (rather than by
// position) is what keeps covers correct for the entries that have none.
search_impl :: proc (arg: string) -> string {
	f: Search_Filter
	if json.unmarshal_string(arg, &f) != nil {
		return "[]"
	}
	page := f.Page
	if page < 1 {
		page = 1
	}

	url := strings.concatenate({
		site(), "/advanced-search?name=", url_encode(strings.trim_space(f.Query)),
		"&page=", fmt.aprintf("%d", page),
	})
	body, ok := abi.http_body(url, headers())
	if !ok {
		return "[]"
	}

	raw_slugs := find_all(body, `<a href="https://comicasura\.net/manga/([a-z0-9-]+)">`)
	slugs := make([dynamic]string, 0, len(raw_slugs))
	defer delete(slugs)
	seen_slugs := make(map[string]bool, len(raw_slugs))
	defer delete(seen_slugs)
	for s in raw_slugs {
		if seen_slugs[s] {
			continue
		}
		seen_slugs[s] = true
		append(&slugs, s)
	}

	// alt + thumb come from the same <img> tag, so pairing them by row is safe.
	pairs := find_all_pairs(body, `<img alt="([^"]*)"[^>]*src="(https?://[^"]+/thumb/[^"]+)"`)
	covers := make(map[string]string, len(pairs))
	defer delete(covers)
	titles := make(map[string]string, len(pairs))
	for p in pairs {
		if len(p) < 2 {
			continue
		}
		key := basename_slug(p[1])
		covers[key] = p[1]
		titles[key] = strings.trim_space(p[0])
	}

	out := make([dynamic]Manga_Out, 0, len(slugs))
	defer delete(out)
	for s in slugs {
		title := titles[s]
		if title == "" {
			spaced, _ := strings.replace_all(s, "-", " ")
			title = abi.host_str("text.titlecase", spaced)
		}
		append(&out, Manga_Out{
			ID        = s,
			Title     = title,
			Cover_URL = covers[s],
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
	slug := strings.trim_space(decode_id(arg))
	if slug == "" {
		return "{}"
	}

	body, ok := abi.http_body(
		strings.concatenate({site(), "/manga/", slug}),
		headers(),
	)
	if !ok {
		return abi.to_json(Manga_Out{ID = slug})
	}

	title := find_one(body, `<h1[^>]*>([^<]+)</h1>`)
	cover := find_one(body, `<meta property="og:image" content="([^"]+)"`)
	status := find_one(body, `Status\s*</div>\s*<div[^>]*>\s*([A-Za-z ]{2,20})\s*</div>`)
	author := find_one(body, `>Author<[^>]*>\s*<div[^>]*>\s*([^<]{1,60})`)
	desc := find_one(body, `<p class="text-\[#A2A2A2\] mb-3">(.*?)</p>`)

	// The modal's Overview paragraph can contain inline markup; strip it.
	if desc != "" {
		desc = abi.host_str("text.strip_html", desc)
	}

	genres := make([dynamic]string, 0, 8)
	defer delete(genres)
	seen_genres := make(map[string]bool, 8)
	defer delete(seen_genres)
	for row in find_all_pairs(body, `/manga-list/([a-z0-9_-]+)"[^>]*>\s*([A-Za-z0-9 ]{2,20})\s*</a>`) {
		if len(row) < 2 {
			continue
		}
		g := strings.trim_space(row[1])
		if g == "" || seen_genres[g] {
			continue
		}
		seen_genres[g] = true
		append(&genres, g)
	}

	return abi.to_json(Manga_Out{
		ID        = slug,
		Title     = title,
		Cover_URL = cover,
		Author    = author,
		Desc      = desc,
		Status    = abi.host_str("text.normalize_status", status),
		Genres    = genres[:],
	})
}

// ─── chapters ────────────────────────────────────────────────────────────────

chapters_impl :: proc (arg: string) -> string {
	slug := strings.trim_space(decode_id(arg))
	if slug == "" {
		return "[]"
	}

	body, ok := abi.http_body(
		strings.concatenate({site(), "/manga/", slug}),
		headers(),
	)
	if !ok {
		return "[]"
	}

	pattern := strings.concatenate({`href="https://comicasura\.net/manga/`, slug, `/(chapter-[0-9a-z.\-]+)"`})
	ids := find_all(body, pattern)

	seen := make(map[string]bool, len(ids))
	defer delete(seen)
	out := make([dynamic]Chapter_Out, 0, len(ids))
	defer delete(out)
	for cid in ids {
		if seen[cid] {
			continue
		}
		seen[cid] = true
		label := chapter_label(cid)
		append(&out, Chapter_Out{
			ID          = strings.concatenate({slug, ":", cid}),
			Manga_ID    = slug,
			Title       = label,
			Chapter_Num = host_num("text.chapter_num", label),
			URL         = strings.concatenate({site(), "/manga/", slug, "/", cid}),
		})
	}

	// Newest-first: the ABI's "continue reading" resumes from chapter one when
	// a source hands chapters back in page order.
	slice.sort_by(out[:], proc(a, b: Chapter_Out) -> bool {
		return a.Chapter_Num > b.Chapter_Num
	})
	return abi.to_json(out[:])
}

// host_num reads a numeric answer from a host helper.
host_num :: proc (fn: string, arg: string) -> f64 { return abi.host_num(fn, arg) }

// ─── pages ───────────────────────────────────────────────────────────────────

pages_impl :: proc (arg: string) -> string {
	raw := strings.trim_space(decode_id(arg))
	i := strings.index_byte(raw, ':')
	if i <= 0 || i + 1 >= len(raw) {
		return "[]"
	}
	slug := raw[:i]
	chapter := raw[i + 1:]

	body, ok := abi.http_body(
		strings.concatenate({site(), "/manga/", slug, "/", chapter}),
		headers(),
	)
	if !ok {
		return "[]"
	}

	// Reader images are single-quoted and live on rotating hosts, so the
	// character class excludes both quote styles.
	urls := find_all(body, `https://imgs-2\.2xstorage\.com/[^"'\s>]+`)

	seen := make(map[string]bool, len(urls))
	defer delete(seen)
	out := make([dynamic]Page_Out, 0, len(urls))
	defer delete(out)
	for u in urls {
		if seen[u] {
			continue
		}
		seen[u] = true
		append(&out, Page_Out{Index = len(out), URL = u})
	}
	return abi.to_json(out[:])
}

// ─── genres ──────────────────────────────────────────────────────────────────

genres_impl :: proc () -> string {
	body, ok := abi.http_body(
		strings.concatenate({site(), "/genres"}),
		headers(),
	)
	if !ok {
		return "[]"
	}

	out := make([dynamic]Genre_Out, 0, 64)
	defer delete(out)
	for row in find_all_pairs(body, `/manga-list/([a-z0-9_-]+)"[^>]*>\s*([A-Za-z0-9 ]{2,20})\s*</a>`) {
		if len(row) < 2 {
			continue
		}
		name := strings.trim_space(row[1])
		slug := strings.trim_space(row[0])
		if name == "" || slug == "" {
			continue
		}
		dupe := false
		for g in out {
			if g.Slug == slug {
				dupe = true
				break
			}
		}
		if !dupe {
			append(&out, Genre_Out{Name = name, Slug = slug})
		}
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
