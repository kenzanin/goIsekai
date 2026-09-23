//go:build wasip1

// KSGroupScans source plugin for goIsekai — a standard-Go WASM reactor
// (GOOS=wasip1 GOARCH=wasm, -buildmode=c-shared).
//
// Ported from the keiyoushi KSGroupScans extension at
// extension-source/src/en/ksgroupscans (site https://ksgroupscans.com). The
// extension is a bare Madara subclass whose only override is
// chapterMode = MangaAjax, so the selectors and endpoints come from
// extension-source/lib-multisrc/madara:
//
//   - search/popular/latest: admin-ajax "madara_load_more" archive endpoint
//   - chapter list:          POST <mangaURL>/ajax/chapters/
//   - genres:                /manga/ listing page links
//   - pages:                 chapter page exposes wp-manga-chapter-img tags
//
// ID scheme (same convention as the other mangaka WASM plugins):
//
//	Manga.ID   = manga slug, e.g. "ntrevenge"
//	Chapter.ID = "<mangaSlug>:<chapterSlug>"
package main

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unsafe"

	"goisekai/pkg/types"
)

const (
	siteURL   = "https://ksgroupscans.com"
	ajaxURL   = siteURL + "/wp-admin/admin-ajax.php"
	mangaPath = "/manga/" // Madara mangaSubString
)

// ─── ABI memory ─────────────────────────────────────────────────────────────
//
// A wasip1 build exports no malloc/free, so the host reaches guest memory
// through these two. Buffers stay referenced in the map so the collector
// cannot reclaim them while the host still holds the pointer; the host frees
// them through the free export.
// ponytail: wasm32 keeps every heap address < 4 GiB, so uintptr -> uint32 is safe.

var buffers = map[uint32][]byte{}

//go:wasmexport malloc
func malloc(n uint32) uint32 {
	if n == 0 {
		n = 1 // the host reads a zero pointer as allocation failure
	}
	b := make([]byte, n)
	ptr := uint32(uintptr(unsafe.Pointer(&b[0])))
	buffers[ptr] = b
	return ptr
}

//go:wasmexport free
func free(ptr uint32) { delete(buffers, ptr) }

//go:wasmimport env host_call
func hostCallRaw(ptr, length uint32) uint64

func readString(ptr, n uint32) string {
	b, ok := buffers[ptr]
	if !ok || uint32(len(b)) < n {
		return ""
	}
	return string(b[:n])
}

func allocCopy(b []byte) uint32 {
	ptr := malloc(uint32(len(b)))
	copy(buffers[ptr], b)
	return ptr
}

// pack combines pointer and length into the i64 the ABI returns.
func pack(ptr, n uint32) uint64 { return uint64(n)<<32 | uint64(ptr) }

// reply marshals v and hands the bytes to the host.
func reply(v any) uint64 {
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 {
		b = []byte("null")
	}
	return pack(allocCopy(b), uint32(len(b)))
}

// hostCall invokes one env.host_call helper. The host JSON-encodes the result,
// so a string body arrives as a JSON string literal and a failure as
// {"error":...}.
func hostCall(fn string, args ...string) string {
	req, err := json.Marshal(struct {
		Fn   string   `json:"fn"`
		Args []string `json:"args"`
	}{fn, args})
	if err != nil {
		return ""
	}
	ptr := allocCopy(req)
	out := hostCallRaw(ptr, uint32(len(req)))
	free(ptr)
	resp, n := uint32(out), uint32(out>>32)
	if resp == 0 || n == 0 {
		return ""
	}
	s := readString(resp, n)
	free(resp)
	return s
}

func getBody(u string) (string, bool) { return httpBody("http.get_body", u) }

func postForm(u, form string) (string, bool) {
	return httpBody("http.post_body", u, form,
		`{"X-Requested-With":"XMLHttpRequest","Content-Type":"application/x-www-form-urlencoded"}`)
}

func httpBody(fn string, args ...string) (string, bool) {
	raw := hostCall(fn, args...)
	if raw == "" || raw == "null" || strings.HasPrefix(raw, "{") {
		return "", false
	}
	var s string
	if json.Unmarshal([]byte(raw), &s) != nil {
		return "", false
	}
	return s, s != ""
}

// main is required by the Go linker for package main; -buildmode=c-shared makes
// the module a reactor (initialised through _initialize, main never runs).
func main() {}

// ─── exports ────────────────────────────────────────────────────────────────

//go:wasmexport contract_version
func contract_version() int32 { return types.ContractVersion }

//go:wasmexport Search
func Search(ptr, n uint32) uint64 { return reply(search(readString(ptr, n))) }

//go:wasmexport GetMangaDetail
func GetMangaDetail(ptr, n uint32) uint64 { return reply(detail(readString(ptr, n))) }

//go:wasmexport GetChapterList
func GetChapterList(ptr, n uint32) uint64 { return reply(chapters(readString(ptr, n))) }

//go:wasmexport GetPageList
func GetPageList(ptr, n uint32) uint64 { return reply(pages(readString(ptr, n))) }

//go:wasmexport GetGenres
func GetGenres(ptr, n uint32) uint64 { return reply(genres()) }

func mangaURL(slug string) string { return siteURL + mangaPath + url.PathEscape(slug) + "/" }

// ─── parsing ────────────────────────────────────────────────────────────────

var (
	reTag    = regexp.MustCompile(`(?s)<[^>]*>`)
	reSpace  = regexp.MustCompile(`\s+`)
	reNumber = regexp.MustCompile(`\d+(?:\.\d+)?`)

	// Archive cards: split on the card marker, then read each card's own
	// anchor/image. KSGroupScans puts the thumbnail anchor href BEFORE its own
	// <img>, so a "closest image above the slug" scan would take the previous
	// card's cover.
	reCardStart  = regexp.MustCompile(`class="page-item-detail`)
	reTitleBlock = regexp.MustCompile(`(?s)class="post-title[^"]*">(.*?)</div>`)
	reHeading    = regexp.MustCompile(`(?s)<h[1-6][^>]*>(.*?)</h[1-6]>`)
	reImage      = regexp.MustCompile(`(?is)<img\b[^>]*>`)

	// MangaAjax chapter list.
	reChapterCard = regexp.MustCompile(`(?s)<li[^>]*class="wp-manga-chapter[^"]*"[^>]*>(.*?)</li>`)
	reAnchor      = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
	reHref        = regexp.MustCompile(`href="([^"]+)"`)

	reCoverBlock  = regexp.MustCompile(`(?s)class="summary_image".*?</a>`)
	reAuthorBlock = regexp.MustCompile(`(?s)class="author-content">(.*?)</div>`)
	reGenreBlock  = regexp.MustCompile(`(?s)class="genres-content">(.*?)</div>`)
	reGenreAnchor = regexp.MustCompile(`(?s)<a[^>]*>([^<]+)</a>`)
	reStatus      = regexp.MustCompile(`(?s)Status\s*</h5>.*?class="summary-content">\s*([^<]+)`)
	reDescMore    = regexp.MustCompile(`(?s)class="summary__content[^"]*">(.*?)<span\s+class="[^"]*content-readmore"`)
	reDescBlock   = regexp.MustCompile(`(?s)class="summary__content[^"]*">(.*?)</div>`)

	reGenreLink = regexp.MustCompile(`(?s)<a[^>]*href="([^"]*manga-genre/([^"/]+)/?)"[^>]*>(.*?)</a>`)

	imgAttrs = []*regexp.Regexp{
		attrRe("data-src"), attrRe("data-lazy-src"), attrRe("data-cfsrc"),
		attrRe("data-manga-src"), attrRe("src"),
	}
)

func attrRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)(?:\s|^)` + name + `="\s*([^"]+)"`)
}

// imgURL returns the best lazy-aware image URL from an <img> tag or window.
func imgURL(s string) string {
	for _, re := range imgAttrs {
		if m := re.FindStringSubmatch(s); m != nil {
			if v := strings.TrimSpace(m[1]); v != "" {
				return v
			}
		}
	}
	return ""
}

// clean strips tags, unescapes entities and collapses whitespace.
func clean(s string) string {
	s = reTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(reSpace.ReplaceAllString(s, " "))
}

func firstSub(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return ""
}

func slugOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	p := strings.TrimSuffix(u.Path, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func absURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if !u.IsAbs() {
		if b, err := url.Parse(siteURL); err == nil {
			u = b.ResolveReference(u)
		}
	}
	return u.String()
}

func decodeString(arg string) string {
	var s string
	_ = json.Unmarshal([]byte(arg), &s)
	return strings.TrimSpace(s)
}

func normalizeStatus(s string) string {
	l := strings.ToLower(s)
	switch {
	case strings.Contains(l, "ongoing"), strings.Contains(l, "on going"), strings.Contains(l, "updating"):
		return "Ongoing"
	case strings.Contains(l, "complete"):
		return "Completed"
	case strings.Contains(l, "hiatus"), strings.Contains(l, "hold"), strings.Contains(l, "pause"):
		return "Hiatus"
	case strings.Contains(l, "cancel"):
		return "Cancelled"
	}
	return ""
}

func chapterNum(title string) float64 {
	if v := reNumber.FindString(title); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

// parseCards splits an archive listing into per-card segments and reads each
// card's own href/title/thumbnail, so covers never bleed across cards.
func parseCards(body string) []types.Manga {
	locs := reCardStart.FindAllStringIndex(body, -1)
	out := make([]types.Manga, 0, len(locs))
	seen := map[string]bool{}
	for i, loc := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		seg := body[loc[0]:end]
		slug := slugOf(firstSub(reHref, seg))
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, types.Manga{
			ID:       slug,
			Title:    clean(firstSub(reHeading, firstSub(reTitleBlock, seg))),
			CoverURL: absURL(imgURL(reImage.FindString(seg))),
		})
	}
	return out
}

// ─── ABI implementations ────────────────────────────────────────────────────

// search uses Madara's AdminAjax archive endpoint. An empty query browses by
// views (the Popular sort); a query uses WordPress search relevance.
func search(arg string) []types.Manga {
	var f types.SearchFilter
	if json.Unmarshal([]byte(arg), &f) != nil {
		return nil
	}
	if f.Page < 1 {
		f.Page = 1
	}
	form := url.Values{}
	form.Set("action", "madara_load_more")
	form.Set("page", strconv.Itoa(f.Page-1))
	form.Set("template", "madara-core/content/content-archive")
	form.Set("vars[paged]", "1")
	form.Set("vars[template]", "archive")
	form.Set("vars[posts_per_page]", "25")
	form.Set("vars[post_type]", "wp-manga")
	form.Set("vars[post_status]", "publish")
	form.Set("vars[manga_archives_item_layout]", "big_thumbnail")
	if q := strings.TrimSpace(f.Query); q != "" {
		form.Set("vars[s]", q)
	} else {
		form.Set("vars[orderby]", "meta_value_num")
		form.Set("vars[meta_key]", "_wp_manga_views")
		form.Set("vars[order]", "DESC")
	}
	if len(f.Genres) > 0 {
		form.Set("vars[tax_query][0][taxonomy]", "wp-manga-genre")
		form.Set("vars[tax_query][0][field]", "slug")
		for i, g := range f.Genres {
			form.Set(fmt.Sprintf("vars[tax_query][0][terms][%d]", i), g)
		}
	}
	body, ok := postForm(ajaxURL, form.Encode())
	if !ok {
		return nil
	}
	return parseCards(body)
}

func detail(arg string) types.Manga {
	slug := decodeString(arg)
	m := types.Manga{ID: slug}
	if slug == "" {
		return m
	}
	body, ok := getBody(mangaURL(slug))
	if !ok {
		return m
	}
	m.Title = clean(firstSub(reHeading, firstSub(reTitleBlock, body)))
	m.CoverURL = absURL(imgURL(reCoverBlock.FindString(body)))
	m.Author = clean(firstSub(reAuthorBlock, body))
	m.Description = clean(firstSub(reDescMore, body))
	if m.Description == "" {
		m.Description = clean(firstSub(reDescBlock, body))
	}
	if s := firstSub(reStatus, body); s != "" {
		m.Status = normalizeStatus(s)
	}
	if gb := firstSub(reGenreBlock, body); gb != "" {
		for _, g := range reGenreAnchor.FindAllStringSubmatch(gb, -1) {
			if n := clean(g[1]); n != "" {
				m.Genres = append(m.Genres, n)
			}
		}
	}
	return m
}

// chapters posts to the MangaAjax endpoint (ChapterMode.MangaAjax) and parses
// the returned li.wp-manga-chapter cards.
func chapters(arg string) []types.Chapter {
	slug := decodeString(arg)
	if slug == "" {
		return nil
	}
	body, ok := postForm(mangaURL(slug)+"ajax/chapters/", "")
	if !ok {
		return nil
	}
	out := make([]types.Chapter, 0, 32)
	seen := map[string]bool{}
	for _, m := range reChapterCard.FindAllStringSubmatch(body, -1) {
		href := firstSub(reHref, m[1])
		if href == "" {
			continue
		}
		cslug := slugOf(href)
		if cslug == "" || seen[cslug] {
			continue
		}
		seen[cslug] = true
		label := clean(firstSub(reAnchor, m[1]))
		out = append(out, types.Chapter{
			ID:         slug + ":" + cslug,
			MangaID:    slug,
			Title:      label,
			ChapterNum: chapterNum(label),
			URL:        absURL(href),
		})
	}
	return out
}

func pages(arg string) []types.Page {
	slug, chapterSlug, ok := strings.Cut(decodeString(arg), ":")
	if !ok || slug == "" || chapterSlug == "" {
		return nil
	}
	chapterURL := mangaURL(slug) + url.PathEscape(chapterSlug) + "/"
	body, ok := getBody(chapterURL)
	if !ok {
		return nil
	}
	// Madara defaults to a paged reader; the list layout exposes the real
	// <img> tags in one document.
	if strings.Contains(body, `id="single-pager"`) {
		if b, ok := getBody(chapterURL + "?style=list"); ok {
			body = b
		}
	}
	var out []types.Page
	for _, tag := range reImage.FindAllString(body, -1) {
		if !strings.Contains(tag, "wp-manga-chapter-img") {
			continue
		}
		src := imgURL(tag)
		if src == "" {
			continue
		}
		out = append(out, types.Page{Index: len(out), URL: absURL(src)})
	}
	return out
}

type genre struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func genres() []genre {
	body, ok := getBody(siteURL + mangaPath)
	if !ok {
		return nil
	}
	out := make([]genre, 0, 24)
	seen := map[string]bool{}
	for _, m := range reGenreLink.FindAllStringSubmatch(body, -1) {
		slug, name := m[2], clean(m[3])
		if slug == "" || name == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, genre{Name: name, Slug: slug})
	}
	return out
}
