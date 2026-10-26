//go:build wasip1

package main

import (
	"encoding/json"
	"strings"

	"goisekai/pkg/types"
)

var (
	base   string // plugin.json param "site_url"
	ua     string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
)

func init() {
	base = param("site_url")
}

// ─── helpers ────────────────────────────────────────────────────────────────

func bareSlug(mangaID string) string {
	for i := 0; i < len(mangaID); i++ {
		if mangaID[i] == '.' {
			return mangaID[:i]
		}
	}
	return mangaID
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// jsonGet fetches a URL through the host and parses the response as JSON.
func jsonGet(url string) (map[string]any, bool) {
	headers := `{"Accept":"application/json","User-Agent":"` + ua + `"}`
	r, ok := httpGet(url, headers)
	if !ok || r.Status != 200 {
		return nil, false
	}
	var m map[string]any
	if json.Unmarshal([]byte(r.Body), &m) != nil {
		return nil, false
	}
	return m, true
}

// htmlGet fetches a URL and returns its body.
func htmlGet(url string) string {
	headers := `{"User-Agent":"` + ua + `"}`
	r, ok := httpGet(url, headers)
	if !ok || r.Status != 200 {
		return ""
	}
	return r.Body
}

// firstCap extracts the first capture group of pattern in s, or "".
func firstCap(s, pattern string) string {
	res := hostCall("regex.find", s, pattern)
	if res == "null" || res == "" {
		return ""
	}
	var ranges []int
	if json.Unmarshal([]byte(res), &ranges) != nil || len(ranges) < 2 {
		return ""
	}
	return s[ranges[1]:ranges[2]]
}

// ─── ABI: search ─────────────────────────────────────────────────────────────

func search(arg string) []types.Manga {
	var f types.SearchFilter
	if json.Unmarshal([]byte(arg), &f) != nil {
		return nil
	}
	if f.Page < 1 {
		f.Page = 1
	}

	out := []types.Manga{}

	if len(f.Genres) > 0 {
		// Genre browse: /series?genres=<slug> renders HTML comic cards.
		url := base + "/series?genres=" + urlEncode(f.Genres[0])
		if f.Page > 1 {
			url = url + "&page=" + itoa(f.Page)
		}
		body := htmlGet(url)
		for _, card := range gAll(body, `<a[^>]*href="/series/[^/?"]+[^>]*>`) {
			href := firstCap(card, `href="/series/([^/?"]+)"`)
			title := firstCap(card, `alt="Cover of ([^"]+)"`)
			cover := firstCap(card, `data-src="([^"]+)"`)
			if href == "" {
				continue
			}
			out = append(out, types.Manga{ID: href, Title: unescape(title), CoverURL: trim(cover)})
		}
	} else if f.Query != "" {
		// Text search: /api/search?search=Q returns JSON.
		data, ok := jsonGet(base + "/api/search?search=" + urlEncode(f.Query))
		if !ok {
			return out
		}
		if comics, isArr := data["comics"].([]any); isArr {
			for _, c := range comics {
				cm, isMap := c.(map[string]any)
				if !isMap {
					continue
				}
				slugHash, _ := cm["slug_hash"].(string)
				if slugHash == "" {
					continue
				}
				out = append(out, types.Manga{
					ID:       slugHash,
					Title:    cm["title"].(string),
					CoverURL: cm["image"].(string),
				})
			}
		}
	}
	return out
}

// ─── ABI: get_manga_detail ───────────────────────────────────────────────────

func detail(arg string) types.Manga {
	mangaID := decodeString(arg)
	m := types.Manga{ID: mangaID}
	if mangaID == "" {
		return m
	}
	slug := bareSlug(mangaID)
	data, ok := jsonGet(base + "/get-chapter-list?slug=" + urlEncode(slug))
	if !ok {
		return m
	}
	return mangaFromChapterList(data, mangaID)
}

// mangaFromChapterList builds a Manga from a /get-chapter-list response.
// That endpoint does not carry the manga title or cover, so we also fetch
// /api/search?search=<slug> to enrich title + cover.
func mangaFromChapterList(data map[string]any, mangaID string) types.Manga {
	m := types.Manga{ID: mangaID}
	slug := bareSlug(mangaID)
	searchData, ok := jsonGet(base + "/api/search?search=" + urlEncode(slug))
	if ok {
		if comics, isArr := searchData["comics"].([]any); isArr && len(comics) > 0 {
			if cm, isM := comics[0].(map[string]any); isM {
				if title, _ := cm["title"].(string); title != "" {
					m.Title = title
				}
				if cover, _ := cm["image"].(string); cover != "" {
					m.CoverURL = cover
				}
			}
		}
	}
	return m
}

// ─── ABI: get_chapter_list ──────────────────────────────────────────────────

func chapters(arg string) []types.Chapter {
	mangaID := decodeString(arg)
	slug := bareSlug(mangaID)
	if mangaID == "" || slug == "" {
		return nil
	}

	data, ok := jsonGet(base + "/get-chapter-list?slug=" + urlEncode(slug))
	out := []types.Chapter{}
	if !ok {
		return out
	}
	if chaptersData, isMap := data["data"].([]any); isMap {
		// chapter_num missing on some rows; slug still ends in number.
		for _, ch := range chaptersData {
			cm, isM := ch.(map[string]any)
			if !isM {
				continue
			}
			chSlug, _ := cm["chapter_slug"].(string)
			chNum, _ := cm["chapter_num"].(float64)
			if chNum == 0 {
				chNum = chapterNum(chSlug)
			}
			if chNum <= 0 && chSlug == "" {
				continue
			}
			c := types.Chapter{
				ID:         mangaID + ":" + chSlug,
				MangaID:    mangaID,
				Title:      cm["chapter_name"].(string),
				ChapterNum: chNum,
				URL:        base + "/series/" + mangaID + "/" + chSlug,
			}
			if chNum == 0 {
				c.Title = "Chapter " + itoa(int(chNum))
			}
			updatedAt, _ := cm["updated_at"].(string)
			if iso := dateToISO(updatedAt); iso != "" {
				c.ReleasedAt = parseTime(iso)
			}
			out = append(out, c)
		}
	}
	// newest-first
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].ChapterNum > out[i].ChapterNum {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// ─── ABI: get_page_list ─────────────────────────────────────────────────────

func pages(arg string) []types.Page {
	composite := decodeString(arg)
	parts := strings.SplitN(composite, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		logError("mangabuddy: invalid chapter id " + composite)
		return nil
	}
	mangaID, chSlug := parts[0], parts[1]

	html := htmlGet(base + "/series/" + mangaID + "/" + chSlug)
	if html == "" {
		return nil
	}
	// Page images: data-src="https://{cdn}/{slug}/{ch}/{i}.webp"
	out := []types.Page{}
	seen := map[string]bool{}
	for _, img := range gAll(html, `data-src="(https://[^"/]+/[^"]*?/[\d.]+/\d+\.webp)"`) {
		if seen[img] {
			continue
		}
		seen[img] = true
		out = append(out, types.Page{Index: len(out), URL: img})
	}
	return out
}

// ─── get_genres ──────────────────────────────────────────────────────────────

var genreTable = [][2]string{
	{"Action", "action"}, {"Adult", "adult"}, {"Adventure", "adventure"},
	{"Boys Love", "boys-love"}, {"Comedy", "comedy"}, {"Demons", "demons"},
	{"Drama", "drama"}, {"Ecchi", "ecchi"}, {"Fantasy", "fantasy"},
	{"Full Color", "full-color"}, {"Girls Love", "girls-love"}, {"Harem", "harem"},
	{"Historical", "historical"}, {"Horror", "horror"}, {"Isekai", "isekai"},
	{"Josei", "josei"}, {"Magic", "magic"}, {"Manga", "manga"},
	{"Manhwa", "manhwa"}, {"Martial Arts", "martial-arts"}, {"Mature", "mature"},
	{"Mystery", "mystery"}, {"One shot", "one-shot"}, {"Others", "others"},
	{"Psychological", "psychological"}, {"Romance", "romance"}, {"School", "school"},
	{"School life", "school-life"}, {"Sci-Fi", "sci-fi"}, {"Seinen", "seinen"},
	{"Shoujo", "shoujo"}, {"Shounen", "shounen"}, {"Slice of Life", "slice-of-life"},
	{"Smut", "smut"}, {"Sports", "sports"}, {"Supernatural", "supernatural"},
	{"Thriller", "thriller"}, {"Tragedy", "tragedy"}, {"Webtoons", "webtoons"},
	{"Yaoi", "yaoi"},
}

func genres() []map[string]string {
	out := make([]map[string]string, 0, len(genreTable))
	for _, g := range genreTable {
		out = append(out, map[string]string{"name": g[0], "slug": g[1]})
	}
	return out
}

// ─── ABI exports ─────────────────────────────────────────────────────────────

func main() {}

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
func GetGenres() uint64 { return reply(genres()) }
