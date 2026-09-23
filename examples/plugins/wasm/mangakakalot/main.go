//go:build wasip1

// MangaKakalot source plugin for goIsekai — a standard-Go WASM reactor
// (GOOS=wasip1 GOARCH=wasm, -buildmode=c-shared).
//
// Ported from the keiyoushi MangaBox multisrc used by
// extension-source/src/en/mangakakalot (site https://www.mangakakalove.com,
// the current live mirror; ww2.mangakakalots.com 404s every app route).
//
// ID scheme:
//
//	Manga.ID   = manga slug, e.g. "one-piece"
//	Chapter.ID = "<mangaSlug>:<chapterSlug>"
//
// Chapter list is a JSON API (/api/manga/<slug>/chapters?limit=-1); page
// images are scraped from the `cdns = [...]` / `chapterImages = [...]`
// script arrays on the chapter page.
package main

import (
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"goisekai/pkg/types"
)

const siteURL = "https://www.mangakakalove.com"

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
func Search(ptr, n uint32) uint64 { return reply(searchImpl(readString(ptr, n))) }

//go:wasmexport GetMangaDetail
func GetMangaDetail(ptr, n uint32) uint64 { return reply(detailImpl(readString(ptr, n))) }

//go:wasmexport GetChapterList
func GetChapterList(ptr, n uint32) uint64 { return reply(chaptersImpl(readString(ptr, n))) }

//go:wasmexport GetPageList
func GetPageList(ptr, n uint32) uint64 { return reply(pagesImpl(readString(ptr, n))) }

//go:wasmexport GetGenres
func GetGenres(ptr, n uint32) uint64 { return reply(genresImpl()) }

// ─── Parsing ────────────────────────────────────────────────────────────────

var (
	reTag    = regexp.MustCompile(`(?s)<[^>]*>`)
	reSpace  = regexp.MustCompile(`\s+`)
	reNumber = regexp.MustCompile(`\d+(?:\.\d+)?`)

	reCard = regexp.MustCompile(`(?s)<h3[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)

	reTitle      = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	reTitleH2    = regexp.MustCompile(`(?is)<h2[^>]*>(.*?)</h2>`)
	reCoverTop   = regexp.MustCompile(`(?is)class="manga-info-pic".*?<img[^>]*>`)
	reCoverPanel = regexp.MustCompile(`(?is)class="info-image".*?<img[^>]*>`)
	reDesc1      = regexp.MustCompile(`(?is)id="noidungm"[^>]*>(.*?)</div>`)
	reDesc2      = regexp.MustCompile(`(?is)id="panel-story-info-description"[^>]*>(.*?)</div>`)
	reDesc3      = regexp.MustCompile(`(?is)id="contentBox"[^>]*>(.*?)</div>`)
	reStatus     = regexp.MustCompile(`(?is)\bStatus\b\s*:?\s*(?:<[^>]*>\s*)*([A-Za-z][A-Za-z ]{2,24})`)

	reImage = regexp.MustCompile(`(?is)<img\b[^>]*>`)

	reCdns         = regexp.MustCompile(`cdns\s*=\s*\[([^]]+)]`)
	reBackupImage  = regexp.MustCompile(`backupImage\s*=\s*\[([^]]+)]`)
	reChapterImage = regexp.MustCompile(`chapterImages\s*=\s*\[([^]]+)]`)

	reQueryPunct = regexp.MustCompile(`[!@%^*()+ =<>\?/,\.:;'"&#\[\]~_$-]`)
	reCollapse   = regexp.MustCompile(`_+`)
	reTrimUnder  = regexp.MustCompile(`^_+|_+$`)

	reVietnamese = map[*regexp.Regexp]string{
		regexp.MustCompile(`[àáạảãâầấậẩẫăằắặẳẵ]`): "a",
		regexp.MustCompile(`[èéẹẻẽêềếệểễ]`):        "e",
		regexp.MustCompile(`[ìíịỉĩ]`):              "i",
		regexp.MustCompile(`[òóọỏõôồốộổỗơờớợởỡ]`): "o",
		regexp.MustCompile(`[ùúụủũưừứựửữ]`):        "u",
		regexp.MustCompile(`[ỳýỵỷỹ]`):              "y",
		regexp.MustCompile(`đ`):                    "d",
	}

	imgAttrs = []*regexp.Regexp{
		attrRe("data-src"), attrRe("data-lazy-src"), attrRe("src"),
	}
)

func attrRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)(?:\s|^)` + name + `="\s*([^"]+)"`)
}

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

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
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

func chapterNum(label string) float64 {
	if m := reNumber.FindString(label); m != "" {
		if f, err := strconv.ParseFloat(m, 64); err == nil {
			return f
		}
	}
	return 0
}

func normalizeStatus(s string) string {
	l := strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.Contains(l, "ongoing"):
		return "Ongoing"
	case strings.Contains(l, "complete"):
		return "Completed"
	case strings.Contains(l, "hiatus"):
		return "Hiatus"
	case strings.Contains(l, "cancel"), strings.Contains(l, "dropped"):
		return "Cancelled"
	}
	return ""
}

// normalizeQuery mirrors MangaBox's change_alias diceware transform.
func normalizeQuery(q string) string {
	s := strings.ToLower(q)
	for re, r := range reVietnamese {
		s = re.ReplaceAllString(s, r)
	}
	s = reQueryPunct.ReplaceAllString(s, "_")
	s = reCollapse.ReplaceAllString(s, "_")
	return reTrimUnder.ReplaceAllString(s, "")
}

// cardIndex locates the card link whose href ends in slug. Matching on the
// quoted path avoids treating a slug as a prefix of a longer one.
func cardIndex(body, slug string) int {
	for _, needle := range []string{`/manga/` + slug + `"`, `/manga/` + slug + `/`} {
		if i := strings.Index(body, needle); i >= 0 {
			return i
		}
	}
	return strings.Index(body, slug)
}

// coverNear returns the cover image of the card whose link contains slug.
// Current MangaBox markup nests <img> inside the anchor that carries the slug;
// older markup places <img> before the title link. Prefer a following image
// whose URL embeds the slug, otherwise use the nearest preceding image.
func coverNear(body, slug string) string {
	i := cardIndex(body, slug)
	if i < 0 {
		return ""
	}
	if j := strings.Index(body[i:], "<img"); j >= 0 {
		j += i
		if k := strings.IndexByte(body[j:], '>'); k >= 0 {
			if u := imgURL(body[j : j+k+1]); strings.Contains(u, slug) {
				return u
			}
		}
	}
	j := strings.LastIndex(body[:i], "<img")
	if j < 0 {
		return ""
	}
	k := strings.IndexByte(body[j:], '>')
	if k < 0 {
		return ""
	}
	return imgURL(body[j : j+k+1])
}

func extractArray(s string, re *regexp.Regexp) []string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return nil
	}
	var out []string
	for _, p := range strings.Split(m[1], ",") {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, `"`)
		p = strings.TrimSuffix(p, `"`)
		p = strings.ReplaceAll(p, `\/`, "/")
		p = strings.TrimSuffix(p, "/")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ─── ABI implementations ────────────────────────────────────────────────────

func searchImpl(arg string) []types.Manga {
	var f types.SearchFilter
	if json.Unmarshal([]byte(arg), &f) != nil {
		return nil
	}
	if f.Page < 1 {
		f.Page = 1
	}
	var target string
	if q := strings.TrimSpace(f.Query); q != "" {
		target = siteURL + "/search/story/" + normalizeQuery(q) + "?page=" + strconv.Itoa(f.Page)
	} else {
		target = siteURL + "/manga-list/hot-manga?page=" + strconv.Itoa(f.Page)
	}
	body, ok := httpBody(target)
	if !ok {
		return nil
	}
	out := make([]types.Manga, 0, 24)
	seen := map[string]bool{}
	for _, m := range reCard.FindAllStringSubmatch(body, -1) {
		href, title := m[1], clean(m[2])
		slug := slugOf(href)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, types.Manga{ID: slug, Title: title, CoverURL: absURL(coverNear(body, slug))})
	}
	return out
}

func detailImpl(arg string) types.Manga {
	slug := decodeString(arg)
	m := types.Manga{ID: slug}
	if slug == "" {
		return m
	}
	body, ok := httpBody(siteURL + "/manga/" + url.PathEscape(slug))
	if !ok {
		return m
	}
	m.Title = firstNonEmpty(clean(firstSub(reTitle, body)), clean(firstSub(reTitleH2, body)))
	m.CoverURL = absURL(imgURL(firstNonEmpty(reCoverTop.FindString(body), reCoverPanel.FindString(body))))
	m.Description = clean(firstNonEmpty(
		firstSub(reDesc1, body),
		firstSub(reDesc2, body),
		firstSub(reDesc3, body),
	))
	if s := firstSub(reStatus, body); s != "" {
		m.Status = normalizeStatus(s)
	}
	return m
}

func chaptersImpl(arg string) []types.Chapter {
	slug := decodeString(arg)
	if slug == "" {
		return nil
	}
	body, ok := httpBody(siteURL + "/api/manga/" + url.PathEscape(slug) + "/chapters?limit=-1")
	if !ok {
		return nil
	}
	var resp struct {
		Success bool `json:"success"`
		Data    *struct {
			Chapters []struct {
				ChapterName *string `json:"chapter_name"`
				ChapterSlug *string `json:"chapter_slug"`
				ChapterNum  float64 `json:"chapter_num"`
				UpdatedAt   *string `json:"updated_at"`
			} `json:"chapters"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(body), &resp) != nil || resp.Data == nil {
		return nil
	}
	out := make([]types.Chapter, 0, len(resp.Data.Chapters))
	for _, c := range resp.Data.Chapters {
		if c.ChapterSlug == nil || *c.ChapterSlug == "" {
			continue
		}
		title := "Chapter"
		if c.ChapterName != nil && *c.ChapterName != "" {
			title = *c.ChapterName
		}
		var released time.Time
		if c.UpdatedAt != nil {
			if t, err := time.Parse(time.RFC3339, *c.UpdatedAt); err == nil {
				released = t
			}
		}
		out = append(out, types.Chapter{
			ID:         slug + ":" + *c.ChapterSlug,
			MangaID:    slug,
			Title:      title,
			ChapterNum: c.ChapterNum,
			ReleasedAt: released,
			URL:        siteURL + "/manga/" + url.PathEscape(slug) + "/" + *c.ChapterSlug,
		})
	}
	return out
}

func pagesImpl(arg string) []types.Page {
	slug, chapterSlug, ok := strings.Cut(decodeString(arg), ":")
	if !ok || slug == "" || chapterSlug == "" {
		return nil
	}
	chapterURL := siteURL + "/manga/" + url.PathEscape(slug) + "/" + url.PathEscape(chapterSlug)
	body, ok := httpBody(chapterURL)
	if !ok {
		return nil
	}
	cdns := extractArray(body, reCdns)
	cdns = append(cdns, extractArray(body, reBackupImage)...)
	images := extractArray(body, reChapterImage)
	headers := map[string]string{"Referer": siteURL}

	var out []types.Page
	if len(images) > 0 && len(cdns) > 0 {
		base, err := url.Parse(cdns[0])
		if err != nil {
			return nil
		}
		for _, p := range images {
			full := p
			if !strings.HasPrefix(p, "http") {
				u := *base
				if !strings.HasPrefix(p, "/") {
					p = "/" + p
				}
				u.Path = p
				full = u.String()
			}
			out = append(out, types.Page{Index: len(out), URL: full, Headers: headers})
		}
		return out
	}
	for _, tag := range reImage.FindAllString(body, -1) {
		src := imgURL(tag)
		if src == "" {
			continue
		}
		out = append(out, types.Page{Index: len(out), URL: absURL(src), Headers: headers})
	}
	return out
}

type genre struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// genresImpl returns no server-side genre filter list (MangaBox hard-codes its
// filters client-side); a valid empty array keeps the host contract happy.
func genresImpl() []genre { return []genre{} }
