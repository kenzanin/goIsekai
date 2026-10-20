// MangaDex enrichment script for goIsekai, Odin wasm port of
// examples/info/mangadex/main.lua.
//
// An ENRICHMENT script, not a manga source: it lives in the info directory, is
// hidden from the plugin list, and exports GetEnrichment instead of the scraper
// ABI. Fields served:
//   titles      alternative titles
//   summaries   synopsis
//   categories  genre tags
//   authors     author credits
//   related     recommended manga (the site's Recommendations tab)
//   covers      every uploaded cover URL
//
// Reference expansion keeps this to three upstream requests per title:
//   search (+includes[])          the winning record carries every field
//   /recommendation (+includes[]) related titles arrive inline
//   /cover?manga[]=               the full cover catalogue (no inline form)
// The search record is byte-identical to /manga/{id} under the same includes,
// so there is no separate detail fetch. The result is memoized in the module
// and reused by all six kinds: MangaDex rate-limits aggressively.
package mangadex_info

import "abi"
import "base:runtime"
import "core:encoding/json"
import "core:fmt"
import "core:strings"

API :: "https://api.mangadex.org"
SITE :: "https://mangadex.org"

// ─── API shapes (verified against the live API) ─────────────────────────────

Lang_Map :: map[string]string

Tag_Attrs :: struct {
	name:  Lang_Map `json:"name"`,
	group: string   `json:"group"`,
}

Tag :: struct {
	attributes: Tag_Attrs `json:"attributes"`,
}

// Rel_Attrs is shared by author credits (name), cover_art (fileName/locale)
// and inline manga relations (title). Absent fields stay zero.
Rel_Attrs :: struct {
	name:     string   `json:"name"`,
	fileName: string   `json:"fileName"`,
	locale:   string   `json:"locale"`,
	title:    Lang_Map `json:"title"`,
}

Rel :: struct {
	id:         string    `json:"id"`,
	type:       string    `json:"type"`,
	attributes: Rel_Attrs `json:"attributes"`,
}

Manga_Attrs :: struct {
	title:       Lang_Map   `json:"title"`,
	altTitles:   []Lang_Map `json:"altTitles"`,
	description: Lang_Map   `json:"description"`,
	tags:        []Tag      `json:"tags"`,
}

Manga :: struct {
	id:            string      `json:"id"`,
	attributes:    Manga_Attrs `json:"attributes"`,
	relationships: []Rel       `json:"relationships"`,
}

Manga_Resp :: struct {
	data: []Manga `json:"data"`,
}

Rec_Entry :: struct {
	relationships: []Rel `json:"relationships"`,
}

Rec_Resp :: struct {
	data: []Rec_Entry `json:"data"`,
}

Cover_Attrs :: struct {
	fileName: string `json:"fileName"`,
	locale:   string `json:"locale"`,
}

Cover_Entry :: struct {
	attributes: Cover_Attrs `json:"attributes"`,
}

Cover_Resp :: struct {
	data: []Cover_Entry `json:"data"`,
}

Item :: struct {
	value: string `json:"value"`,
	url:   string `json:"url"`,
}

Enrich_Req :: struct {
	title: string `json:"title"`,
	kind:  string `json:"kind"`,
}

// Rank orders candidates; lower is better. tier 0/1 = contained match (tier 0
// when the candidate ends with the searched string), tier 2 = fuzzy.
Rank :: struct {
	tier:  int,
	score: int,
}

// ─── memo (single slot) ─────────────────────────────────────────────────────
// Enrichment runs kind by kind for one manga at a time, so the previous title's
// record is never needed again.
memo_title: string
memo_manga: Manga
memo_ok: bool

// preferred lists the languages a displayed string may come from, best first.
preferred :: []string{"en", "ja-ro", "ko-ro", "ja", "ko"}

// ─── helpers ────────────────────────────────────────────────────────────────

// pick returns the best available string from a {lang: text} map.
pick :: proc(m: Lang_Map) -> string {
	for lang in preferred {
		if v, ok := m[lang]; ok && v != "" {
			return v
		}
	}
	return ""
}

// norm_title normalizes a title for comparison. Apostrophes are stripped first
// so romanizations like "Ten'i" match the flattened "Teni" other sources use,
// then the host normalizer does the rest.
norm_title :: proc(s: string) -> string {
	no_apos, _ := strings.replace_all(s, "'", "")
	// U+2019 RIGHT SINGLE QUOTATION MARK, spelled in bytes to match the Lua.
	no_curly, _ := strings.replace_all(no_apos, "\xe2\x80\x99", "")
	return abi.host_str("text.normalize_title", no_curly)
}

tokens :: proc(normalized: string) -> []string {
	t, _ := strings.fields(normalized)
	return t
}

// fuzzy_rank scores a romanization-variant match ("Haburaretara" vs
// "Haburareta node", "Doukyuu" vs "Doukyuusei"): every searched token must find
// an exact or prefix-tolerant partner in the candidate (prefix matching only
// for tokens of 4+ chars so short words stay strict). Fails below 70% coverage
// or on titles shorter than 3 tokens.
fuzzy_rank :: proc(search: []string, cand: []string) -> (Rank, bool) {
	if len(search) < 3 {
		return {}, false
	}
	used := make([]bool, len(cand))
	defer delete(used)
	matched := 0
	for want in search {
		for have, i in cand {
			if used[i] {
				continue
			}
			tolerant := len(want) >= 4 && len(have) >= 4 &&
				(strings.has_prefix(want, have) || strings.has_prefix(have, want))
			if want == have || tolerant {
				used[i] = true
				matched += 1
				break
			}
		}
	}
	if f64(matched) < f64(len(search)) * 0.7 {
		return {}, false
	}
	return Rank{tier = 2, score = (len(search) - matched) * 1000 + (len(cand) - matched)}, true
}

// title_match_score ranks candidates so the right series wins: an exact
// normalized match wins outright (exact = true), a contained match follows
// (bracketed suffixes penalized), romanization variants take the fuzzy tier
// last. ok = false means no match at all.
title_match_score :: proc(m: Manga, searched: string) -> (rank: Rank, exact: bool, ok: bool) {
	normalized_search := norm_title(searched)
	search_tokens := tokens(normalized_search)

	candidates := make([dynamic]string, 0, len(m.attributes.altTitles) + 1)
	defer delete(candidates)
	if main_title := pick(m.attributes.title); main_title != "" {
		append(&candidates, main_title)
	}
	for entry in m.attributes.altTitles {
		if alt := pick(entry); alt != "" {
			append(&candidates, alt)
		}
	}

	for candidate in candidates {
		normalized := norm_title(candidate)
		if normalized == normalized_search {
			return {}, true, true
		}
		cand_rank: Rank
		found: bool
		if len(normalized) > len(normalized_search) && strings.contains(normalized, normalized_search) {
			// A candidate ending with the searched string is a publisher
			// prefix ("Trapped in a Dating Sim: X") on the original; anything
			// else carries a sequel or spin-off suffix after the match.
			ends_with := strings.has_suffix(normalized, normalized_search)
			score := len(normalized) - len(normalized_search)
			if strings.has_suffix(normalized, ")") && strings.contains(normalized, "(") {
				score += 50
			}
			cand_rank = Rank{tier = ends_with ? 0 : 1, score = score}
			found = true
		} else {
			cand_tokens := tokens(normalized)
			cand_rank, found = fuzzy_rank(search_tokens, cand_tokens)
		}
		if found && (!ok || cand_rank.tier < rank.tier || (cand_rank.tier == rank.tier && cand_rank.score < rank.score)) {
			rank, ok = cand_rank, true
		}
	}
	return rank, false, ok
}

// trim_trailing drops the last n whitespace-separated words, keeping at least
// one. Mirrors the Lua's repeated "^.-%s+%S+%s*$" match, which stops changing
// the string once a single word remains.
trim_trailing :: proc(s: string, n: int) -> string {
	if n <= 0 {
		return s
	}
	words, _ := strings.fields(s)
	if len(words) == 0 {
		return s
	}
	keep := len(words) - n
	if keep < 1 {
		keep = 1
	}
	out, _ := strings.join(words[:keep], " ")
	return out
}

// search_manga queries /manga and returns the decoded response.
//
// MangaDex's search engine returns zero hits for long romaji titles whose word
// choice differs from the listing ("Ke wo" vs "Ie wo", "no de" vs "node"). Trim
// trailing words until something comes back; the scorer still ranks candidates
// against the FULL title, so a wrong series cannot win.
// ponytail: seven attempts cover the observed particle-tail cases; a real
// search-relevance fallback would need a second API.
search_manga :: proc(title: string) -> (Manga_Resp, bool) {
	for attempt in 0 ..= 6 {
		query := trim_trailing(title, attempt * 2)
		if query == "" {
			break
		}
		url := fmt.aprintf(
			"%s/manga?limit=10&order[relevance]=desc&title=%s&includes[]=cover_art&includes[]=author&includes[]=artist&includes[]=manga",
			API,
			abi.url_encode(query),
		)
		body, got := abi.http_body(url, `{"Accept":"application/json"}`)
		if !got {
			continue
		}
		resp: Manga_Resp
		if json.unmarshal(transmute([]u8)body, &resp) != nil {
			continue
		}
		if len(resp.data) > 0 {
			return resp, true
		}
	}
	return {}, false
}

// detail resolves a title to its full MangaDex record, memoized.
detail :: proc(title: string) -> (Manga, bool) {
	if memo_ok && memo_title == title {
		return memo_manga, true
	}
	memo_ok = false

	search, found := search_manga(title)
	if !found {
		return {}, false
	}

	// The most relevant hit is often a sequel or spin-off whose alt title
	// merely contains the searched string. Score every candidate so an exact
	// match anywhere wins, and the least-divergent contained match follows.
	record: Manga
	best: Rank
	have, have_best := false, false
	for candidate in search.data {
		rank, exact, ok := title_match_score(candidate, title)
		if exact {
			record, have = candidate, true
			break
		}
		if ok && (!have_best || rank.tier < best.tier || (rank.tier == best.tier && rank.score < best.score)) {
			record, best, have_best, have = candidate, rank, true, true
		}
	}
	if !have {
		return {}, false
	}

	memo_title, memo_manga, memo_ok = title, record, true
	return memo_manga, true
}

// collect dedupes items by value, keeping the first occurrence.
collect_values :: proc(items: []Item) -> string {
	out := make([dynamic]Item, 0, len(items))
	defer delete(out)
	seen := make(map[string]bool)
	defer delete(seen)
	for it in items {
		if it.value == "" || seen[it.value] {
			continue
		}
		seen[it.value] = true
		append(&out, it)
	}
	return abi.to_json(out[:])
}

// ─── per-kind builders ──────────────────────────────────────────────────────

by_kind :: proc(m: Manga, kind: string) -> (string, bool) {
	page_url := fmt.aprintf("%s/title/%s", SITE, m.id)

	switch kind {
	case "titles":
		// altTitles first, then the main title — the Lua's order, so the UI
		// keeps showing translated names above the romanized one.
		out := make([dynamic]Item, 0, len(m.attributes.altTitles) + 1)
		defer delete(out)
		for entry in m.attributes.altTitles {
			append(&out, Item{value = pick(entry), url = page_url})
		}
		append(&out, Item{value = pick(m.attributes.title), url = page_url})
		return collect_values(out[:]), true

	case "summaries":
		desc := pick(m.attributes.description)
		if desc == "" {
			return "[]", true
		}
		// markdown first, then link blocks — this order is the Lua's.
		clean := abi.host_str("text.strip_link_blocks", abi.host_str("text.strip_markdown", desc))
		return abi.to_json([]Item{{value = clean, url = page_url}}), true

	case "categories":
		out := make([dynamic]Item, 0, len(m.attributes.tags))
		defer delete(out)
		for tag in m.attributes.tags {
			if tag.attributes.group != "genre" {
				continue
			}
			append(&out, Item{value = pick(tag.attributes.name), url = page_url})
		}
		return collect_values(out[:]), true

	case "authors":
		out := make([dynamic]Item, 0, len(m.relationships))
		defer delete(out)
		for rel in m.relationships {
			if rel.type == "author" {
				append(&out, Item{value = rel.attributes.name, url = page_url})
			}
		}
		return collect_values(out[:]), true

	case "related":
		// Recommendations are the main list, matching what the MangaDex page
		// shows; inline plot relations are the fallback, since only a minority
		// of series have any. The API's own ranking order is preserved.
		out := make([dynamic]Item, 0, 16)
		defer delete(out)
		seen := make(map[string]bool)
		defer delete(seen)

		body, got := abi.http_body(
			fmt.aprintf("%s/manga/%s/recommendation?includes[]=manga", API, m.id),
			`{"Accept":"application/json"}`,
		)
		if got {
			rec: Rec_Resp
			if json.unmarshal(transmute([]u8)body, &rec) == nil {
				for entry in rec.data {
					for rel in entry.relationships {
						add_related(&out, &seen, m.id, rel)
					}
				}
			}
		}
		if len(out) == 0 {
			for rel in m.relationships {
				if rel.type == "manga" {
					add_related(&out, &seen, m.id, rel)
				}
			}
		}
		return abi.to_json(out[:]), true

	case "covers":
		// /cover lists every uploaded cover per volume/edition (the manga
		// payload carries only the primary cover_art). English first, Japanese
		// as fallback, everything else after.
		body, got := abi.http_body(
			fmt.aprintf("%s/cover?manga%%5B%%5D=%s&limit=50", API, m.id),
			`{"Accept":"application/json"}`,
		)
		if !got {
			return "[]", true
		}
		page: Cover_Resp
		if json.unmarshal(transmute([]u8)body, &page) != nil {
			return "[]", true
		}

		en := make([dynamic]Item, 0, len(page.data))
		defer delete(en)
		ja := make([dynamic]Item, 0, 4)
		defer delete(ja)
		rest := make([dynamic]Item, 0, len(page.data))
		defer delete(rest)

		for entry in page.data {
			file := entry.attributes.fileName
			if file == "" {
				continue
			}
			it := Item{
				value = fmt.aprintf("https://uploads.mangadex.org/covers/%s/%s", m.id, file),
				url   = page_url,
			}
			switch entry.attributes.locale {
			case "en":
				append(&en, it)
			case "ja":
				append(&ja, it)
			case:
				append(&rest, it)
			}
		}

		out := make([dynamic]Item, 0, len(page.data))
		defer delete(out)
		seen := make(map[string]bool)
		defer delete(seen)
		for group in ([][]Item{en[:], ja[:], rest[:]}) {
			for it in group {
				if seen[it.value] {
					continue
				}
				seen[it.value] = true
				append(&out, it)
			}
		}
		return abi.to_json(out[:]), true
	}
	return "", false
}

// add_related appends one inline manga relation, skipping the record itself and
// repeats. seen is a pointer because Odin map parameters are not addressable.
add_related :: proc(out: ^[dynamic]Item, seen: ^map[string]bool, self: string, rel: Rel) {
	if rel.type != "manga" || rel.id == "" || rel.id == self {
		return
	}
	if _, dup := seen^[rel.id]; dup {
		return
	}
	title := pick(rel.attributes.title)
	if title == "" {
		return
	}
	seen^[rel.id] = true
	append(out, Item{value = title, url = fmt.aprintf("%s/title/%s", SITE, rel.id)})
}

// get_enrichment_impl — arg is {"title":...,"kind":...,"source":...}. An
// unknown kind or a failed lookup yields an empty array.
get_enrichment_impl :: proc(arg: string) -> string {
	req: Enrich_Req
	if json.unmarshal(transmute([]u8)arg, &req) != nil || req.title == "" || req.kind == "" {
		return "[]"
	}
	record, ok := detail(req.title)
	if !ok {
		abi.host_args("log.debug", {fmt.aprintf("mangadex info: no match for %s (%s)", req.title, req.kind)})
		return "[]"
	}
	out, known := by_kind(record, req.kind)
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
