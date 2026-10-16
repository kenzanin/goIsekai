// KunManga (kunmanga.online) source plugin for goIsekai — standard-Go WASM reactor
// (GOOS=wasip1 GOARCH=wasm -buildmode=c-shared).
//
// KunManga is WordPress Madara-like with JSON APIs:
//   search:   GET /manga/searching?title=<query>
//             → {"success":true,"data":[{comic_id,name,slug,image_original,
//                author,artist,status,kind,detail,...}]}
//   chapters: GET /api/comics/<slug>/chapters?page=&per_page=50&order=desc
//             → {"success":true,"data":{"chapters":[{comic_id,chapter_id,
//                chapter_num,chapter_name,chapter_slug,updated_at,view}],...}}
//   genres:   scrape /manga-genre/<slug> listing for /manga-genre/* links
//   detail:   scrape /manga/<slug> (og:image, summary__content, genres-content)
//   reader:   /manga/<slug>/<chapter-slug>, images in data-src
//             https://img-r1.2xstorage.com/<slug>/<n>/<i>.webp
//
// ID scheme:
//   Manga.ID   = slug, e.g. "isekai-all-purpose-supermarket"
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

const siteURL = "https://www.kunmanga.online"

const acceptJSON = `{"Accept":"application/json","Referer":"https://www.kunmanga.online/"}`

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
	ComicID int    `json:"comic_id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Image   string `json:"image_original"`
	Author  string `json:"author"`
	Artist  string `json:"artist"`
	Status  string `json:"status"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail"`
}

type searchResp struct {
	Success bool         `json:"success"`
	Data    []searchItem `json:"data"`
}

type apiChapter struct {
	ComicID int     `json:"comic_id"`
	ID      int     `json:"chapter_id"`
	Num     float64 `json:"chapter_num"`
	Name    string  `json:"chapter_name"`
	Slug    string  `json:"chapter_slug"`
	Updated string  `json:"updated_at"`
}

type chaptersResp struct {
	Success bool `json:"success"`
	Data    struct {
		Chapters  []apiChapter `json:"chapters"`
		Total     int          `json:"total"`
		LastPage  int          `json:"last_page"`
		PerPage   int          `json:"per_page"`
		Current   int          `json:"current_page"`
	} `json:"data"`
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

	// Genre-only filter → scrape genre listing page.
	if strings.TrimSpace(f.Query) == "" && len(f.Genres) > 0 {
		return genreListing(f.Genres[0], f.Page)
	}

	q := url.Values{}
	q.Set("title", f.Query)
	body, ok := getJSON(siteURL + "/manga/searching?" + q.Encode())
	if !ok {
		return nil
	}
	var resp searchResp
	if json.Unmarshal([]byte(body), &resp) != nil || !resp.Success {
		return nil
	}
	out := make([]types.Manga, 0, len(resp.Data))
	for _, it := range resp.Data {
		cover := strings.TrimSpace(it.Image)
		if cover != "" && !strings.HasPrefix(cover, "http") {
			cover = siteURL + cover
		}
		m := types.Manga{
			ID:       strings.TrimSpace(it.Slug),
			Title:    strings.TrimSpace(it.Name),
			CoverURL: cover,
			Author:   strings.TrimSpace(it.Author),
			Status:   normalizeStatus(it.Status),
		}
		if kinds := splitKinds(it.Kind); len(kinds) > 0 {
			m.Genres = kinds
		}
		out = append(out, m)
	}
	return out
}

// genreListing scrapes /manga-genre/<slug> cards (.page-item-detail).
func genreListing(genre string, page int) []types.Manga {
	slug := genreSlug(genre)
	u := siteURL + "/manga-genre/" + url.PathEscape(slug)
	if page > 1 {
		u += "/page/" + strconv.Itoa(page)
	}
	body, ok := getBody(u)
	if !ok {
		return nil
	}
	cardRe := regexp.MustCompile(`(?s)<div class="page-item-detail[^>]*>(.*?)</div>\s*</div>\s*</div>`)
	linkRe := regexp.MustCompile(`<a href=["']?https?://www\.kunmanga\.online/manga/([^/"'>\s]+)["'\s>]`)
	titleRe := regexp.MustCompile(`title="Read ([^"]+?) Manga Online`)
	imgRe := regexp.MustCompile(`(?:data-src|src)="(https?://[^"]+\.(?:webp|jpg|jpeg|png))"`)
	out := []types.Manga{}
	for _, cm := range cardRe.FindAllStringSubmatch(body, 60) {
		card := cm[1]
		lm := linkRe.FindStringSubmatch(card)
		if lm == nil {
			continue
		}
		m := types.Manga{ID: lm[1]}
		if tm := titleRe.FindStringSubmatch(card); tm != nil {
			m.Title = html.UnescapeString(strings.TrimSpace(tm[1]))
		} else {
			m.Title = strings.ReplaceAll(lm[1], "-", " ")
		}
		if im := imgRe.FindStringSubmatch(card); im != nil {
			m.CoverURL = im[1]
		}
		m.Genres = []string{humanize(slug)}
		out = append(out, m)
	}
	return out
}

// ─── detail ─────────────────────────────────────────────────────────────────

func detailImpl(arg string) types.Manga {
	slug := strings.TrimSpace(decodeString(arg))
	if slug == "" {
		return types.Manga{}
	}
	body, ok := getBody(siteURL + "/manga/" + url.PathEscape(slug))
	if !ok {
		return types.Manga{}
	}
	m := types.Manga{ID: slug}
	if tm := regexp.MustCompile(`<h1[^>]*>(.*?)</h1>`).FindStringSubmatch(body); tm != nil {
		m.Title = html.UnescapeString(stripTags(tm[1]))
	}
	if m.Title == "" {
		m.Title = strings.ReplaceAll(slug, "-", " ")
	}
	if om := regexp.MustCompile(`og:image"\s*content="([^"]+)"`).FindStringSubmatch(body); om != nil {
		m.CoverURL = om[1]
	}
	if dm := regexp.MustCompile(`(?s)summary__content[^>]*>(.*?)</div>`).FindStringSubmatch(body); dm != nil {
		m.Description = html.UnescapeString(strings.TrimSpace(stripTags(dm[1])))
	}
	if sm := regexp.MustCompile(`(?s)post-status[^>]*>.*?<span[^>]*>\s*Status\s*</span>\s*([^<]+)`).FindStringSubmatch(body); sm != nil {
		m.Status = normalizeStatus(sm[1])
	}
	genRe := regexp.MustCompile(`manga-genre/([a-z0-9-]+)">([^<]+)`)
	if gm := regexp.MustCompile(`(?s)genres-content[^>]*>(.*?)</div>`).FindStringSubmatch(body); gm != nil {
		for _, g := range genRe.FindAllStringSubmatch(gm[1], 40) {
			m.Genres = append(m.Genres, html.UnescapeString(strings.TrimSpace(g[2])))
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
	out := []types.Chapter{}
	page := 1
	for {
		u := siteURL + "/api/comics/" + url.PathEscape(slug) + "/chapters?page=" + strconv.Itoa(page) + "&per_page=50&order=desc"
		body, ok := getJSON(u)
		if !ok {
			break
		}
		var resp chaptersResp
		if json.Unmarshal([]byte(body), &resp) != nil || !resp.Success {
			break
		}
		for _, c := range resp.Data.Chapters {
			out = append(out, types.Chapter{
				ID:         slug + ":" + strings.TrimSpace(c.Slug),
				MangaID:    slug,
				Title:      strings.TrimSpace(c.Name),
				ChapterNum: c.Num,
				ReleasedAt: parseTime(c.Updated),
				URL:        siteURL + "/manga/" + slug + "/" + strings.TrimSpace(c.Slug),
			})
		}
		if resp.Data.Current >= resp.Data.LastPage || resp.Data.LastPage == 0 {
			break
		}
		page++
		if page > 40 { // ponytail: 2000-chapter ceiling; paginate further if needed
			break
		}
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
		pageURL = siteURL + "/manga/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])
	} else {
		pageURL = chID
	}
	body, ok := getBody(pageURL)
	if !ok {
		return nil
	}
	imgRe := regexp.MustCompile(`data-src=['"](https?://[^'"]+\.(?:webp|jpg|jpeg|png))['"]`)
	seen := map[string]bool{}
	out := []types.Page{}
	for _, m := range imgRe.FindAllStringSubmatch(body, 500) {
		u := m[1]
		if seen[u] || strings.Contains(u, "/thumb/") {
			continue
		}
		seen[u] = true
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
	body, ok := getBody(siteURL + "/manga-genre/action")
	if ok {
		genRe := regexp.MustCompile(`manga-genre/([a-z0-9-]+)">([^<]+)`)
		seen := map[string]bool{}
		names = nil
		for _, g := range genRe.FindAllStringSubmatch(body, 100) {
			name := html.UnescapeString(strings.TrimSpace(g[2]))
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	// ponytail: static fallback list above when live fetch fails.
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

func splitKinds(kind string) []string {
	out := []string{}
	for _, k := range strings.Split(kind, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			out = append(out, k)
		}
	}
	return out
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

func stripTags(s string) string {
	return regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, " ")
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
