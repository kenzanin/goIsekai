//go:build wasip1

// Command mangadex-plugin is a goIsekai manga source plugin that fetches
// manga, chapters, and pages from the MangaDex public API (api.mangadex.org).
//
// It implements the Extism PDK ABI: exports read input via pdk.Input() and
// write output via pdk.Output(), returning int32 (0 = success).
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/extism/go-pdk"

	"goisekai/pkg/types"
)
const (
	apiURL = "https://api.mangadex.org"
	cdnURL = "https://uploads.mangadex.org"
	lang   = "en"
)

// ---------------------------------------------------------------------------
// Extism host function — imported from the Extism kernel
// ---------------------------------------------------------------------------

//go:wasmimport extism:host/user host_http_request
func hostHTTPRequest(offset uint64) uint64

// fetch performs an HTTP GET through the host proxy and decodes the response.
func doFetch(requestURL string) (*types.HTTPResponse, error) {
	b, err := json.Marshal(types.HTTPRequest{Method: "GET", URL: requestURL})
	if err != nil {
		return nil, err
	}
	mem := pdk.AllocateString(string(b))
	defer mem.Free()

	respOffset := hostHTTPRequest(mem.Offset())
	if respOffset == 0 {
		return nil, fmt.Errorf("host function failed")
	}
	respMem := pdk.FindMemory(respOffset)
	defer respMem.Free()

	var resp types.HTTPResponse
	if err := json.Unmarshal(respMem.ReadBytes(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ---------------------------------------------------------------------------
// MangaDex API response DTOs
// ---------------------------------------------------------------------------

type mangaListResp struct {
	Result   string      `json:"result"`
	Response string      `json:"response"`
	Data     []mangaData `json:"data"`
	Limit    int         `json:"limit"`
	Offset   int         `json:"offset"`
	Total    int         `json:"total"`
}

type singleMangaResp struct {
	Result   string    `json:"result"`
	Response string    `json:"response"`
	Data     mangaData `json:"data"`
}

type mangaData struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Attributes    mangaAttrs     `json:"attributes"`
	Relationships []relationship `json:"relationships"`
}

type mangaAttrs struct {
	Title       map[string]string   `json:"title"`
	AltTitles   []map[string]string `json:"altTitles"`
	Description map[string]string   `json:"description"`
	Status      string              `json:"status"`
	Tags        []mangaTag          `json:"tags"`
}

type mangaTag struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name  map[string]string `json:"name"`
		Group string            `json:"group"`
	} `json:"attributes"`
}

type relationship struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes *struct {
		FileName string `json:"fileName"`
		Name     string `json:"name"`
	} `json:"attributes"`
}

type chapterListResp struct {
	Result string        `json:"result"`
	Data   []chapterData `json:"data"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
	Total  int           `json:"total"`
}

type chapterData struct {
	ID         string          `json:"id"`
	Attributes chapterAttrs    `json:"attributes"`
	Relationships []relationship `json:"relationships"`
}

type chapterAttrs struct {
	Chapter      string `json:"chapter"`
	Volume       string `json:"volume"`
	Title        string `json:"title"`
	TranslatedLanguage string `json:"translatedLanguage"`
	PublishAt    string `json:"publishAt"`
	ExternalURL  string `json:"externalURL"`
}

type atHomeResp struct {
	BaseURL  string `json:"baseUrl"`
	Chapter  struct {
		Hash string   `json:"hash"`
		Data []string `json:"data"`
	} `json:"chapter"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func defaultHeaders() map[string]string {
	return map[string]string{
		"Referer": cdnURL + "/",
	}
}

func firstTitle(attrs mangaAttrs) string {
	if t, ok := attrs.Title[lang]; ok && t != "" {
		return t
	}
	for _, at := range attrs.AltTitles {
		if t, ok := at[lang]; ok && t != "" {
			return t
		}
	}
	for _, at := range attrs.AltTitles {
		for _, t := range at {
			if t != "" {
				return t
			}
		}
	}
	for _, t := range attrs.Title {
		if t != "" {
			return t
		}
	}
	return ""
}

func coverURL(md mangaData) string {
	for _, r := range md.Relationships {
		if r.Type == "cover_art" && r.Attributes != nil {
			return cdnURL + "/covers/" + md.ID + "/" + r.Attributes.FileName + ".256.jpg"
		}
	}
	return ""
}

func toManga(md mangaData) types.Manga {
	return types.Manga{
		ID:       md.ID,
		Title:    firstTitle(md.Attributes),
		CoverURL: coverURL(md),
	}
}

func parseFloat64(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func contentRatingQuery(q url.Values) {
	q.Add("contentRating[]", "safe")
	q.Add("contentRating[]", "suggestive")
	q.Add("contentRating[]", "erotica")
}

// ---------------------------------------------------------------------------
// Extism ABI exports — pdk.Input() / pdk.Output(), return int32
// ---------------------------------------------------------------------------

//go:wasmexport contract_version
func contractVersion() int32 {
	pdk.OutputString("1")
	return 0
}

//go:wasmexport Init
func Init() int32 {
	b, _ := json.Marshal(types.PluginMeta{ThumbRatio: 0.703})
	pdk.Output(b)
	return 0
}

// Search returns manga matching a query. Empty query returns popular manga.
//
//go:wasmexport Search
func Search() int32 {
	var f types.SearchFilter
	_ = json.Unmarshal(pdk.Input(), &f)

	if f.Page < 1 {
		f.Page = 1
	}

	q := url.Values{}
	q.Set("limit", "24")
	q.Set("offset", strconv.Itoa((f.Page-1)*24))
	q.Add("includes[]", "cover_art")
	q.Add("includes[]", "author")
	q.Add("availableTranslatedLanguage[]", lang)
	if strings.TrimSpace(f.Query) == "" {
		q.Add("order[followedCount]", "desc")
	} else {
		q.Add("order[relevance]", "desc")
	}
	contentRatingQuery(q)

	if title := strings.TrimSpace(f.Query); title != "" {
		q.Set("title", title)
	}

	resp, err := doFetch(apiURL + "/manga?" + q.Encode())
	if err != nil || resp.Status < 200 || resp.Status >= 300 {
		b, _ := json.Marshal([]types.Manga{})
		pdk.Output(b)
		return 0
	}

	var list mangaListResp
	if err := json.Unmarshal([]byte(resp.Body), &list); err != nil {
		b, _ := json.Marshal([]types.Manga{})
		pdk.Output(b)
		return 0
	}

	results := make([]types.Manga, 0, len(list.Data))
	for _, md := range list.Data {
		results = append(results, toManga(md))
	}
	b, _ := json.Marshal(results)
	pdk.Output(b)
	return 0
}

// GetMangaDetail returns full metadata for a single manga by ID.
//
//go:wasmexport GetMangaDetail
func GetMangaDetail() int32 {
	var mangaID string
	_ = json.Unmarshal(pdk.Input(), &mangaID)
	if mangaID == "" {
		b, _ := json.Marshal(types.Manga{})
		pdk.Output(b)
		return 0
	}

	q := url.Values{}
	q.Add("includes[]", "cover_art")
	q.Add("includes[]", "author")
	q.Add("includes[]", "artist")

	resp, err := doFetch(apiURL + "/manga/" + mangaID + "?" + q.Encode())
	if err != nil || resp.Status < 200 || resp.Status >= 300 {
		b, _ := json.Marshal(types.Manga{})
		pdk.Output(b)
		return 0
	}

	var single singleMangaResp
	if err := json.Unmarshal([]byte(resp.Body), &single); err != nil {
		b, _ := json.Marshal(types.Manga{})
		pdk.Output(b)
		return 0
	}
	b, _ := json.Marshal(toManga(single.Data))
	pdk.Output(b)
	return 0
}

// GetChapterList returns all chapters for a manga (paginated feed).
//
//go:wasmexport GetChapterList
func GetChapterList() int32 {
	var mangaID string
	_ = json.Unmarshal(pdk.Input(), &mangaID)
	if mangaID == "" {
		b, _ := json.Marshal([]types.Chapter{})
		pdk.Output(b)
		return 0
	}

	var all []types.Chapter
	offset := 0
	for {
		q := url.Values{}
		q.Set("limit", "500")
		q.Set("offset", strconv.Itoa(offset))
		q.Add("translatedLanguage[]", lang)
		q.Add("order[volume]", "asc")
		q.Add("order[chapter]", "asc")
		q.Add("includes[]", "scanlation_group")
		q.Set("includeEmptyPages", "0")
		contentRatingQuery(q)

		resp, err := doFetch(apiURL + "/manga/" + mangaID + "/feed?" + q.Encode())
		if err != nil || resp.Status < 200 || resp.Status >= 300 {
			break
		}

		var list chapterListResp
		if err := json.Unmarshal([]byte(resp.Body), &list); err != nil {
			break
		}

		for _, cd := range list.Data {
			if cd.Attributes.ExternalURL != "" {
				continue
			}

			chNum := parseFloat64(cd.Attributes.Chapter)
			if math.IsNaN(chNum) {
				chNum = 0
			}

			volNum := parseFloat64(cd.Attributes.Volume)
			if math.IsNaN(volNum) {
				volNum = 0
			}

			chTitle := "Chapter " + cd.Attributes.Chapter
			if cd.Attributes.Title != "" {
				chTitle = cd.Attributes.Title
			}
			if cd.Attributes.Chapter == "" && cd.Attributes.Title == "" {
				chTitle = "Oneshot"
			}

			all = append(all, types.Chapter{
				ID:         cd.ID,
				MangaID:    mangaID,
				Title:      chTitle,
				ChapterNum: chNum,
				VolumeNum:  volNum,
				ReleasedAt: parseTime(cd.Attributes.PublishAt),
				URL:        "https://mangadex.org/chapter/" + cd.ID,
			})
		}

		offset += list.Limit
		if offset >= list.Total {
			break
		}
	}
	b, _ := json.Marshal(all)
	pdk.Output(b)
	return 0
}

// GetPageList returns image URLs for a single chapter via the MangaDex at-home API.
//
//go:wasmexport GetPageList
func GetPageList() int32 {
	var chapterID string
	_ = json.Unmarshal(pdk.Input(), &chapterID)
	if chapterID == "" {
		b, _ := json.Marshal([]types.Page{})
		pdk.Output(b)
		return 0
	}

	resp, err := doFetch(apiURL + "/at-home/server/" + chapterID)
	if err != nil || resp.Status < 200 || resp.Status >= 300 {
		b, _ := json.Marshal([]types.Page{})
		pdk.Output(b)
		return 0
	}

	var ah atHomeResp
	if err := json.Unmarshal([]byte(resp.Body), &ah); err != nil {
		b, _ := json.Marshal([]types.Page{})
		pdk.Output(b)
		return 0
	}

	pages := make([]types.Page, 0, len(ah.Chapter.Data))
	for i, filename := range ah.Chapter.Data {
		pages = append(pages, types.Page{
			Index:   i,
			URL:     ah.BaseURL + "/data/" + ah.Chapter.Hash + "/" + filename,
			Headers: defaultHeaders(),
		})
	}
	b, _ := json.Marshal(pages)
	pdk.Output(b)
	return 0
}

func main() {}
