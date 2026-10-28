//go:build wasip1

// Mangapill source plugin for goIsekai — TinyGo WASM reactor.
//
// Mangapill (mangapill.com) is a PHP frontend over HTML: search and manga
// detail are server-rendered HTML, chapter lists come from a partial HTML
// fragment served at /manga/<id>/<slug>/chapters, and the reader page at
// /chapters/<id>-<cid>/<slug> holds <img class="js-page" data-src="...">.
// There is no JSON API, so every function parses HTML through host.http.get.
//
// ID scheme:
//   Manga.ID     = numeric site id as a string, e.g. "2574"
//   Chapter.ID   = "<mangaID>:<chapterNum>"  e.g. "2574:1"
//   PageList arg = the chapter ID ("2574:1")
package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"goisekai/pkg/types"
)

// ─── config ──────────────────────────────────────────────────────────────────

const (
	siteBase = "https://mangapill.com"
	siteURL  = "https://mangapill.com/manga"
	ua       = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36"
)

// logInfo/logWarn/logError come from hostcall.go.
var _ = logInfo

// main is required by the Go linker; -buildmode=c-shared makes the module a
// reactor (initialised through _initialize, main never runs).
func main() {}

// ─── scraping helpers ────────────────────────────────────────────────────────

// gAll returns every full match of a global regex pattern in s. Patterns with a
// single capture group yield a string per match; patterns with several groups
// yield a []string per match. Mirrors mangabuddy's gAll helper.
func gAll(s, pattern string) [][]string {
	res := hostCall("regex.find_all", s, pattern)
	if res == "null" || res == "" {
		return nil
	}
	var rows []json.RawMessage
	if json.Unmarshal([]byte(res), &rows) != nil {
		return nil
	}
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		var vals []string
		if json.Unmarshal(row, &vals) == nil && len(vals) > 0 && vals[0] != "" {
			out = append(out, vals)
			continue
		}
		var single string
		if json.Unmarshal(row, &single) == nil && single != "" {
			out = append(out, []string{single})
		}
	}
	return out
}

// firstCap extracts the first capture group of pattern in s, or "".
func firstCap(s, pattern string) string {
	res := hostCall("regex.find", s, pattern)
	if res == "null" || res == "" {
		return ""
	}
	var vals []string
	if json.Unmarshal([]byte(res), &vals) == nil && len(vals) >= 1 {
		return vals[0]
	}
	var str string
	if json.Unmarshal([]byte(res), &str) == nil {
		return str
	}
	return ""
}

// MANGAPILL NOTE (goisekai): the host's regex engine is coregex, an O(n)
// non-backtracking engine — it does NOT support lazy quantifiers (.*?) the way
// mangabuddy's Lua fallback does. Every field is therefore captured with its
// own single-group pattern and gAll/firstCap runs separately, so a single match
// never needs to span tags.

// ── per-card patterns (must stay in sync) ─────────────────────────────────
// Each result card on the search page is a <div> containing, in order:
//
//	<a href="/manga/<id>/<slug>" class="relative block"> … cover img … </a>
//	<div class="flex flex-col justify-end">
//	  <a href="/manga/<id>/<slug>" class="mb-2">
//	    <div class="mt-3 font-black leading-tight line-clamp-2">TITLE</div>
//	  </a>
//	</div>
//
// Two hrefs per card (the cover link and the title link) plus the standalone
// cover <img> and the title <div> all appear in document order, one set per
// card. We collect four parallel single-group scans and zip by position;
// because the per-card elements are emitted in a fixed order and each card
// contributes exactly one of each, the slices stay aligned.
//
// mangaIDRe matches the cover link's <a> anchor (the first <a> in each card,
// marked by class="relative block"). One id per card, in document order.
// The earlier bare `href="/manga/(\d+)/"` matched both per-card links
// (cover + title), doubling the id slice and shifting the zip out of
// alignment with the 1-per-card cover/title slices.
var mangaIDRe = `<a href="/manga/(\d+)/[^"]+" class="relative block"`

// mangaCoverRe matches the cover <img> data-src (lazy-loaded) inside a card.
// Use `(?:data-src|src)` to also catch already-loaded images that carry the
// plain src attribute. Pin the CDN prefix so the match is robust to other
// img tags on the page (JS bundle, favicon, …).
var mangaCoverRe = `(?:data-src|src)="(https://cdn\.readdetectiveconan\.com/file/mangapill/i/[^"]+\.(?:jpg|jpeg|png|webp))"`

// mangaTitleRe matches the title <div> (font-black variant only, not the
// text-xs synopsis). group 1 is the title text. (Mangapill repeats the title
// in the img alt too, so the <div> text is preferred.)
var mangaTitleRe = `font-black leading-tight line-clamp-2">([^<]+)<`

var reMangaSlug = regexp.MustCompile(`/manga/(\d+)/([^"]+)`)
var reChapterHref = regexp.MustCompile(`/chapters/(\d+)-(\d+)/`)

// normalizeStatus maps mangapill's status strings to the host enum.
func normalizeStatus(s string) string {
	l := strings.ToLower(s)
	switch {
	case strings.Contains(l, "ongoing"):
		return "Ongoing"
	case strings.Contains(l, "completed"):
		return "Completed"
	case strings.Contains(l, "hiatus"):
		return "Hiatus"
	case strings.Contains(l, "cancelled"), strings.Contains(l, "dropped"):
		return "Cancelled"
	}
	return ""
}

// parseQueryPage builds a /search URL honouring the SearchFilter. Mangapill
// supports ?q= , &type= , &status= , repeated &genre=<slug>, and &page=.
func parseQuery(f types.SearchFilter) string {
	var b strings.Builder
	b.WriteString(siteBase + "/search?")
	if f.Page > 1 {
		b.WriteString("page=" + itoa(f.Page) + "&")
	}
	if f.Query != "" {
		b.WriteString("q=" + urlEncode(f.Query) + "&")
	}
	for _, g := range f.Genres {
		b.WriteString("genre=" + urlEncode(g) + "&")
	}
	// trim trailing &
	u := b.String()
	return strings.TrimSuffix(u, "&")
}

// urlEncode delegates to the host to avoid pulling net/url into TinyGo.
func urlEncode(s string) string { return hostString("text.url_encode", s) }

// ─── Search ──────────────────────────────────────────────────────────────────

func search(arg string) []types.Manga {
	var f types.SearchFilter
	if json.Unmarshal([]byte(arg), &f) != nil {
		return nil
	}
	if f.Page < 1 {
		f.Page = 1
	}
	body := htmlGet(parseQuery(f))
	if body == "" {
		return nil
	}
	out := []types.Manga{}
	seen := map[string]bool{}
	// Pair each card by manga id, not by list position. The cover <img>
	// document order does not always match the card-link order (a grid can
	// reorder images), so a positional zip pairs a cover from card N+k onto
	// card N. Instead build a cover-id → url map and look up each card's
	// cover by its own id; titles stay positional because each card block
	// contains exactly one title div, emitted in card order.
	covers := gAll(body, mangaCoverRe)
	coverByID := map[string]string{}
	for _, c := range covers {
		u := c[0]
		if id := coverID(u); id != "" {
			coverByID[id] = u
		}
	}
	titles := gAll(body, mangaTitleRe)
	for i, idRow := range gAll(body, mangaIDRe) {
		id := idRow[0]
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		m := types.Manga{ID: id, CoverURL: coverByID[id]}
		if i < len(titles) && len(titles[i]) > 0 {
			m.Title = unescape(titles[i][0])
		}
		out = append(out, m)
	}
	return out
}

// ─── GetMangaDetail ─────────────────────────────────────────────────────────

func detail(arg string) types.Manga {
	id := decodeString(arg)
	if id == "" || !isDigits(id) {
		return types.Manga{ID: id}
	}
	body := htmlGet(siteBase + "/manga/" + id)
	if body == "" {
		return types.Manga{ID: id}
	}
	m := types.Manga{ID: id}
	m.Title = firstCap(body, `<h1 class="font-bold[^>]*>([^<]+)<`)
	if m.Title == "" {
		m.Title = firstCap(body, `<title>([^<]+) Manga - Mangapill<`)
	}
	m.Title = trim(unescape(m.Title))
	m.Description = trim(firstCap(body, `<p class="text-sm text--secondary">([^<]+)</p`))
	m.Status = normalizeStatus(firstCap(body, `Status</label>\s*<div>([^<]+)<`))
	m.CoverURL = firstCap(body, `og:image" content="([^"]+)"`)
	if m.CoverURL == "" {
		m.CoverURL = firstCap(body, `data-src="([^"]+)"`)
	}
	// Genres: anchor tags under the Genres label, e.g. /search?genre=Action
	for _, g := range gAll(body, `href="/search\?genre=([^"&]+)"[^>]*>([^<]+)<`) {
		if len(g) >= 2 && g[1] != "" {
			m.Genres = append(m.Genres, unescape(g[1]))
		}
	}
	return m
}

// ─── GetChapterList ─────────────────────────────────────────────────────────

// chaptersImpl reads the manga detail page and extracts the chapter links.
// Mangapill embeds the full chapter list (newest-first) in the detail page's
// "Chapters" section. Each entry looks like:
//   <a href="/chapters/2574-10001000/lazy-dungeon-master-chapter-1" ... title=" Chapter 1">Chapter 1</a>
// The numeric suffix on the chapter id (here 10001000) is a stable per-title
// counter; the displayed number ("Chapter 1") is what we surface.
func chaptersImpl(arg string) []types.Chapter {
	id := decodeString(arg)
	if id == "" || !isDigits(id) {
		return nil
	}
	body := htmlGet(siteBase + "/manga/" + id)
	if body == "" {
		return nil
	}
	slug := firstCap(body, `<a href="/manga/(\d+)/([^"]+)"`)
	if slug == "" {
		// fall back to reading the slug from the canonical link
		slug = firstCap(body, `/manga/\d+/([^"]+)`)
	}
	out := []types.Chapter{}
	seen := map[string]bool{}
	for _, m := range gAll(body, `/chapters/(\d+)-(\d+)/([^"]+)"[^>]*title="[^"]*">([^<]*)<`) {
		if len(m) < 4 {
			continue
		}
		mangaID, chapterID, label := m[0], m[1], m[3]
		if mangaID != id {
			continue
		}
		num := chapterNum(strings.TrimSpace(label))
		if num == 0 {
			continue
		}
		cid := mangaID + ":" + floatString(num)
		if seen[cid] {
			continue
		}
		seen[cid] = true
		out = append(out, types.Chapter{
			ID:         cid,
			MangaID:    mangaID,
			Title:      unescape(strings.TrimSpace(label)),
			ChapterNum: num,
			URL:        siteBase + "/chapters/" + mangaID + "-" + chapterID + "/" + slug,
		})
	}
	// detail page lists newest-first; host contract wants newest-first too.
	return out
}

// ─── GetPageList ────────────────────────────────────────────────────────────

// pagesImpl fetches a reader page and returns the js-page <img> data-src URLs.
// Chapter.ID is "<mangaID>:<chapterNum>"; the reader URL is
// /chapters/<mangaID>-<chapterNumZeroPadded>/<slug>-chapter-<num> but mangapill
// resolves any chapter id slug, so we reconstruct it from the chapters list.
func pagesImpl(arg string) []types.Page {
	composite := decodeString(arg)
	parts := strings.SplitN(composite, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		logError("mangapill: invalid chapter id " + composite)
		return nil
	}
	mangaID, chNum := parts[0], parts[1]
	if !isDigits(mangaID) {
		return nil
	}
	// Resolve the chapter slug/url via the detail page's chapter list.
	body := htmlGet(siteBase + "/manga/" + mangaID)
	if body == "" {
		return nil
	}
	// find the chapter link whose displayed number matches chNum.
	url := ""
	for _, m := range gAll(body, `/chapters/(\d+)-(\d+)/([^"]+)"[^>]*title="[^"]*">([^<]*)<`) {
		if len(m) < 4 || m[0] != mangaID {
			continue
		}
		if chapterNum(strings.TrimSpace(m[3])) == chapterNum(chNum) {
			url = siteBase + "/chapters/" + mangaID + "-" + m[1] + "/" + m[2]
			break
		}
	}
	if url == "" {
		// best-effort: guess the slug from the manga title link
		slug := firstCap(body, `<h1 class="font-bold[^>]*>([^<]+)<`)
		url = siteBase + "/chapters/" + mangaID + "-0/" + slugify(slug) + "-chapter-" + chNum
	}
	reader := htmlGet(url)
	if reader == "" {
		return nil
	}
	out := []types.Page{}
	seen := map[string]bool{}
	for _, m := range gAll(reader, `class="js-page"[^>]*data-src="([^"]+)"`) {
		if len(m) < 1 || m[0] == "" || seen[m[0]] {
			continue
		}
		seen[m[0]] = true
		out = append(out, types.Page{Index: len(out), URL: m[0]})
	}
	return out
}

// ─── GetGenres ─�─────────────────────────────────────────────────────────────

// Mangapill's search supports a genre facet; hardcode the site's known genres
// so the filter dropdown works. These mirror the visible genre anchors on
// /search pages.
var genreTable = []struct{ name, slug string }{
	{"Action", "Action"}, {"Adventure", "Adventure"}, {"Comedy", "Comedy"},
	{"Doujin", "Doujin"}, {"Drama", "Drama"}, {"Ecchi", "Ecchi"},
	{"Fantasy", "Fantasy"}, {"Game", "Game"}, {"Harem", "Harem"},
	{"Historical", "Historical"}, {"Horror", "Horror"}, {"Isekai", "Isekai"},
	{"Josei", "Josei"}, {"Mature", "Mature"}, {"Mecha", "Mecha"},
	{"Music", "Music"}, {"Mystery", "Mystery"}, {"Psychological", "Psychological"},
	{"Romance", "Romance"}, {"School", "School"}, {"Sci-Fi", "Sci-Fi"},
	{"Seinen", "Seinen"}, {"Shoujo", "Shoujo"}, {"Shounen", "Shounen"},
	{"Slice of Life", "Slice of Life"}, {"Sports", "Sports"},
	{"Supernatural", "Supernatural"}, {"Tragedy", "Tragedy"},
}

func genresImpl() []map[string]string {
	out := make([]map[string]string, 0, len(genreTable))
	for _, g := range genreTable {
		out = append(out, map[string]string{"name": g.name, "slug": g.slug})
	}
	return out
}

// ─── tiny helpers ───────────────────────────────────────────────────────────

var reDigits = regexp.MustCompile(`^\d+$`)

func isDigits(s string) bool { return reDigits.MatchString(s) }

func slugify(s string) string {
	s = strings.ToLower(unescape(s))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		if r == ' ' || r == '_' {
			return '-'
		}
		return -1
	}, s)
	return s
}

// trim/unescape helpers delegate to the host text.* namespace.
func trim(s string) string     { return hostString("text.trim", s) }
func unescape(s string) string { return hostString("text.unescape", s) }

// coverID extracts the numeric manga id from a mangapill cover URL
// (https://cdn.readdetectiveconan.com/file/mangapill/i/<id>.<ext>), or "".
var reCoverID = regexp.MustCompile(`/mangapill/i/(\d+)`)

func coverID(u string) string {
	if m := reCoverID.FindStringSubmatch(u); m != nil {
		return m[1]
	}
	return ""
}

// ─── ABI exports ────────────────────────────────────────────────────────────

//go:wasmexport contract_version
func contract_version() int32 { return types.ContractVersion }

//go:wasmexport Search
func Search(ptr, n uint32) uint64 { return reply(search(readString(ptr, n))) }

//go:wasmexport GetMangaDetail
func GetMangaDetail(ptr, n uint32) uint64 { return reply(detail(readString(ptr, n))) }

//go:wasmexport GetChapterList
func GetChapterList(ptr, n uint32) uint64 { return reply(chaptersImpl(readString(ptr, n))) }

//go:wasmexport GetPageList
func GetPageList(ptr, n uint32) uint64 { return reply(pagesImpl(readString(ptr, n))) }

//go:wasmexport GetGenres
func GetGenres(ptr, n uint32) uint64 { return reply(genresImpl()) }

// ─── keep unused imports referenced for the tinygo build ────────────────────

var _ = time.Time{}
