//go:build wasip1

// MangaUno (manga.uno) source plugin for goIsekai — standard-Go WASM reactor
//
// https://manga.uno is a Next.js site with JSON API at /api/...
//
// From HAR:
//   search:   GET /api/search/advanced?title=...&genres=...&limit=24
//   genres:   GET /api/genres
//   detail:   GET /api/manga/<slug>    -> {"manga":{...},"chapters":[...]}
//   chapters: GET /api/manga/<slug>    -> top-level "chapters" (same response)
//   pages:    GET /api/chapter/<id>    -> {"chapter":{...},"pages":[...]}
//
// ID scheme: Manga.ID = slug, e.g. "mushoku-tensei-jobless-reincarnation"
//            Chapter.ID = the site's numeric chapter id
//
// Covers and pages resolve against the CDN origin (xz7.fstr-cdn.com), never
// manga.uno: manga.uno/covers/*.webp answers an HTML shell, which broke every
// cover and reader image.

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

const siteURL = "https://manga.uno"
const apiURL = "https://manga.uno/api"
const cdnURL = "https://xz7.fstr-cdn.com"

// cdn resolves a site path the way the site itself does — against the CDN
// origin, because manga.uno serves an HTML shell for /covers/*.webp.
func cdn(raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "//") {
		return "https:" + p
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	if strings.HasPrefix(p, "/") {
		return cdnURL + p
	}
	return cdnURL + "/" + p
}

const acceptJSON = `{"Accept":"application/json","Referer":"https://manga.uno/"}`

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

func allocCopy(b []byte) uint32 {
	if len(b) == 0 {
		return 0
	}
	ptr := malloc(uint32(len(b)))
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

type searchItem struct {
	ID       int    `json:"id"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	English  string `json:"english_title"`
	Japanese string `json:"japanese_title"`
	Cover    string `json:"cover"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Updated  string `json:"last_chapter_at"`
}

type searchResp struct {
	Data []searchItem `json:"data"`
}

type mangaDetail struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	English   string `json:"english_title"`
	Synopsis  string `json:"synopsis"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	TotalChap int    `json:"total_chapters"`
	Cover     string `json:"cover"`
	Author    string `json:"author"`
}

type mangaResp struct {
	Manga    *mangaDetail  `json:"manga"`
	Chapters []chapterItem `json:"chapters"`
}

type chapterItem struct {
	ID            int64  `json:"id"`
	MangaID       int64  `json:"manga_id"`
	ChapterNumber string `json:"chapter_number"`
	Title         string `json:"title"`
	PageCount     int    `json:"page_count"`
	PublishedAt   string `json:"published_at"`
	CreatedAt     string `json:"created_at"`
}

type chapterResp struct {
	Chapter *chapterItem `json:"chapter"`
	Pages   []string     `json:"pages"`
}

type genresResp struct {
	Genres []string `json:"genres"`
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
	if f.Query != "" {
		q.Set("title", f.Query)
	}
	if len(f.Genres) > 0 {
		q.Set("genres", f.Genres[0])
	}
	q.Set("limit", "24")

	body, ok := getJSON(apiURL + "/search/advanced?" + q.Encode())
	if !ok {
		return nil
	}
	var resp searchResp
	if json.Unmarshal([]byte(body), &resp) != nil {
		return nil
	}
	out := make([]types.Manga, 0, len(resp.Data))
	for _, it := range resp.Data {
		out = append(out, types.Manga{
			ID:       it.Slug,
			Title:    strings.TrimSpace(it.Title),
			CoverURL: cdn(it.Cover),
			Status:   normalizeStatus(it.Status),
		})
	}
	return out
}

func detailImpl(arg string) types.Manga {
	slug := strings.TrimSpace(decodeString(arg))
	if slug == "" {
		return types.Manga{}
	}
	body, ok := getJSON(apiURL + "/manga/" + url.PathEscape(slug))
	if !ok {
		return types.Manga{}
	}
	var resp mangaResp
	if json.Unmarshal([]byte(body), &resp) != nil || resp.Manga == nil {
		return types.Manga{}
	}
	m := resp.Manga
	return types.Manga{
		ID:          m.Slug,
		Title:       strings.TrimSpace(m.Title),
		CoverURL:    cdn(m.Cover),
		Author:      strings.TrimSpace(m.Author),
		Description: strings.TrimSpace(m.Synopsis),
		Status:      normalizeStatus(m.Status),
	}
}

func chaptersImpl(arg string) []types.Chapter {
	slug := strings.TrimSpace(decodeString(arg))
	if slug == "" {
		return nil
	}
	body, ok := getJSON(apiURL + "/manga/" + url.PathEscape(slug))
	if !ok {
		return nil
	}
	var resp mangaResp
	if json.Unmarshal([]byte(body), &resp) != nil || resp.Manga == nil {
		return nil
	}

	// The chapter list rides along on the detail response — /search/advanced
	// returns *other manga*, which is how this used to emit one fake chapter.
	out := make([]types.Chapter, 0, len(resp.Chapters))
	for _, c := range resp.Chapters {
		number := strings.TrimSpace(c.ChapterNumber)
		title := strings.TrimSpace(c.Title)
		if title == "" {
			title = "Chapter " + number
		}
		num, _ := strconv.ParseFloat(number, 64)
		released := c.PublishedAt
		if released == "" {
			released = c.CreatedAt
		}
		out = append(out, types.Chapter{
			ID:         strconv.FormatInt(c.ID, 10),
			MangaID:    slug,
			Title:      title,
			ChapterNum: num,
			ReleasedAt: parseTime(released),
			URL:        siteURL + "/" + slug,
		})
	}
	return out
}

func pagesImpl(arg string) []types.Page {
	chID := strings.TrimSpace(decodeString(arg))
	if chID == "" {
		return nil
	}
	body, ok := getJSON(apiURL + "/chapter/" + chID)
	if !ok {
		return nil
	}
	var resp chapterResp
	if json.Unmarshal([]byte(body), &resp) != nil {
		return nil
	}
	// Page URLs sit at the TOP level of the response, not under "chapter".
	out := make([]types.Page, 0, len(resp.Pages))
	for _, p := range resp.Pages {
		if p == "" {
			continue
		}
		out = append(out, types.Page{Index: len(out), URL: p})
	}
	if len(out) > 0 || resp.Chapter == nil {
		return out
	}
	// pages omitted (uncached): they resolve to /pages/<manga_id>/<chapter_id>/<n>.webp
	c := resp.Chapter
	if c.MangaID <= 0 || c.ID <= 0 || c.PageCount <= 0 {
		return out
	}
	for i := 1; i <= c.PageCount; i++ {
		out = append(out, types.Page{Index: len(out), URL: cdnURL + "/pages/" +
			strconv.FormatInt(c.MangaID, 10) + "/" + strconv.FormatInt(c.ID, 10) +
			"/" + strconv.Itoa(i) + ".webp"})
	}
	return out
}

type genre struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func genresImpl() []genre {
	body, ok := getJSON(apiURL + "/genres")
	if !ok {
		return nil
	}
	var resp genresResp
	if json.Unmarshal([]byte(body), &resp) != nil {
		return nil
	}
	out := make([]genre, 0, len(resp.Genres))
	for _, n := range resp.Genres {
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

func chapterNum(title string) float64 {
	re := regexp.MustCompile(`\d+(?:\.\d+)?`)
	if v := re.FindString(title); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
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
