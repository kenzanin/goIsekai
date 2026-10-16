//go:build wasip1

// Manhwato (manhwato.com) source plugin for goIsekai — standard-Go WASM reactor
// (GOOS=wasip1 GOARCH=wasm -buildmode=c-shared).
//
// Manhwato is plain server-rendered HTML (Madara-like theme):
//   genre listing: /manhwa-genre/<slug>
//   search:        /search?q=<query>
//   detail:        /manhwa/<slug>
//   chapter:       /manhwa/<slug>/<chapter-slug>  (e.g. chapter-0-ch264518)
//   pages:         <img> tags with /images/manga/<slug>/<chapter>/<n>.jpg
//
// ID scheme:
//   Manga.ID   = manhwa slug, e.g. "isekai-meikyuu-de-harem-wo"
//   Chapter.ID = "<mangaSlug>:<chapterSlug>"
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

const siteURL = "https://manhwato.com"

// ─── ABI memory ─────────────────────────────────────────────────────────────

var buffers = map[uint32][]byte{}

//go:wasmexport malloc
func malloc(n uint32) uint32 {
	if n == 0 {
		n = 1
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
	if len(b) == 0 {
		return 0
	}
	ptr := malloc(uint32(len(b)))
	copy(buffers[ptr], b)
	return ptr
}

func pack(ptr, n uint32) uint64 { return uint64(n)<<32 | uint64(ptr) }

func reply(v any) uint64 {
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 {
		b = []byte("null")
	}
	return pack(allocCopy(b), uint32(len(b)))
}

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

func getBody(u string) (string, bool) { return httpBody("http.get_body", u) }

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

// ─── patterns ───────────────────────────────────────────────────────────────

var (
	reTag     = regexp.MustCompile(`(?s)<[^>]*>`)
	reSpace   = regexp.MustCompile(`\s+`)
	reNum     = regexp.MustCompile(`\d+(?:\.\d+)?`)
	reCard    = regexp.MustCompile(`(?s)<div class="visual">(.*?)</div>\s*</div>`)
	reCover   = regexp.MustCompile(`(?s)<div class="novel-cover">(.*?)</div>`)
	reTitleH3 = regexp.MustCompile(`(?s)<h3 class="title">(.*?)</h3>`)
	reAnchor  = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
	reHref    = regexp.MustCompile(`href="([^"]+)"`)
	reTitleAt = regexp.MustCompile(`title="([^"]+)"`)
	reImg     = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	reChapUl  = regexp.MustCompile(`(?s)<ul class="chapter-list">(.*?)</ul>`)
	reChapLi  = regexp.MustCompile(`(?s)<li>(.*?)</li>`)
	reTime    = regexp.MustCompile(`datetime="([^"]+)"`)
	reGenre   = regexp.MustCompile(`(?s)<a[^>]*href="/manhwa-genre/([^"/]+)/?"[^>]*>([^<]+)</a>`)
	reOGImage = regexp.MustCompile(`<meta[^>]*property="og:image"[^>]*content="([^"]+)"`)
	reOGDesc  = regexp.MustCompile(`<meta[^>]*property="og:description"[^>]*content="([^"]+)"`)
	reStatus  = regexp.MustCompile(`(?is)status[^<]{0,60}?(ongoing|completed|hiatus|cancelled)`)
)

var imgAttrs = []*regexp.Regexp{
	regexp.MustCompile(`(?s)(?:\s|^)data-src="\s*([^"]+)"`),
	regexp.MustCompile(`(?s)(?:\s|^)src="\s*([^"]+)"`),
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

func mangaURL(slug string) string { return siteURL + "/manhwa/" + url.PathEscape(slug) }

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
	case strings.Contains(l, "complete"), strings.Contains(l, "finished"):
		return "Completed"
	case strings.Contains(l, "hiatus"), strings.Contains(l, "hold"), strings.Contains(l, "pause"):
		return "Hiatus"
	case strings.Contains(l, "cancel"), strings.Contains(l, "drop"):
		return "Cancelled"
	}
	return ""
}

// ─── search ─────────────────────────────────────────────────────────────────

func parseCards(body string) []types.Manga {
	out := []types.Manga{}
	seen := map[string]bool{}
	for _, m := range reCard.FindAllStringSubmatch(body, -1) {
		seg := m[1]
		h3 := firstSub(reTitleH3, seg)
		href := firstSub(reHref, h3)
		if href == "" {
			href = firstSub(reHref, seg)
		}
		slug := slugOf(href)
		if slug == "" || seen[slug] || strings.Contains(href, "/chapter-") {
			continue
		}
		seen[slug] = true
		title := clean(firstSub(reAnchor, h3))
		if title == "" {
			title = clean(firstSub(reTitleAt, h3))
		}
		out = append(out, types.Manga{
			ID:       slug,
			Title:    title,
			CoverURL: absURL(imgURL(firstSub(reCover, seg))),
		})
	}
	return out
}

func searchImpl(arg string) []types.Manga {
	var f types.SearchFilter
	if json.Unmarshal([]byte(arg), &f) != nil {
		return nil
	}
	if f.Page < 1 {
		f.Page = 1
	}
	q := strings.TrimSpace(f.Query)
	var body string
	var ok bool
	if q == "" && len(f.Genres) > 0 {
		body, ok = getBody(siteURL + "/manhwa-genre/" + url.PathEscape(strings.ToLower(f.Genres[0])))
	} else if q == "" {
		body, ok = getBody(siteURL + "/manhwa-genre/isekai")
	} else {
		body, ok = getBody(siteURL + "/search?q=" + url.QueryEscape(q))
	}
	if !ok {
		return nil
	}
	return parseCards(body)
}

// ─── detail ─────────────────────────────────────────────────────────────────

func detailImpl(arg string) types.Manga {
	slug := decodeString(arg)
	m := types.Manga{ID: slug}
	if slug == "" {
		return m
	}
	body, ok := getBody(mangaURL(slug))
	if !ok {
		return m
	}
	if t := firstSub(reOGImage, body); t != "" {
		m.CoverURL = strings.TrimSpace(t)
	}
	if d := firstSub(reOGDesc, body); d != "" {
		m.Description = strings.TrimSpace(html.UnescapeString(d))
	}
	if h := regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`).FindStringSubmatch(body); len(h) > 1 {
		m.Title = clean(h[1])
	}
	if s := firstSub(reStatus, body); s != "" {
		m.Status = normalizeStatus(s)
	}
	seen := map[string]bool{}
	for _, g := range reGenre.FindAllStringSubmatch(body, -1) {
		name := clean(g[2])
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		m.Genres = append(m.Genres, name)
	}
	return m
}

// ─── chapters ───────────────────────────────────────────────────────────────

func chaptersImpl(arg string) []types.Chapter {
	slug := decodeString(arg)
	if slug == "" {
		return nil
	}
	body, ok := getBody(mangaURL(slug))
	if !ok {
		return nil
	}
	ul := firstSub(reChapUl, body)
	if ul == "" {
		ul = body
	}
	out := []types.Chapter{}
	seen := map[string]bool{}
	for _, m := range reChapLi.FindAllStringSubmatch(ul, -1) {
		href := firstSub(reHref, m[1])
		cslug := slugOf(href)
		if cslug == "" || seen[cslug] {
			continue
		}
		seen[cslug] = true
		label := clean(firstSub(reAnchor, m[1]))
		if label == "" {
			label = clean(firstSub(reTitleAt, m[1]))
		}
		num := 0.0
		if v := reNum.FindString(label); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				num = f
			}
		}
		var rel time.Time
		if ds := firstSub(reTime, m[1]); ds != "" {
			for _, layout := range []string{"2006-01-02", time.RFC3339} {
				if t, err := time.Parse(layout, ds); err == nil {
					rel = t
					break
				}
			}
		}
		out = append(out, types.Chapter{
			ID:         slug + ":" + cslug,
			MangaID:    slug,
			Title:      label,
			ChapterNum: num,
			ReleasedAt: rel,
			URL:        absURL(href),
		})
	}
	return out
}

// ─── pages ──────────────────────────────────────────────────────────────────

func pagesImpl(arg string) []types.Page {
	slug, cslug, ok := strings.Cut(decodeString(arg), ":")
	if !ok || slug == "" || cslug == "" {
		return nil
	}
	body, ok := getBody(mangaURL(slug) + "/" + url.PathEscape(cslug))
	if !ok {
		return nil
	}
	out := []types.Page{}
	for _, tag := range reImg.FindAllString(body, -1) {
		src := imgURL(tag)
		if src == "" || !strings.Contains(src, "/images/manga/") {
			continue
		}
		out = append(out, types.Page{Index: len(out), URL: absURL(src)})
	}
	return out
}

// ─── genres ─────────────────────────────────────────────────────────────────

type genre struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func genresImpl() []genre {
	body, ok := getBody(siteURL + "/")
	if !ok {
		return nil
	}
	out := []genre{}
	seen := map[string]bool{}
	for _, m := range reGenre.FindAllStringSubmatch(body, -1) {
		slug, name := m[1], clean(m[2])
		if slug == "" || name == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, genre{Name: name, Slug: slug})
	}
	return out
}
