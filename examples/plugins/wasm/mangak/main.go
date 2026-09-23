//go:build wasip1

// MangaK source plugin for goIsekai — standard-Go WASM reactor
// (GOOS=wasip1 GOARCH=wasm -buildmode=c-shared).
//
// https://mangak.io is a Next.js front end over a JSON API at
// https://api.mangak.io. The site's Comics client (chunks/pages/_app) maps to:
//
//	titles/search?q=..&page=..&genre=..   titles/:hsid
//	titles/:hsid/chapters                 titles/:hsid/chapters/:chapterId
//	genres
//
// Everything is plain JSON served with a Referer gate — no HTML scraping.
//
// ID scheme:
//
//	Manga.ID   = API title id (hsid), e.g. "b2KAxZ4Y"
//	Chapter.ID = "<hsid>:<chapterId>"
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

const (
	siteURL = "https://mangak.io"
	apiURL  = "https://api.mangak.io"
)

// The API only checks Accept + same-site Referer; both are cheap insurance.
const acceptJSON = `{"Accept":"application/json, text/plain, */*","Referer":"https://mangak.io/"}`

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

// getJSON fetches an API URL with the JSON accept header.
func getJSON(u string) (string, bool) { return httpBody("http.get_body", u, acceptJSON) }

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

// ─── DTOs (api.mangak.io shapes) ────────────────────────────────────────────

// named is the {name,slug} pair used by genres, authors and the genre list.
type named struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type titleItem struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Slug   string  `json:"slug"`
	Cover  string  `json:"cover"`
	Status string  `json:"status"`
	Genres []named `json:"genres"`
}

type searchResp struct {
	Data struct {
		Items []titleItem `json:"items"`
	} `json:"data"`
}

type titleDetail struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Cover   string  `json:"cover"`
	Status  string  `json:"status"`
	Summary string  `json:"summary"`
	Genres  []named `json:"genres"`
	Authors []named `json:"authors"`
}

type detailResp struct {
	Data struct {
		Title *titleDetail `json:"title"`
	} `json:"data"`
}

type chapterItem struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Name      string `json:"name"`
	UpdatedAt string `json:"updated_at"`
}

type chaptersResp struct {
	Data struct {
		Chapters []chapterItem `json:"chapters"`
	} `json:"data"`
}

type pagesResp struct {
	Data struct {
		Chapter *struct {
			Images []string `json:"images"`
		} `json:"chapter"`
	} `json:"data"`
}

type genresResp struct {
	Data struct {
		Items []named `json:"items"`
	} `json:"data"`
}

// ─── search / detail ────────────────────────────────────────────────────────

func searchImpl(arg string) []types.Manga {
	var f types.SearchFilter
	if json.Unmarshal([]byte(arg), &f) != nil {
		return nil
	}
	if f.Page < 1 {
		f.Page = 1
	}
	body, ok := getJSON(queryURL(f))
	if !ok {
		return nil
	}
	var resp searchResp
	if json.Unmarshal([]byte(body), &resp) != nil {
		return nil
	}
	out := make([]types.Manga, 0, len(resp.Data.Items))
	for _, it := range resp.Data.Items {
		if it.ID == "" {
			continue
		}
		out = append(out, types.Manga{
			ID:       it.ID,
			Title:    it.Name,
			CoverURL: it.Cover,
			Status:   normalizeStatus(it.Status),
		})
	}
	return out
}

// queryURL builds /titles/search. An empty query returns the site's default
// popular listing, which is what the browse view relies on.
func queryURL(f types.SearchFilter) string {
	q := url.Values{}
	q.Set("q", strings.TrimSpace(f.Query))
	q.Set("page", strconv.Itoa(f.Page))
	if len(f.Genres) > 0 {
		q.Set("genre", f.Genres[0])
	}
	return apiURL + "/titles/search?" + q.Encode()
}

func detailImpl(arg string) types.Manga {
	id := decodeString(arg)
	if id == "" {
		return types.Manga{}
	}
	body, ok := getJSON(apiURL + "/titles/" + url.PathEscape(id))
	if !ok {
		return types.Manga{}
	}
	var resp detailResp
	if json.Unmarshal([]byte(body), &resp) != nil || resp.Data.Title == nil {
		return types.Manga{}
	}
	return toManga(resp.Data.Title)
}

func toManga(t *titleDetail) types.Manga {
	m := types.Manga{
		ID:          t.ID,
		Title:       t.Name,
		CoverURL:    t.Cover,
		Description: strings.TrimSpace(t.Summary),
		Status:      normalizeStatus(t.Status),
	}
	for _, g := range t.Genres {
		if g.Name != "" {
			m.Genres = append(m.Genres, g.Name)
		}
	}
	if len(t.Authors) > 0 {
		m.Author = strings.TrimSpace(t.Authors[0].Name)
	}
	return m
}

// ─── chapters ───────────────────────────────────────────────────────────────

func chaptersImpl(arg string) []types.Chapter {
	id := decodeString(arg)
	if id == "" {
		return nil
	}
	body, ok := getJSON(apiURL + "/titles/" + url.PathEscape(id) + "/chapters")
	if !ok {
		return nil
	}
	var resp chaptersResp
	if json.Unmarshal([]byte(body), &resp) != nil {
		return nil
	}
	out := make([]types.Chapter, 0, len(resp.Data.Chapters))
	for _, c := range resp.Data.Chapters {
		if c.ID == "" {
			continue
		}
		title := strings.TrimSpace(c.Name)
		if title == "" {
			title = "Chapter"
		}
		out = append(out, types.Chapter{
			ID:         id + ":" + c.ID,
			MangaID:    id,
			Title:      title,
			ChapterNum: chapterNum(title),
			ReleasedAt: parseTime(c.UpdatedAt),
			URL:        siteURL + c.URL,
		})
	}
	return out
}

// ─── pages ──────────────────────────────────────────────────────────────────

func pagesImpl(arg string) []types.Page {
	id, chID, ok := strings.Cut(decodeString(arg), ":")
	if !ok || id == "" || chID == "" {
		return nil
	}
	// The chapter detail response carries the complete images array inline.
	body, ok := getJSON(apiURL + "/titles/" + url.PathEscape(id) + "/chapters/" + url.PathEscape(chID))
	if !ok {
		return nil
	}
	var resp pagesResp
	if json.Unmarshal([]byte(body), &resp) != nil || resp.Data.Chapter == nil {
		return nil
	}
	out := make([]types.Page, 0, len(resp.Data.Chapter.Images))
	for _, img := range resp.Data.Chapter.Images {
		if img == "" {
			continue
		}
		out = append(out, types.Page{Index: len(out), URL: img})
	}
	return out
}

// ─── genres ─────────────────────────────────────────────────────────────────

func genresImpl() []named {
	body, ok := getJSON(apiURL + "/genres")
	if !ok {
		return nil
	}
	var resp genresResp
	if json.Unmarshal([]byte(body), &resp) != nil {
		return nil
	}
	out := make([]named, 0, len(resp.Data.Items))
	for _, g := range resp.Data.Items {
		if g.Name != "" {
			out = append(out, g)
		}
	}
	return out
}

// ─── helpers ────────────────────────────────────────────────────────────────

var reNum = regexp.MustCompile(`\d+(?:\.\d+)?`)

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

func chapterNum(title string) float64 {
	if v := reNum.FindString(title); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
