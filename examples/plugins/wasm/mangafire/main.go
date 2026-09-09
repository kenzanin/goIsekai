//go:build wasip1

// Command mangafire-plugin is a goIsekai manga source plugin for MangaFire
// (mangafire.to), a JSON-API manga source with a VRF-signed request layer.
//
// It implements the Extism PDK ABI: exports read input via pdk.Input() and
// write output via pdk.Output(), returning int32 (0 = success).
//
// MangaFire signs every /api request with a `vrf` query parameter: a 3-stage
// XOR-table transform over a sign-string, then base64url-Raw (no padding).
//
// Image CDN (e.g. img-r1.2xstorage.com) returns 403 without a Referer, so
// page objects carry Headers={"Referer":"https://mangafire.to/"}; the host
// /image endpoint forwards that Referer upstream.

package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/extism/go-pdk"

	"mangafire-plugin/types"
)

const (
	apiBase = "https://mangafire.to/api"
	referer = "https://mangafire.to/"
)

// ---------------------------------------------------------------------------
// Extism host function — imported from the Extism kernel
// ---------------------------------------------------------------------------

//go:wasmimport extism:host/user host_http_request
func hostHTTPRequest(offset uint64) uint64

// fetchJSON performs an HTTP GET through the host proxy and decodes the response.
func fetchJSON(requestURL string, v any) error {
	b, err := json.Marshal(types.HTTPRequest{Method: "GET", URL: requestURL})
	if err != nil {
		return err
	}
	mem := pdk.AllocateString(string(b))
	defer mem.Free()

	respOffset := hostHTTPRequest(mem.Offset())
	if respOffset == 0 {
		return fmt.Errorf("host_http_request returned empty result")
	}
	respMem := pdk.FindMemory(respOffset)
	defer respMem.Free()

	var resp types.HTTPResponse
	if err := json.Unmarshal(respMem.ReadBytes(), &resp); err != nil {
		return err
	}
	if resp.Status < 200 || resp.Status >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.Status, resp.Body)
	}
	return json.Unmarshal([]byte(resp.Body), v)
}

// ---------------------------------------------------------------------------
// VRF signer — port of the MangaFire frontend signer (see ./vrf).
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Extism ABI exports — pdk.Input() / pdk.Output(), return int32
// ---------------------------------------------------------------------------

const thumbRatio = 0.677 // 264×390 posters (MangaFire default)

//go:wasmexport contract_version
func contractVersion() int32 {
	pdk.OutputString("1")
	return 0
}

//go:wasmexport Init
func Init() int32 {
	b, _ := json.Marshal(types.PluginMeta{
		Name:           "MangaFire",
		SiteURL:        "https://mangafire.to",
		Logo:           "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Ctext y='28' font-size='28'%3E🔥%3C/text%3E%3C/svg%3E",
		ThumbRatio:     thumbRatio,
		SearchPageSize: 50,
	})
	pdk.Output(b)
	return 0
}

// Search — returns ALL results across all upstream pages.
// The host handles host-side pagination via search_page_size.

//go:wasmexport Search
func Search() int32 {
	var f types.SearchFilter
	_ = json.Unmarshal(pdk.Input(), &f)

	type poster struct {
		Small  string `json:"small"`
		Medium string `json:"medium"`
		Large  string `json:"large"`
	}
	type item struct {
		HID    string `json:"hid"`
		Slug   string `json:"slug"`
		Title  string `json:"title"`
		Poster poster `json:"poster"`
	}
	var all []types.Manga
	page := 1
	for {
		params := map[string]string{
			"keyword": f.Query,
			"limit":   "50",
			"page":    strconv.Itoa(page),
		}
		var resp struct {
			Items []item `json:"items"`
			Meta  struct {
				LastPage int  `json:"last_page"`
				HasNext  bool `json:"has_next"`
			} `json:"meta"`
		}
		u := vrfURL("/titles", params)
		if err := fetchJSON(u, &resp); err != nil || len(resp.Items) == 0 {
			break
		}
		for _, it := range resp.Items {
			all = append(all, types.Manga{
				ID:       it.HID,
				Title:    sanitizeTitle(it.Title),
				CoverURL: it.Poster.Medium,
			})
		}
		if !resp.Meta.HasNext || page >= resp.Meta.LastPage {
			break
		}
		page++
	}
	b, _ := json.Marshal(all)
	pdk.Output(b)
	return 0
}

// GetMangaDetail — arg = JSON mangaID (the hid). Returns Manga.

//go:wasmexport GetMangaDetail
func GetMangaDetail() int32 {
	var hid string
	_ = json.Unmarshal(pdk.Input(), &hid)

	var response struct {
		Data struct {
			HID     string                  `json:"hid"`
			Title   string                  `json:"title"`
			Summary string                  `json:"synopsisHtml"`
			Status  string                  `json:"status"`
			Poster  struct{ Medium string } `json:"poster"`
		} `json:"data"`
	}
	if err := fetchJSON(vrfURL("/titles/"+hid, nil), &response); err != nil {
		b, _ := json.Marshal(types.Manga{})
		pdk.Output(b)
		return 0
	}
	manga := response.Data
	b, _ := json.Marshal(types.Manga{
		ID:          manga.HID,
		Title:       sanitizeTitle(manga.Title),
		Description: stripHTML(manga.Summary),
		CoverURL:    manga.Poster.Medium,
		Status:      normalizeStatus(manga.Status),
	})
	pdk.Output(b)
	return 0
}

// GetChapterList — arg = JSON mangaID (the hid). Returns []Chapter.
// Fetches up to 3 pages (200/page, ~600 chapters), newest-first.

// GetPageList — arg = JSON chapterID (the numeric string from a chapter id).
// Returns []Page; each page carries the Referer required by the image CDN.

//go:wasmexport GetPageList
func GetPageList() int32 {
	var chapterID string
	_ = json.Unmarshal(pdk.Input(), &chapterID)

	var resp struct {
		Data struct {
			Pages []struct {
				URL string `json:"url"`
			} `json:"pages"`
		} `json:"data"`
	}
	if err := fetchJSON(vrfURL("/chapters/"+chapterID, nil), &resp); err != nil {
		b, _ := json.Marshal([]types.Page{})
		pdk.Output(b)
		return 0
	}
	pages := make([]types.Page, 0, len(resp.Data.Pages))
	for i, p := range resp.Data.Pages {
		pages = append(pages, types.Page{
			Index:   i,
			URL:     p.URL,
			Headers: map[string]string{"Referer": referer},
		})
	}
	b, _ := json.Marshal(pages)
	pdk.Output(b)
	return 0
}

func main() {}
