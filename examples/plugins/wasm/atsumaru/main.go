//go:build wasip1

package main

import (
	"encoding/json"
	"strconv"
	"strings"

	"goisekai/pkg/types"
)

var (
	base     string // plugin.json param "site_url"
	cdn      string // plugin.json param "cdn"
	jsonHead = `{"accept":"application/json"}`
)

func init() {
	base = param("site_url")
	cdn = param("cdn")
}

// imageUrl resolves a relative image path against the CDN.
func imageUrl(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.HasPrefix(path, "//") {
		return "https:" + path
	}
	path = strings.TrimPrefix(path, "/static/")
	path = strings.TrimPrefix(path, "static/")
	return cdn + "/static/" + path
}

func getJSON(url string) (map[string]any, bool) {
	headers := `{"accept":"application/json"}`
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

// ─── ABI: search ─────────────────────────────────────────────────────────────

func search(arg string) []types.Manga {
	var f types.SearchFilter
	if json.Unmarshal([]byte(arg), &f) != nil {
		return nil
	}
	if f.Page < 1 {
		f.Page = 1
	}
	q := f.Query
	if q == "" {
		q = "*"
	}

	filter := "hidden:!=true && views:>0 && medium:!=[`Novel`]"
	if len(f.Genres) > 0 {
		var ids []string
		for _, g := range f.Genres {
			ids = append(ids, "`"+g+"`")
		}
		filter += " && genreIds:=[" + strings.Join(ids, ",") + "]"
	}

	url := base + "/collections/manga/documents/search?q=" + urlEncode(q) +
		"&filter_by=" + urlEncode(filter) +
		"&query_by=" + urlEncode("title,englishTitle,otherNames,authors") +
		"&page=" + itoa(f.Page) +
		"&per_page=20"

	data, ok := getJSON(url)
	out := []types.Manga{}
	if !ok {
		return out
	}
	hitsVal := data["hits"]
	if hits, isArr := hitsVal.([]any); isArr {
		for _, h := range hits {
			hit, ok := h.(map[string]any)
			if !ok {
				continue
			}
			doc, _ := hit["document"].(map[string]any)
			if doc == nil {
				doc = hit // older Typesense shape: document at top level
			}
			id, _ := doc["id"].(string)
			title, _ := doc["title"].(string)
			if title == "" {
				title, _ = doc["englishTitle"].(string)
			}
			m := types.Manga{ID: id, Title: title}
			if p, _ := doc["poster"].(string); p != "" {
				m.CoverURL = imageUrl(p)
			} else if p, _ := doc["posterSmall"].(string); p != "" {
				m.CoverURL = imageUrl(p)
			}
			out = append(out, m)
		}
	}
	return out
}

// ─── ABI: get_manga_detail ───────────────────────────────────────────────────

func detail(arg string) types.Manga {
	id := decodeString(arg)
	m := types.Manga{ID: id}
	data, ok := getJSON(base + "/api/manga/page?id=" + urlEncode(id))
	if !ok {
		return m
	}
	page, isMap := data["mangaPage"].(map[string]any)
	if !isMap {
		return m
	}
	title, _ := page["title"].(string)
	m.Title = title

	status, _ := page["status"].(string)
	m.Status = hostString("text.normalize_status", status)

	synopsis, _ := page["synopsis"].(string)
	m.Description = hostString("text.strip_html", synopsis)

	if poster, isMap := page["poster"].(map[string]any); isMap {
		if img, _ := poster["image"].(string); img != "" {
			m.CoverURL = imageUrl(img)
		} else {
			if id2, _ := poster["id"].(string); id2 != "" {
				m.CoverURL = imageUrl(id2)
			}
		}
	} else if poster, isStr := page["poster"].(string); isStr {
		m.CoverURL = imageUrl(poster)
	}

	if authors, isArr := page["authors"].([]any); isArr {
		var names []string
		for _, a := range authors {
			if am, isM := a.(map[string]any); isM {
				if n, _ := am["name"].(string); n != "" {
					names = append(names, n)
				}
			}
		}
		m.Author = strings.Join(names, ", ")
	}

	if genres, isArr := page["genres"].([]any); isArr {
		for _, g := range genres {
			if gm, isM := g.(map[string]any); isM {
				if n, _ := gm["name"].(string); n != "" {
					m.Genres = append(m.Genres, n)
				}
			}
		}
	}
	return m
}

// ─── ABI: get_chapter_list ──────────────────────────────────────────────────

func chapters(arg string) []types.Chapter {
	// Sandbox passes bare string, Lua version passes JSON-encoded string
	var id string
	if err := json.Unmarshal([]byte(arg), &id); err != nil {
		id = strings.TrimSpace(arg)
	}
	// The API endpoint wants the id directly (no JSON encoding)
	data, ok := getJSON(base + "/api/manga/allChapters?mangaId=" + urlEncode(id))
	if !ok {
		return nil
	}
	out := []types.Chapter{}
	if raw, isArr := data["chapters"].([]any); isArr {
		for _, c := range raw {
			cm, isM := c.(map[string]any)
			if !isM {
				continue
			}
			cid, _ := cm["id"].(string)
			num, _ := cm["number"].(string)
			title, _ := cm["title"].(string)
			if title == "" {
				title = "Chapter " + num
			}
			ch := types.Chapter{
				ID:         id + ":" + cid,
				MangaID:    id,
				Title:      strings.TrimSpace(title),
				ChapterNum: chapterNum(strings.TrimSpace(title)),
			}
			// createdAt is epoch millis; strconv keeps it exact on 32-bit wasm.
			if ca, isNum := cm["createdAt"].(float64); isNum && ca > 0 {
				if iso := dateToISO(strconv.FormatInt(int64(ca), 10)); iso != "" {
					ch.ReleasedAt = parseTime(iso)
				}
			}
			out = append(out, ch)
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
		logError("atsumaru: invalid chapter id " + composite)
		return nil
	}
	mangaID, chapterID := parts[0], parts[1]
	data, ok := getJSON(base + "/api/read/chapter?mangaId=" + urlEncode(mangaID) + "&chapterId=" + urlEncode(chapterID))
	if !ok {
		return nil
	}
	read, isMap := data["readChapter"].(map[string]any)
	if !isMap {
		return nil
	}
	out := []types.Page{}
	if list, isArr := read["pages"].([]any); isArr {
		for _, p := range list {
			pm, isM := p.(map[string]any)
			if !isM {
				continue
			}
			img, _ := pm["image"].(string)
			if img != "" {
				out = append(out, types.Page{Index: len(out), URL: imageUrl(img)})
			}
		}
	}
	return out
}

// ─── get_genres ──────────────────────────────────────────────────────────────

var genresTable = [][2]string{
	{"Action", "39"},
	{"Adventure", "37"},
	{"Comedy", "6"},
	{"Drama", "31"},
	{"Fantasy", "36"},
	{"Historical", "45"},
	{"Horror", "44"},
	{"Martial Arts", "29"},
	{"Mystery", "32"},
	{"Psychological", "18"},
	{"Romance", "9"},
	{"Sci-Fi", "1"},
	{"Slice of Life", "7"},
	{"Supernatural", "22"},
	{"Thriller", "19"},
	{"Tragedy", "5"},
}

func genres() []map[string]string {
	out := make([]map[string]string, 0, len(genresTable))
	for _, g := range genresTable {
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
