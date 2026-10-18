//go:build wasip1

// ReiManga (reimanga.net) source plugin for goIsekai — standard-Go WASM reactor
// (GOOS=wasip1 GOARCH=wasm -buildmode=c-shared, or TinyGo wasm).
//
// https://reimanga.net has a clean JSON API at /api/manga/...
// No HTML scraping needed for search/detail/chapters/pages.
//
// Endpoints (from HAR):
//   search:   GET /api/manga/search/advanced?esOnly=1&page=N&limit=24&sort=viewed&order=desc&genres=...&tags=...&title=...
//   detail:   GET /api/manga/<numericID>  (need to confirm)
//   genres:   GET /api/genres (confirm)
//
// ID scheme: Manga.ID = numeric manga ID as string, e.g. "82201"
//            Chapter.ID = "<mangaID>:<chapterID>"

package main

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"goisekai/pkg/types"
)

const siteURL = "https://reimanga.net"
const acceptJSON = `{"Accept":"application/json","Referer":"https://reimanga.net/"}`

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
func free(ptr uint32) {
	delete(buffers, ptr)
}

//go:wasmimport env host_call
func hostCallRaw(ptr, length uint32) uint64

func allocCopy(b []byte) uint32 {
	if len(b) == 0 {
		return 0
	}
	n := uint32(len(b))
	ptr := malloc(n)
	copy(buffers[ptr], b)
	return ptr
}

func readString(ptr, n uint32) string {
	if n == 0 {
		return ""
	}
	b, ok := buffers[ptr]
	if !ok {
		return ""
	}
	return string(b[:n])
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

func getJSON(u string) (string, bool) {
	raw := hostCall("http.get_body", u, acceptJSON)
	if raw == "" || raw == "null" || strings.HasPrefix(raw, "{") && strings.Contains(raw, "error") {
		return "", false
	}
	var s string
	if json.Unmarshal([]byte(raw), &s) != nil {
		return "", false
	}
	return s, s != ""
}

func main() {}

// ─── exports ─────────────────────────────────────────────────────────────────

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

// ─── types ───────────────────────────────────────────────────────────────────

type searchResult struct {
	Data []struct {
		ID           int     `json:"id"`
		Title        string  `json:"title"`
		AltTitle     string  `json:"alt_title"`
		NameURL      string  `json:"name_url"`
		CoverURL     string  `json:"cover_url"`
		ViewCount    int     `json:"view_count"`
		Rating       float64 `json:"rating"`
		IsAdult      bool    `json:"is_adult"`
		Status       int     `json:"status"`
		ChapterNum   string  `json:"chapter_number"`
		ChapterTitle string  `json:"chapter_title"`
		Updated      string  `json:"chapter_updated_at"`
	} `json:"data"`
}

type named struct {
	Name string `json:"name"`
}

// /api/manga/<id> answers {"manga":{...},"similar":[...]} — there is no "data"
// envelope here (that is the /api/chapters/manga/<id> shape).
type detailResp struct {
	Manga struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		AltTitle    string  `json:"alt_title"`
		NameURL     string  `json:"name_url"`
		CoverURL    string  `json:"cover_url"`
		Description string  `json:"description"`
		IsAdult     bool    `json:"is_adult"`
		Status      int     `json:"status"`
		Completed   bool    `json:"completed"`
		Authors     []named `json:"authors"`
		Genres      []named `json:"genres"`
		Updated     string  `json:"updated_at"`
	} `json:"manga"`
}

// Chapters come from /api/chapters/manga/<id> as {"data":[...],"pagination":{...}}.
type chaptersResp struct {
	Data []struct {
		ID            int    `json:"id"`
		ChapterNumber string `json:"chapter_number"`
		Name          string `json:"name"`
		CreatedAt     string `json:"created_at"`
	} `json:"data"`
}

// Page URLs come from /api/chapters/<id>/images; a bare /api/chapters/<id>
// returns chapter metadata with no page list at all.
type pageResp struct {
	Images []struct {
		ImageURL string `json:"image_url"`
	} `json:"images"`
}

// ─── search ──────────────────────────────────────────────────────────────────

func searchImpl(arg string) []types.Manga {
	var f types.SearchFilter
	if json.Unmarshal([]byte(arg), &f) != nil {
		return nil
	}
	if f.Page < 1 {
		f.Page = 1
	}

	q := url.Values{}
	q.Set("esOnly", "1")
	q.Set("page", strconv.Itoa(f.Page))
	q.Set("limit", "24")
	q.Set("sort", "viewed")
	q.Set("order", "desc")
	if f.Query != "" {
		q.Set("title", f.Query)
	}
	if len(f.Genres) > 0 {
		q.Set("genres", strings.Join(f.Genres, ","))
	}

	body, ok := getJSON(siteURL + "/api/manga/search/advanced?" + q.Encode())
	if !ok {
		return nil
	}
	var resp searchResult
	if json.Unmarshal([]byte(body), &resp) != nil {
		return nil
	}
	out := make([]types.Manga, 0, len(resp.Data))
	for _, it := range resp.Data {
		out = append(out, types.Manga{
			ID:       strconv.Itoa(it.ID),
			Title:    strings.TrimSpace(it.Title),
			CoverURL: siteURL + "/covers/" + strconv.Itoa(it.ID) + "/thumbnail.png",
			Status:   normalizeStatus(it.Status),
		})
	}
	return out
}

func detailImpl(arg string) types.Manga {
	id := strings.TrimSpace(decodeString(arg))
	if id == "" {
		return types.Manga{}
	}
	body, ok := getJSON(siteURL + "/api/manga/" + url.PathEscape(id))
	if !ok {
		return types.Manga{}
	}
	var resp detailResp
	if json.Unmarshal([]byte(body), &resp) != nil {
		return types.Manga{}
	}
	d := resp.Manga
	genres := make([]string, 0, len(d.Genres))
	for _, g := range d.Genres {
		if n := strings.TrimSpace(g.Name); n != "" {
			genres = append(genres, n)
		}
	}
	authors := make([]string, 0, len(d.Authors))
	for _, a := range d.Authors {
		if n := strings.TrimSpace(a.Name); n != "" {
			authors = append(authors, n)
		}
	}
	m := types.Manga{
		ID:          strconv.Itoa(d.ID),
		Title:       strings.TrimSpace(d.Title),
		CoverURL:    siteURL + "/covers/" + strconv.Itoa(d.ID) + "/thumbnail.png",
		Description: strings.TrimSpace(d.Description),
		Status:      normalizeStatus(d.Status),
		Genres:      genres,
		Author:      strings.Join(authors, ", "),
	}
	return m
}

func chaptersImpl(arg string) []types.Chapter {
	mangaID := strings.TrimSpace(decodeString(arg))
	if mangaID == "" {
		return nil
	}
	// limit=1000 because the default page is 50: a partial list looks like the
	// manga ends there (144 chapters would arrive as 3 pages of 50).
	body, ok := getJSON(siteURL + "/api/chapters/manga/" + url.PathEscape(mangaID) + "?limit=1000")
	if !ok {
		return nil
	}
	var resp chaptersResp
	if json.Unmarshal([]byte(body), &resp) != nil {
		return nil
	}
	out := make([]types.Chapter, 0, len(resp.Data))
	for _, c := range resp.Data {
		title := strings.TrimSpace(c.Name)
		if title == "" {
			title = "Chapter " + c.ChapterNumber
		}
		num, _ := strconv.ParseFloat(strings.TrimSpace(c.ChapterNumber), 64)
		out = append(out, types.Chapter{
			ID:         mangaID + ":" + strconv.Itoa(c.ID),
			MangaID:    mangaID,
			Title:      title,
			ChapterNum: num,
			ReleasedAt: parseTime(c.CreatedAt),
		})
	}
	return out
}

func pagesImpl(arg string) []types.Page {
	mangaID, chID, ok := strings.Cut(decodeString(arg), ":")
	if !ok || mangaID == "" || chID == "" {
		return nil
	}
	body, ok := getJSON(siteURL + "/api/chapters/" + url.PathEscape(chID) + "/images")
	if !ok {
		return nil
	}
	var resp pageResp
	if json.Unmarshal([]byte(body), &resp) != nil {
		return nil
	}
	out := make([]types.Page, 0, len(resp.Images))
	for _, p := range resp.Images {
		u := strings.TrimSpace(p.ImageURL)
		if u == "" {
			continue
		}
		out = append(out, types.Page{Index: len(out), URL: u})
	}
	return out
}

type genre struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func genresImpl() []genre {
	// There *is* a genres endpoint; the old hardcoded list made the filter row
	// a different set from what the site actually filters on.
	if body, ok := getJSON(siteURL + "/api/genres"); ok {
		var raw []struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		}
		if json.Unmarshal([]byte(body), &raw) == nil {
			out := make([]genre, 0, len(raw))
			for _, g := range raw {
				name := strings.TrimSpace(g.Name)
				if name == "" {
					continue
				}
				slug := strings.TrimSpace(g.Slug)
				if slug == "" {
					slug = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
				}
				out = append(out, genre{Name: name, Slug: slug})
			}
			if len(out) > 0 {
				return out
			}
		}
	}

	// Fallback when /api/genres is unreachable, so the filter row is never empty.
	names := []string{
		"Fantasy", "Action", "Adventure", "Comedy", "Drama",
		"Romance", "Mystery", "Thriller", "Horror", "Isekai",
		"Sci-Fi", "Slice of Life", "Supernatural",
	}
	out := make([]genre, 0, len(names))
	for _, n := range names {
		out = append(out, genre{Name: n, Slug: n})
	}
	return out
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func decodeString(arg string) string {
	var s string
	_ = json.Unmarshal([]byte(arg), &s)
	return s
}

func normalizeStatus(s int) string {
	switch s {
	case 0:
		return "Ongoing"
	case 1:
		return "Completed"
	default:
		return ""
	}
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

var reNum = regexp.MustCompile(`\d+(?:\.\d+)?`)

func chapterNum(title string) float64 {
	if v := reNum.FindString(title); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}
