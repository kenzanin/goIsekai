// Komix.to (komix.to) source plugin for goIsekai — standard-Go WASM reactor
// (GOOS=wasip1 GOARCH=wasm -buildmode=c-shared).
//
// Komix.to is Next.js + JSON API:
//   search: GET /api/search?q=<query>
//           → {"results":[{"id,title,slug,cover_image,status,type,...}]}
//           cover_image like "/api/images?path=<slug>%2Fcover.jpg"
//   detail: scrape /title/<slug> (og:image, og:description, /genres/* links,
//           embedded \"chapters\":[{id,manga_id,chapter_number,title,...}])
//   pages:  scrape /title/<slug>/chapter/<n>
//           embedded \"pages\":[{page_number,image_url,enc,...}]
//           image_url is empty; real URL = /api/images?t=<enc>
//   genres: scrape /genres index for /genres/* links
//
// ID scheme:
//   Manga.ID   = slug, e.g. "isekai-mesu-yuusha"
//   Chapter.ID = "<mangaSlug>:<chapterNum>"
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

const siteURL = "https://komix.to"

const acceptJSON = `{"Accept":"application/json","Referer":"https://komix.to/"}`

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

func getJSON(u string) (string, bool) { return httpBody("http.get_body", u, acceptJSON) }

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

// ─── types ──────────────────────────────────────────────────────────────────

type searchItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Slug   string `json:"slug"`
	Cover  string `json:"cover_image"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

type searchResp struct {
	Results []searchItem `json:"results"`
}

type apiChapter struct {
	ID     string  `json:"id"`
	Manga  string  `json:"manga_id"`
	Num    float64 `json:"chapter_number"`
	Title  string  `json:"title"`
	Create string  `json:"created_at"`
}

type apiPage struct {
	Num float64 `json:"page_number"`
	URL string  `json:"image_url"`
	Enc string  `json:"enc"`
}

// ─── search ─────────────────────────────────────────────────────────────────

func searchImpl(arg string) []types.Manga {
	var f types.SearchFilter
	if json.Unmarshal([]byte(arg), &f) != nil {
		return nil
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if strings.TrimSpace(f.Query) == "" && len(f.Genres) == 0 {
		return nil
	}
	if strings.TrimSpace(f.Query) == "" && len(f.Genres) > 0 {
		return genreListing(f.Genres[0])
	}
	q := url.Values{}
	q.Set("q", f.Query)
	body, ok := getJSON(siteURL + "/api/search?" + q.Encode())
	if !ok {
		return nil
	}
	var resp searchResp
	if json.Unmarshal([]byte(body), &resp) != nil {
		return nil
	}
	out := make([]types.Manga, 0, len(resp.Results))
	for _, it := range resp.Results {
		cover := strings.TrimSpace(it.Cover)
		if cover != "" && strings.HasPrefix(cover, "/") {
			cover = siteURL + cover
		}
		out = append(out, types.Manga{
			ID:       strings.TrimSpace(it.Slug),
			Title:    strings.TrimSpace(it.Title),
			CoverURL: cover,
			Status:   normalizeStatus(it.Status),
		})
	}
	return out
}

// genreListing scrapes /genres/<slug> cards (links to /title/<slug>).
func genreListing(genre string) []types.Manga {
	slug := genreSlug(genre)
	body, ok := getBody(siteURL + "/genres/" + url.PathEscape(slug))
	if !ok {
		return nil
	}
	linkRe := regexp.MustCompile(`/title/([a-z0-9-]+)`)
	seen := map[string]bool{}
	out := []types.Manga{}
	for _, m := range linkRe.FindAllStringSubmatch(body, 120) {
		s := m[1]
		if seen[s] || s == "" {
			continue
		}
		seen[s] = true
		out = append(out, types.Manga{
			ID:     s,
			Title:  strings.ReplaceAll(s, "-", " "),
			Genres: []string{humanize(slug)},
		})
		if len(out) >= 48 {
			break
		}
	}
	return out
}

// ─── detail ─────────────────────────────────────────────────────────────────

func detailImpl(arg string) types.Manga {
	slug := strings.TrimSpace(decodeString(arg))
	if slug == "" {
		return types.Manga{}
	}
	body, ok := getBody(siteURL + "/title/" + url.PathEscape(slug))
	if !ok {
		return types.Manga{}
	}
	m := types.Manga{ID: slug}
	if tm := regexp.MustCompile(`og:title"\s*content="([^"]+)"`).FindStringSubmatch(body); tm != nil {
		t := strings.TrimSpace(tm[1])
		t = strings.TrimSuffix(t, " - Read Free Online | Komix")
		m.Title = html.UnescapeString(t)
	}
	if m.Title == "" {
		m.Title = strings.ReplaceAll(slug, "-", " ")
	}
	if om := regexp.MustCompile(`og:image"\s*content="([^"]+)"`).FindStringSubmatch(body); om != nil {
		m.CoverURL = html.UnescapeString(om[1])
	}
	if dm := regexp.MustCompile(`og:description"\s*content="([^"]+)"`).FindStringSubmatch(body); dm != nil {
		m.Description = html.UnescapeString(dm[1])
	}
	genRe := regexp.MustCompile(`/genres/([a-z0-9-]+)`)
	seen := map[string]bool{}
	for _, g := range genRe.FindAllStringSubmatch(body, 40) {
		name := humanize(g[1])
		if !seen[name] {
			seen[name] = true
			m.Genres = append(m.Genres, name)
		}
	}
	return m
}

// ─── chapters ───────────────────────────────────────────────────────────────

func chaptersImpl(arg string) []types.Chapter {
	slug := strings.TrimSpace(decodeString(arg))
	if slug == "" {
		return nil
	}
	body, ok := getBody(siteURL + "/title/" + url.PathEscape(slug))
	if !ok {
		return nil
	}
	// Embedded JSON is escaped: \"chapters\":[{\"id\":...}]
	re := regexp.MustCompile(`\\?"chapters\\?":\[(.*?)\]`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		return nil
	}
	raw := "[" + strings.ReplaceAll(m[1], `\"`, `"`) + "]"
	var chs []apiChapter
	if json.Unmarshal([]byte(raw), &chs) != nil {
		return nil
	}
	out := make([]types.Chapter, 0, len(chs))
	for _, c := range chs {
		num := strconv.FormatFloat(c.Num, 'f', -1, 64)
		out = append(out, types.Chapter{
			ID:         slug + ":" + num,
			MangaID:    slug,
			Title:      strings.TrimSpace(c.Title),
			ChapterNum: c.Num,
			ReleasedAt: parseTime(c.Create),
			URL:        siteURL + "/title/" + slug + "/chapter/" + num,
		})
	}
	return out
}

// ─── pages ──────────────────────────────────────────────────────────────────

func pagesImpl(arg string) []types.Page {
	chID := strings.TrimSpace(decodeString(arg))
	if chID == "" {
		return nil
	}
	parts := strings.SplitN(chID, ":", 2)
	var pageURL string
	if len(parts) == 2 {
		pageURL = siteURL + "/title/" + url.PathEscape(parts[0]) + "/chapter/" + url.PathEscape(parts[1])
	} else {
		pageURL = chID
	}
	body, ok := getBody(pageURL)
	if !ok {
		return nil
	}
	raw := strings.ReplaceAll(body, `\"`, `"`)
	re := regexp.MustCompile(`"pages":\[(.*?)\]`)
	m := re.FindStringSubmatch(raw)
	if m == nil {
		return nil
	}
	var pgs []apiPage
	if json.Unmarshal([]byte("["+m[1]+"]"), &pgs) != nil {
		return nil
	}
	out := []types.Page{}
	for _, p := range pgs {
		u := strings.TrimSpace(p.URL)
		if u == "" && strings.TrimSpace(p.Enc) != "" {
			u = siteURL + "/api/images?t=" + strings.TrimSpace(p.Enc)
		}
		if u == "" {
			continue
		}
		out = append(out, types.Page{Index: len(out), URL: u})
	}
	return out
}

// ─── genres ─────────────────────────────────────────────────────────────────

type genre struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func genresImpl() []genre {
	names := []string{"Action", "Adventure", "Comedy", "Drama", "Fantasy", "Isekai", "Romance"}
	body, ok := getBody(siteURL + "/genres")
	if ok {
		genRe := regexp.MustCompile(`/genres/([a-z0-9-]+)`)
		seen := map[string]bool{}
		names = nil
		for _, g := range genRe.FindAllStringSubmatch(body, 80) {
			name := humanize(g[1])
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	out := make([]genre, 0, len(names))
	for _, n := range names {
		out = append(out, genre{Name: n, Slug: n})
	}
	return out
}

// ─── helpers ────────────────────────────────────────────────────────────────

func decodeString(arg string) string {
	var s string
	_ = json.Unmarshal([]byte(arg), &s)
	return s
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

func genreSlug(g string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(g), " ", "-"))
}

func humanize(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
