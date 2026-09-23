//go:build wasip1

// OmegaScans source plugin for goIsekai — standard-Go WASM reactor
// (GOOS=wasip1 GOARCH=wasm, -buildmode=c-shared).
//
// Ported from the keiyoushi HeanCms multisrc extension
// extension-source/src/en/omegascans (site https://omegascans.org, JSON API at
// https://api.omegascans.org). HeanCms.kt / HeanCmsDto.kt in
// extension-source/lib-multisrc/heancms are the parsing reference.
//
// ID scheme (same convention as the mangaka WASM plugin):
//
//	Manga.ID   = series slug, e.g. "sex-stopwatch"
//	Chapter.ID = "<seriesSlug>:<chapterSlug>"
//
// Chapter listing needs the numeric series id, so GetChapterList re-reads the
// series detail endpoint first.
package main

import (
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"goisekai/pkg/types"
)

const (
	siteURL = "https://omegascans.org"
	apiURL  = "https://api.omegascans.org"
)

const acceptJSON = `{"Accept":"application/json, text/plain, */*"}`

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

// ─── DTOs (HeanCmsDto.kt) ───────────────────────────────────────────────────

type series struct {
	ID     int    `json:"id"`
	Slug   string `json:"series_slug"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Studio string `json:"studio"`
	Status string `json:"status"`
	Thumb  string `json:"thumbnail"`
	Desc   string `json:"description"`
	Tags   []tag  `json:"tags"`
}

type tag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type pager struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
}

type queryResp struct {
	Data []series `json:"data"`
	Meta *pager   `json:"meta"`
}

type chapter struct {
	Name    string  `json:"chapter_name"`
	Title   *string `json:"chapter_title"`
	Slug    string  `json:"chapter_slug"`
	Price   int     `json:"price"`
	Created string  `json:"created_at"`
}

type chapterResp struct {
	Data []chapter `json:"data"`
	Meta *pager    `json:"meta"`
}

type pageResp struct {
	Chapter struct {
		Data *struct {
			Images []string `json:"images"`
		} `json:"chapter_data"`
	} `json:"chapter"`
}

type genre struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
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
	body, ok := getJSON(queryURL(f.Page, f.Query, f.Genres))
	if !ok {
		return nil
	}
	var resp queryResp
	if json.Unmarshal([]byte(body), &resp) != nil {
		return nil
	}
	out := make([]types.Manga, 0, len(resp.Data))
	for _, s := range resp.Data {
		if s.Slug != "" {
			out = append(out, toManga(s))
		}
	}
	return out
}

// queryURL mirrors HeanCms.queryUrlBuilder: "/query" with the default
// popular sort (empty query) or the requested text; genre tags ride as ids.
func queryURL(page int, query string, genres []string) string {
	q := url.Values{}
	q.Set("query_string", query)
	q.Set("status", "All")
	q.Set("order", "desc")
	q.Set("orderBy", "total_views")
	q.Set("series_type", "Comic")
	q.Set("page", strconv.Itoa(page))
	q.Set("perPage", "24")
	q.Set("tags_ids", tagIDs(genres))
	q.Set("adult", "true")
	return apiURL + "/query?" + q.Encode()
}

func tagIDs(genres []string) string {
	if len(genres) == 0 {
		return "[]"
	}
	return "[" + strings.Join(genres, ",") + "]"
}

func detailImpl(arg string) types.Manga {
	slug := decodeString(arg)
	if slug == "" {
		return types.Manga{}
	}
	body, ok := getJSON(apiURL + "/series/" + url.PathEscape(slug))
	if !ok {
		return types.Manga{}
	}
	var s series
	if json.Unmarshal([]byte(body), &s) != nil {
		return types.Manga{}
	}
	return toManga(s)
}

func toManga(s series) types.Manga {
	names := make([]string, 0, len(s.Tags)+1)
	for _, t := range s.Tags {
		if t.Name != "" {
			names = append(names, t.Name)
		}
	}
	sort.Strings(names)
	// ponytail: source appends a synthetic "Manhwa" genre to force webtoon mode;
	// display-only here, drop it if the extra label becomes noise.
	names = append(names, "Manhwa")

	return types.Manga{
		ID:          s.Slug,
		Title:       s.Title,
		CoverURL:    absImage(s.Thumb),
		Author:      strings.TrimSpace(s.Author),
		Description: description(s.Desc),
		Status:      normalizeStatus(s.Status),
		Genres:      names,
	}
}

// ─── chapters ───────────────────────────────────────────────────────────────

func chaptersImpl(arg string) []types.Chapter {
	slug := decodeString(arg)
	if slug == "" {
		return nil
	}
	// Chapter listing is keyed by numeric series id; read it from the detail
	// endpoint first.
	body, ok := getJSON(apiURL + "/series/" + url.PathEscape(slug))
	if !ok {
		return nil
	}
	var s series
	if json.Unmarshal([]byte(body), &s) != nil || s.ID == 0 {
		return nil
	}

	var out []types.Chapter
	// ponytail: source asks for 1000 chapters per page; kept, paid chapters
	// (price != 0) are skipped as source default hides them.
	for page := 1; page <= 100; page++ {
		q := url.Values{}
		q.Set("page", strconv.Itoa(page))
		q.Set("perPage", "1000")
		q.Set("series_id", strconv.Itoa(s.ID))
		b, ok := getJSON(apiURL + "/chapter/query?" + q.Encode())
		if !ok {
			break
		}
		var resp chapterResp
		if json.Unmarshal([]byte(b), &resp) != nil {
			break
		}
		for _, c := range resp.Data {
			if c.Slug == "" || c.Price != 0 {
				continue
			}
			out = append(out, types.Chapter{
				ID:         slug + ":" + c.Slug,
				MangaID:    slug,
				Title:      chapterTitle(c),
				ChapterNum: chapterNum(c.Name),
				ReleasedAt: parseTime(c.Created),
				URL:        siteURL + "/series/" + url.PathEscape(slug) + "/" + url.PathEscape(c.Slug),
			})
		}
		if resp.Meta == nil || resp.Meta.CurrentPage >= resp.Meta.LastPage {
			break
		}
	}
	return out
}

func chapterTitle(c chapter) string {
	name := strings.TrimSpace(c.Name)
	if c.Title != nil && strings.TrimSpace(*c.Title) != "" {
		name += " - " + strings.TrimSpace(*c.Title)
	}
	return name
}

// ─── pages ──────────────────────────────────────────────────────────────────

func pagesImpl(arg string) []types.Page {
	slug, chSlug, ok := strings.Cut(decodeString(arg), ":")
	if !ok || slug == "" || chSlug == "" {
		return nil
	}
	body, ok := getJSON(apiURL + "/chapter/" + url.PathEscape(slug) + "/" + url.PathEscape(chSlug))
	if !ok {
		return nil
	}
	var resp pageResp
	if json.Unmarshal([]byte(body), &resp) != nil || resp.Chapter.Data == nil {
		return nil
	}
	headers := map[string]string{"Referer": siteURL}
	out := make([]types.Page, 0, len(resp.Chapter.Data.Images))
	for _, img := range resp.Chapter.Data.Images {
		if img == "" {
			continue
		}
		out = append(out, types.Page{Index: len(out), URL: absImage(img), Headers: headers})
	}
	return out
}

// ─── genres ─────────────────────────────────────────────────────────────────

func genresImpl() []genre {
	body, ok := getJSON(apiURL + "/tags")
	if !ok {
		return nil
	}
	var tags []tag
	if json.Unmarshal([]byte(body), &tags) != nil {
		return nil
	}
	out := make([]genre, 0, len(tags))
	for _, t := range tags {
		if t.Name != "" {
			out = append(out, genre{Name: t.Name, Slug: strconv.Itoa(t.ID)})
		}
	}
	return out
}

// ─── helpers ────────────────────────────────────────────────────────────────

var (
	rePara  = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	reTag   = regexp.MustCompile(`(?s)<[^>]*>`)
	reSpace = regexp.MustCompile(`\s+`)
	reNum   = regexp.MustCompile(`\d+(?:\.\d+)?`)
)

// absImage turns a possibly-relative asset path into an absolute URL. The
// API's cdnUrl is apiURL with coverPath "" (HeanCms defaults).
func absImage(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return apiURL + "/" + strings.TrimPrefix(raw, "/")
}

func decodeString(arg string) string {
	var s string
	_ = json.Unmarshal([]byte(arg), &s)
	return strings.TrimSpace(s)
}

func plainText(s string) string {
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(reSpace.ReplaceAllString(s, " "))
}

// description mirrors HeanCms.toSManga: join <p> texts with a blank line,
// falling back to the whole fragment's text.
func description(s string) string {
	if s == "" {
		return ""
	}
	paras := rePara.FindAllStringSubmatch(s, -1)
	parts := make([]string, 0, len(paras))
	for _, p := range paras {
		if t := plainText(p[1]); t != "" {
			parts = append(parts, t)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n\n")
	}
	return plainText(s)
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

// parseTime accepts the Laravel-style ISO-8601 timestamps the API returns
// (fractional seconds and offsets included).
func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.999999Z07:00", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
