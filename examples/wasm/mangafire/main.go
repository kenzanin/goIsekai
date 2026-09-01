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
	neturl "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/extism/go-pdk"

	"mangafire-plugin/types"
	"mangafire-plugin/vrf"
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

// vrfURL builds the full API URL: path + url-encoded params + vrf param.
func vrfURL(apiPath string, params map[string]string) string {
	sig := vrf.Sign(apiPath, params)
	u := apiBase + apiPath
	if len(params) == 0 {
		return u + "?vrf=" + sig
	}
	q := neturl.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return u + "?" + q.Encode() + "&vrf=" + sig
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// stripHTML removes HTML tags from a synopsis HTML string.
func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	re := strings.NewReplacer(
		"&quot;", "\"", "&#039;", "'", "&amp;", "&", "&lt;", "<", "&gt;", ">",
	)
	s = re.Replace(s)
	var b strings.Builder
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(c)
		}
	}
	return strings.TrimSpace(b.String())
}

// sanitizeTitle strips MangaFire's HTML-escaped title entities.
func sanitizeTitle(s string) string {
	s = strings.ReplaceAll(s, "&#039;", "'")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return strings.TrimSpace(s)
}

// ---------------------------------------------------------------------------
// Extism ABI exports — pdk.Input() / pdk.Output(), return int32
// ---------------------------------------------------------------------------

const thumbRatio = 0.677 // 264×390 posters (MangaFire default)

//export contract_version
func contractVersion() int32 {
	pdk.OutputString("1")
	return 0
}

//export Init
func Init() int32 {
	b, _ := json.Marshal(types.PluginMeta{
		ThumbRatio:     thumbRatio,
		SearchPageSize: 50,
	})
	pdk.Output(b)
	return 0
}

// Search — arg = JSON SearchFilter{Query, Page}. Returns []Manga.

//export Search
func Search() int32 {
	var f types.SearchFilter
	_ = json.Unmarshal(pdk.Input(), &f)

	params := map[string]string{
		"keyword": f.Query,
		"limit":   "50",
		"page":    strconv.Itoa(f.Page),
	}
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
	var resp struct {
		Items []item `json:"items"`
	}
	u := vrfURL("/titles", params)
	if err := fetchJSON(u, &resp); err != nil {
		b, _ := json.Marshal([]types.Manga{})
		pdk.Output(b)
		return 0
	}
	out := make([]types.Manga, 0, len(resp.Items))
	for _, it := range resp.Items {
		out = append(out, types.Manga{
			ID:       it.HID,
			Title:    sanitizeTitle(it.Title),
			CoverURL: it.Poster.Medium,
		})
	}
	b, _ := json.Marshal(out)
	pdk.Output(b)
	return 0
}

// GetMangaDetail — arg = JSON mangaID (the hid). Returns Manga.

//export GetMangaDetail
func GetMangaDetail() int32 {
	var hid string
	_ = json.Unmarshal(pdk.Input(), &hid)

	var response struct {
		Data struct {
			HID     string `json:"hid"`
			Title   string `json:"title"`
			Summary string `json:"synopsisHtml"`
			Status  string `json:"status"`
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
		Status:      manga.Status,
	})
	pdk.Output(b)
	return 0
}

// GetChapterList — arg = JSON mangaID (the hid). Returns []Chapter.
// Fetches up to 3 pages (200/page, ~600 chapters). Reversed to oldest-first.

//export GetChapterList
func GetChapterList() int32 {
	var hid string
	_ = json.Unmarshal(pdk.Input(), &hid)

	chapters := []types.Chapter{}
	page := 1
	for {
		params := map[string]string{
			"language": "en",
			"limit":    "200",
			"order":    "desc",
			"page":     strconv.Itoa(page),
			"sort":     "number",
		}
		var resp struct {
			Items []struct {
				ID        int     `json:"id"`
				Number    float64 `json:"number"`
				Name      string  `json:"name"`
				CreatedAt int64   `json:"created_at"`
			} `json:"items"`
			Meta struct {
				LastPage int  `json:"last_page"`
				HasNext  bool `json:"has_next"`
			} `json:"meta"`
		}
		u := vrfURL("/titles/"+hid+"/chapters", params)
		if err := fetchJSON(u, &resp); err != nil {
			break
		}
		for _, c := range resp.Items {
			chapters = append(chapters, types.Chapter{
				ID:         strconv.Itoa(c.ID),
				MangaID:    hid,
				ChapterNum: c.Number,
				Title:      c.Name,
				ReleasedAt: time.Unix(c.CreatedAt, 0).UTC(),
			})
		}
		if page >= resp.Meta.LastPage || !resp.Meta.HasNext || page >= 3 {
			break
		}
		page++
	}
	// Descending page order => newest first; reverse to oldest-first.
	for i, j := 0, len(chapters)-1; i < j; i, j = i+1, j-1 {
		chapters[i], chapters[j] = chapters[j], chapters[i]
	}
	b, _ := json.Marshal(chapters)
	pdk.Output(b)
	return 0
}

// GetPageList — arg = JSON chapterID (the numeric string from a chapter id).
// Returns []Page; each page carries the Referer required by the image CDN.

//export GetPageList
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
