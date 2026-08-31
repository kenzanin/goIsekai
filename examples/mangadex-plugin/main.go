//go:build wasip1

// Command mangadex-plugin is a goIsekai manga source plugin that fetches
// manga, chapters, and pages from the MangaDex public API (api.mangadex.org).
//
// It implements the full host/plugin ABI (contract_version, malloc/free, and
// the four JSON-over-memory functions) and uses host_http_request for all
// network access.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"goisekai/pkg/types"
)

const abiVersion int32 = 1

const (
	apiURL = "https://api.mangadex.org"
	cdnURL = "https://uploads.mangadex.org"
	lang   = "en"
)

// ---------------------------------------------------------------------------
// ABI plumbing — identical to examples/dummy-plugin
// ---------------------------------------------------------------------------

// allocations tracks every live WASM allocation so we can free() them later.
var allocations = map[uint32][]byte{}

//go:wasmexport malloc
func malloc(size uint32) uint32 {
	b := make([]byte, size)
	ptr := uint32(uintptr(unsafe.Pointer(unsafe.SliceData(b))))
	allocations[ptr] = b
	return ptr
}

//go:wasmexport free
func free(ptr uint32) {
	delete(allocations, ptr)
}

//go:wasmexport contract_version
func contractVersion() int32 { return abiVersion }

// readString reinterprets a (ptr, len) pair in WASM linear memory as a Go string.
func readString(ptr, length uint32) string {
	return unsafe.String((*byte)(unsafe.Pointer(uintptr(ptr))), int(length))
}

// readID decodes a host-supplied id. The host JSON-encodes ids
// (json.Marshal(mangaID)), so this unmarshals the input string.
func readID(ptr, length uint32) string {
	var id string
	_ = json.Unmarshal([]byte(readString(ptr, length)), &id)
	return id
}

// returnJSON marshals v and returns the packed (ptr, len) i64 the ABI expects:
// low 32 bits = pointer, high 32 bits = length.
func returnJSON(v any) uint64 {
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	ptr := malloc(uint32(len(b)))
	copy(allocations[ptr], b)
	return uint64(uint32(len(b)))<<32 | uint64(ptr)
}

// hostHTTPRequest is imported from the host ("env" module). It performs all
// network access on the plugin's behalf: the host injects the default headers
// (User-Agent, Accept-Language, Referer), manages a per-plugin cookie jar, and
// enforces the no-direct-socket rule.
//
//go:wasmimport env host_http_request
func hostHTTPRequest(ptr, length uint32) uint64

// unpack splits the host's packed (ptr, len) i64 into its parts.
func unpack(packed uint64) (ptr, length uint32) {
	return uint32(packed), uint32(packed >> 32)
}

// fetch performs an HTTP GET through the host proxy and decodes the response.
func fetch(url string) (*types.HTTPResponse, error) {
	b, err := json.Marshal(types.HTTPRequest{Method: "GET", URL: url})
	if err != nil {
		return nil, err
	}
	inPtr := malloc(uint32(len(b)))
	copy(allocations[inPtr], b)
	defer free(inPtr)

	outPtr, outLen := unpack(hostHTTPRequest(inPtr, uint32(len(b))))
	if outPtr == 0 || outLen == 0 {
		return nil, fmt.Errorf("host_http_request returned empty result")
	}
	defer free(outPtr)

	var resp types.HTTPResponse
	if err := json.Unmarshal([]byte(readString(outPtr, outLen)), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// fetchWithHeaders performs an HTTP request with extra headers.
func fetchWithHeaders(method, requestURL string, headers map[string]string) (*types.HTTPResponse, error) {
	b, err := json.Marshal(types.HTTPRequest{Method: method, URL: requestURL, Headers: headers})
	if err != nil {
		return nil, err
	}
	inPtr := malloc(uint32(len(b)))
	copy(allocations[inPtr], b)
	defer free(inPtr)

	outPtr, outLen := unpack(hostHTTPRequest(inPtr, uint32(len(b))))
	if outPtr == 0 || outLen == 0 {
		return nil, fmt.Errorf("host_http_request returned empty result")
	}
	defer free(outPtr)

	var resp types.HTTPResponse
	if err := json.Unmarshal([]byte(readString(outPtr, outLen)), &resp); err != nil {
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
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Volume    string `json:"volume"`
		Chapter   string `json:"chapter"`
		Title     string `json:"title"`
		PublishAt string `json:"publishAt"`
		TranslatedLanguage string `json:"translatedLanguage"`
		ExternalURL string `json:"externalUrl"`
	} `json:"attributes"`
}

type atHomeResp struct {
	Result  string `json:"result"`
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash      string   `json:"hash"`
		Data      []string `json:"data"`
		DataSaver []string `json:"dataSaver"`
	} `json:"chapter"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// titleFrom picks the best title: English first, then first available.
func titleFrom(m map[string]string) string {
	if t, ok := m[lang]; ok && t != "" {
		return t
	}
	for _, t := range m {
		if t != "" {
			return t
		}
	}
	return ""
}

// bestTitle picks the best display title from title + altTitles.
func bestTitle(attrs mangaAttrs) string {
	if t := titleFrom(attrs.Title); t != "" {
		return t
	}
	for _, alt := range attrs.AltTitles {
		if t := titleFrom(alt); t != "" {
			return t
		}
	}
	return "Unknown"
}

// descFrom picks the English description.
func descFrom(attrs mangaAttrs) string {
	if d, ok := attrs.Description[lang]; ok && d != "" {
		return d
	}
	for _, d := range attrs.Description {
		if d != "" {
			return d
		}
	}
	return ""
}

// coverURL returns the MangaDex CDN cover URL (256px thumb).
func coverURL(md mangaData) string {
	for _, rel := range md.Relationships {
		if rel.Type == "cover_art" && rel.Attributes != nil && rel.Attributes.FileName != "" {
			return cdnURL + "/covers/" + md.ID + "/" + rel.Attributes.FileName + ".256.jpg"
		}
	}
	return ""
}

// authorName extracts the first author name from relationships.
func authorName(md mangaData) string {
	for _, rel := range md.Relationships {
		if rel.Type == "author" && rel.Attributes != nil && rel.Attributes.Name != "" {
			return rel.Attributes.Name
		}
	}
	return ""
}

// genreTags collects tag names where group == "genre".
func genreTags(attrs mangaAttrs) []string {
	var out []string
	for _, t := range attrs.Tags {
		if t.Attributes.Group == "genre" {
			if name := titleFrom(t.Attributes.Name); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// toManga converts a MangaDex mangaData to our types.Manga.
func toManga(md mangaData) types.Manga {
	return types.Manga{
		ID:          md.ID,
		Title:       bestTitle(md.Attributes),
		CoverURL:    coverURL(md),
		Author:      authorName(md),
		Description: descFrom(md.Attributes),
		Status:      md.Attributes.Status,
		Genres:      genreTags(md.Attributes),
	}
}

// parseFloat64 parses a string to float64, returning 0 on failure.
func parseFloat64(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// parseTime parses an ISO8601 timestamp, returning zero time on failure.
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// contentRatingQuery returns the contentRating URL params.
func contentRatingQuery(q url.Values) {
	q.Add("contentRating[]", "safe")
	q.Add("contentRating[]", "suggestive")
	q.Add("contentRating[]", "erotica")
}

// defaultHeaders returns the Referer header MangaDex pages expect.
func defaultHeaders() map[string]string {
	return map[string]string{"Referer": "https://mangadex.org/"}
}

// ---------------------------------------------------------------------------
// ABI exports
// ---------------------------------------------------------------------------

// Search returns manga matching a query. Empty query returns popular manga.
//
//go:wasmexport Search
func Search(ptr, length uint32) uint64 {
	var f types.SearchFilter
	_ = json.Unmarshal([]byte(readString(ptr, length)), &f)

	if f.Page < 1 {
		f.Page = 1
	}

	q := url.Values{}
	q.Set("limit", "24")
	q.Set("offset", strconv.Itoa((f.Page-1)*24))
	q.Add("includes[]", "cover_art")
	q.Add("includes[]", "author")
	q.Add("availableTranslatedLanguage[]", lang)
	// Empty query (browse popular) sorts by follows; a text query must sort by
	// relevance or MangaDex returns scattered popular titles instead of matches.
	if strings.TrimSpace(f.Query) == "" {
		q.Add("order[followedCount]", "desc")
	} else {
		q.Add("order[relevance]", "desc")
	}
	contentRatingQuery(q)

	if title := strings.TrimSpace(f.Query); title != "" {
		q.Set("title", title)
	}

	resp, err := fetch(apiURL + "/manga?" + q.Encode())
	if err != nil || resp.Status < 200 || resp.Status >= 300 {
		return returnJSON([]types.Manga{})
	}

	var list mangaListResp
	if err := json.Unmarshal([]byte(resp.Body), &list); err != nil {
		return returnJSON([]types.Manga{})
	}

	results := make([]types.Manga, 0, len(list.Data))
	for _, md := range list.Data {
		results = append(results, toManga(md))
	}
	return returnJSON(results)
}

// GetMangaDetail returns full metadata for a single manga by ID.
//
//go:wasmexport GetMangaDetail
func GetMangaDetail(ptr, length uint32) uint64 {
	mangaID := readID(ptr, length)
	if mangaID == "" {
		return returnJSON(types.Manga{})
	}

	q := url.Values{}
	q.Add("includes[]", "cover_art")
	q.Add("includes[]", "author")
	q.Add("includes[]", "artist")

	resp, err := fetch(apiURL + "/manga/" + mangaID + "?" + q.Encode())
	if err != nil || resp.Status < 200 || resp.Status >= 300 {
		return returnJSON(types.Manga{})
	}

	var single singleMangaResp
	if err := json.Unmarshal([]byte(resp.Body), &single); err != nil {
		return returnJSON(types.Manga{})
	}
	return returnJSON(toManga(single.Data))
}

// GetChapterList returns all chapters for a manga (paginated feed).
//
//go:wasmexport GetChapterList
func GetChapterList(ptr, length uint32) uint64 {
	mangaID := readID(ptr, length)
	if mangaID == "" {
		return returnJSON([]types.Chapter{})
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

		resp, err := fetch(apiURL + "/manga/" + mangaID + "/feed?" + q.Encode())
		if err != nil || resp.Status < 200 || resp.Status >= 300 {
			break
		}

		var list chapterListResp
		if err := json.Unmarshal([]byte(resp.Body), &list); err != nil {
			break
		}

		for _, cd := range list.Data {
			// Skip chapters hosted on external sites (e.g. MangaPlus).
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
	return returnJSON(all)
}

// GetPageList returns image URLs for a single chapter via the MangaDex at-home API.
//
//go:wasmexport GetPageList
func GetPageList(ptr, length uint32) uint64 {
	chapterID := readID(ptr, length)
	if chapterID == "" {
		return returnJSON([]types.Page{})
	}

	resp, err := fetch(apiURL + "/at-home/server/" + chapterID)
	if err != nil || resp.Status < 200 || resp.Status >= 300 {
		return returnJSON([]types.Page{})
	}

	var ah atHomeResp
	if err := json.Unmarshal([]byte(resp.Body), &ah); err != nil {
		return returnJSON([]types.Page{})
	}

	pages := make([]types.Page, 0, len(ah.Chapter.Data))
	for i, filename := range ah.Chapter.Data {
		pages = append(pages, types.Page{
			Index:   i,
			URL:     ah.BaseURL + "/data/" + ah.Chapter.Hash + "/" + filename,
			Headers: defaultHeaders(),
		})
	}
	return returnJSON(pages)
}

func main() {}
