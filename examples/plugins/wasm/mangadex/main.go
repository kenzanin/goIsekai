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
	"net/url"
	"strconv"
	"strings"

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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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
