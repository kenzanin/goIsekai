// MangaUpdates enrichment script for goIsekai, Odin wasm port of
// examples/info/mangaupdates/main.lua.
//
// An ENRICHMENT script, not a manga source: it lives in the info directory, is
// hidden from the plugin list, and exports GetEnrichment instead of the scraper
// ABI. Fields served:
//   titles      associated[] titles
//   summaries   description with link blocks + markdown stripped
//   categories  genres[]
//   authors     authors[] names
//   related     category_recommendations[] (the site's "Recommendations")
//   covers      image.url.original
//
// MangaUpdates answers a search with the series id only, so one title costs two
// requests (search, then detail). The detail response carries every field, so
// it is memoized in the module and reused by the remaining kinds.
package mangaupdates_info

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:strings"

API :: "https://api.mangaupdates.com/v1"

// ─── API shapes (verified against the live API, not guessed) ────────────────

Genre :: struct {
	genre: string `json:"genre"`,
}

Associated :: struct {
	title: string `json:"title"`,
}

Author :: struct {
	name: string `json:"name"`,
	url:  string `json:"url"`,
}

Cat_Rec :: struct {
	series_name: string `json:"series_name"`,
	series_url:  string `json:"series_url"`,
}

Image_URL :: struct {
	original: string `json:"original"`,
	thumb:    string `json:"thumb"`,
}

Image :: struct {
	url: Image_URL `json:"url"`,
}

// Record is the search hit's inline record: it already carries genres,
// description and the cover, so it stands in when detail leaves them out.
Record :: struct {
	series_id:   f64          `json:"series_id"`,
	title:       string       `json:"title"`,
	url:         string       `json:"url"`,
	type:        string       `json:"type"`,
	description: string       `json:"description"`,
	genres:      []Genre      `json:"genres"`,
	associated:  []Associated `json:"associated"`,
	image:       Image        `json:"image"`,
}

Hit :: struct {
	record: Record `json:"record"`,
}

Search_Resp :: struct {
	results: []Hit `json:"results"`,
}

Detail :: struct {
	title:                    string       `json:"title"`,
	url:                      string       `json:"url"`,
	description:              string       `json:"description"`,
	genres:                   []Genre      `json:"genres"`,
	authors:                  []Author     `json:"authors"`,
	associated:               []Associated `json:"associated"`,
	category_recommendations: []Cat_Rec    `json:"category_recommendations"`,
	image:                    Image        `json:"image"`,
}

// Item is one enrichment entry the host stores.
Item :: struct {
	value: string `json:"value"`,
	url:   string `json:"url"`,
}

// Search_Req is POST /series/search's body. A struct rather than a map literal:
// Odin disables dynamic-type compound literals by default, and this also states
// the wire shape.
Search_Req :: struct {
	search: string `json:"search"`,
}

Enrich_Req :: struct {
	title: string `json:"title"`,
	kind:  string `json:"kind"`,
}

// ─── memo (single slot) ─────────────────────────────────────────────────────
// Enrichment runs kind by kind for one manga at a time, so the previous title's
// detail is never needed again. The module stays instantiated between calls.
memo_title: string
memo_detail: Detail
memo_ok: bool

// ─── helpers ────────────────────────────────────────────────────────────────

// decode_generic parses a JSON answer into v, false on empty/failed input.
decode_into :: proc(body: string, v: ^$T) -> bool {
	if body == "" || body == "null" {
		return false
	}
	return json.unmarshal(transmute([]u8)body, v) == nil
}

normalize_title :: proc(s: string) -> string {
	return abi.host_str("text.normalize_title", s)
}

// prefix_match is the tolerant comparison the Lua uses for romaji variants
// ("Ke wo" vs "Ie wo"): both sides need 4+ chars and one must prefix the other.
prefix_match :: proc(want, have: string) -> bool {
	if len(want) < 4 || len(have) < 4 {
		return false
	}
	if len(want) <= len(have) {
		return have[:len(want)] == want
	}
	return want[:len(have)] == have
}

// variant_match reports whether at least 70% of the searched title's tokens
// appear in candidate, so an unrelated series sharing a couple of words fails.
variant_match :: proc(want_tokens: []string, candidate: string) -> bool {
	if len(want_tokens) < 3 {
		return false
	}
	cand_tokens, _ := strings.fields(normalize_title(candidate))
	if len(cand_tokens) == 0 {
		return false
	}
	used := make([]bool, len(cand_tokens))
	defer delete(used)
	matched := 0
	for want in want_tokens {
		for have, i in cand_tokens {
			if used[i] {
				continue
			}
			if want == have || prefix_match(want, have) {
				used[i] = true
				matched += 1
				break
			}
		}
	}
	return f64(matched) >= f64(len(want_tokens)) * 0.7
}

// detail searches the best match and loads its full record into the memo.
// Only the first non-novel hit is used: the rest are unrelated series.
detail :: proc(title: string) -> (Detail, bool) {
	if memo_ok && memo_title == title {
		return memo_detail, true
	}
	memo_ok = false

	search_body, ok := abi.http_post_body(
		strings.concatenate({API, "/series/search"}, context.temp_allocator),
		abi.to_json(Search_Req{search = title}),
		`{"Content-Type":"application/json","Accept":"application/json"}`,
	)
	if !ok {
		return Detail{}, false
	}
	search: Search_Resp
	if !decode_into(search_body, &search) || len(search.results) == 0 {
		abi.host_args("log.debug", {strings.concatenate({"mangaupdates info: no match for ", title}, context.temp_allocator)})
		return Detail{}, false
	}

	record: Record
	found := false
	for hit in search.results {
		// A light novel tops the results for the manga adapted from it, and
		// its "(Novel)" titles differ from every other source's.
		if hit.record.type != "Novel" {
			record = hit.record
			found = true
			break
		}
	}
	if !found {
		return Detail{}, false
	}

	detail_body, got := abi.http_body(
		fmt.aprintf("%s/series/%.0f", API, record.series_id),
		`{"Accept":"application/json"}`,
	)
	if !got {
		detail_body = ""
	}
	data: Detail
	if !decode_into(detail_body, &data) {
		data = Detail{}
	}

	// Verify the hit is the searched series: the title must match a record or
	// associated title exactly (after normalization), or clear the tolerant
	// token check. Sequels and spin-offs fail it.
	wanted := normalize_title(title)
	want_tokens, _ := strings.fields(wanted)
	matched := normalize_title(record.title) == wanted || variant_match(want_tokens, record.title)
	if !matched {
		for assoc in data.associated {
			if normalize_title(assoc.title) == wanted || variant_match(want_tokens, assoc.title) {
				matched = true
				break
			}
		}
	}
	if !matched {
		abi.host_args("log.debug", {strings.concatenate({"mangaupdates info: title mismatch for ", title}, context.temp_allocator)})
		return Detail{}, false
	}

	// Fill from the search record when detail leaves a field out.
	if data.url == "" {
		data.url = record.url
	}
	if len(data.genres) == 0 {
		data.genres = record.genres
	}
	if len(data.associated) == 0 {
		data.associated = record.associated
	}
	if data.image.url.original == "" {
		data.image = record.image
	}
	if data.description == "" {
		data.description = record.description
	}

	memo_title, memo_detail, memo_ok = title, data, true
	return memo_detail, true
}

// collect dedupes (value, url) pairs, keeping the first occurrence.
collect :: proc(pairs: []Item) -> string {
	out := make([dynamic]Item, 0, len(pairs))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)
	for p in pairs {
		if p.value == "" || seen[p.value] {
			continue
		}
		seen[p.value] = true
		append(&out, p)
	}
	return abi.to_json(out[:])
}

// ─── per-kind builders ──────────────────────────────────────────────────────

by_kind :: proc(data: Detail, kind: string) -> (string, bool) {
	switch kind {
	case "titles":
		pairs := make([dynamic]Item, 0, len(data.associated))
		defer delete(pairs)
		for a in data.associated {
			append(&pairs, Item{value = a.title, url = data.url})
		}
		return collect(pairs[:]), true

	case "summaries":
		if data.description == "" {
			return "[]", true
		}
		// Link blocks first, then markdown: the order matters for the same
		// reason it does in the Lua — markdown stripping can leave a bare URL.
		clean := abi.host_str("text.strip_markdown", abi.host_str("text.strip_link_blocks", data.description))
		return abi.to_json([]Item{{value = clean, url = data.url}}), true

	case "categories":
		pairs := make([dynamic]Item, 0, len(data.genres))
		defer delete(pairs)
		for g in data.genres {
			append(&pairs, Item{value = g.genre, url = data.url})
		}
		return collect(pairs[:]), true

	case "authors":
		pairs := make([dynamic]Item, 0, len(data.authors))
		defer delete(pairs)
		for a in data.authors {
			url := a.url
			if url == "" {
				url = data.url
			}
			append(&pairs, Item{value = a.name, url = url})
		}
		return collect(pairs[:]), true

	case "related":
		// category_recommendations[] is what the site renders under
		// "Recommendations"; related_series[] carries only direct plot
		// relations, which are empty for most series.
		pairs := make([dynamic]Item, 0, len(data.category_recommendations))
		defer delete(pairs)
		for r in data.category_recommendations {
			append(&pairs, Item{value = r.series_name, url = r.series_url})
		}
		return collect(pairs[:]), true

	case "covers":
		url := data.image.url.original
		if url == "" {
			return "[]", true
		}
		return abi.to_json([]Item{{value = url, url = data.url}}), true
	}
	return "", false
}

// getEnrichment — arg is {"title":...,"kind":...,"source":...}. An unknown kind
// or a failed lookup yields an empty array, which the host records as "nothing
// found".
get_enrichment_impl :: proc(arg: string) -> string {
	req: Enrich_Req
	if !decode_into(arg, &req) || req.title == "" || req.kind == "" {
		return "[]"
	}
	data, ok := detail(req.title)
	if !ok {
		return "[]"
	}
	out, known := by_kind(data, req.kind)
	if !known {
		return "[]"
	}
	return out
}

@(export)
GetEnrichment :: proc "c" (ptr: u32, length: u32) -> u64 {
	context = runtime.default_context()
	return abi.reply_json(get_enrichment_impl(abi.read_string(ptr, length)))
}
