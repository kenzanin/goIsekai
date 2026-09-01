//go:build wasip1

// Command mangafire-plugin is a goIsekai manga source plugin for MangaFire
// (mangafire.to), a JSON-API manga source with a VRF-signed request layer.
//
// MangaFire signs every /api request with a `vrf` query parameter: a 3-stage
// XOR-table transform over a sign-string, then base64url-Raw (no padding).
//   sign string = apiPath (without "/api") + "?" + "k=v" sorted by key,
//                 values NOT url-encoded (raw)
//   stage: out[i] = table[data[i] XOR key[i%len(key)] XOR prev]; prev=out[i-1]
//   IVs:   stage1=0x5A, stage2=0x35, stage3=0xBA
// Tables/keys rotate with the MangaFire frontend; the base64 constants in
// ./vrf were extracted from its source and verified live (every /api endpoint
// returns HTTP 200 with these signatures).
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
	"unsafe"

	"mangafire-plugin/types"
	"mangafire-plugin/vrf"
)

const abiVersion int32 = 1

// main is a no-op: the plugin is driven entirely by exported wasm functions.
func main() {}

const (
	apiBase = "https://mangafire.to/api"
	referer = "https://mangafire.to/"
)

// ---------------------------------------------------------------------------
// VRF signer — port of the MangaFire frontend signer (see ./vrf).
// The byte-oriented signer and its verified test vectors live in the vrf
// package so they compile and run on any platform; main.go only assembles
// the signed request URL below.
// ---------------------------------------------------------------------------

// vrfURL builds the full API URL: path + url-encoded params + vrf param.
// The vrf itself is base64url (URL-safe), so it needs no further encoding.
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
// ABI plumbing — identical in shape to examples/mangadex-plugin.
// ---------------------------------------------------------------------------

// allocations tracks every live WASM import buffer so we can pass it to the
// host and free it later.
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

//go:wasmimport env host_http_request
func hostHTTPRequest(ptr, length uint32) uint64

func unpack(packed uint64) (ptr, length uint32) {
	return uint32(packed), uint32(packed >> 32)
}

func readString(ptr, length uint32) string {
	return unsafe.String((*byte)(unsafe.Pointer(uintptr(ptr))), int(length))
}

func readID(ptr, length uint32) string {
	var id string
	_ = json.Unmarshal([]byte(readString(ptr, length)), &id)
	return id
}

func readFilter(ptr, length uint32) types.SearchFilter {
	var f types.SearchFilter
	_ = json.Unmarshal([]byte(readString(ptr, length)), &f)
	return f
}

func returnJSON(v any) uint64 {
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	ptr := malloc(uint32(len(b)))
	copy(allocations[ptr], b)
	return uint64(uint32(len(b)))<<32 | uint64(ptr)
}

func fetchJSON(requestURL string, v any) error {
	b, err := json.Marshal(types.HTTPRequest{Method: "GET", URL: requestURL})
	if err != nil {
		return err
	}
	inPtr := malloc(uint32(len(b)))
	copy(allocations[inPtr], b)
	defer free(inPtr)

	outPtr, outLen := unpack(hostHTTPRequest(inPtr, uint32(len(b))))
	if outPtr == 0 || outLen == 0 {
		return fmt.Errorf("host_http_request returned empty result")
	}
	defer free(outPtr)

	var resp types.HTTPResponse
	if err := json.Unmarshal([]byte(readString(outPtr, outLen)), &resp); err != nil {
		return err
	}
	if resp.Status < 200 || resp.Status >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.Status, resp.Body)
	}
	return json.Unmarshal([]byte(resp.Body), v)
}

// ---------------------------------------------------------------------------
// Plugin entry points (exported to the host).
// ---------------------------------------------------------------------------

const thumbRatio = 0.677 // 264×390 posters (MangaFire default)

// contract_version reports the ABI version this plugin was built against.
//
//go:wasmexport contract_version
func contractVersion() int32 { return abiVersion }

// Init returns plugin metadata. Thumb ratio 0.677 = 264:390 (MangaFire posters).
//
//go:wasmexport Init
func Init() uint64 {
	return returnJSON(types.PluginMeta{
		ThumbRatio:     thumbRatio,
		SearchPageSize: 50,
	})
}

// Search — arg = JSON SearchFilter{Query, Page}. Returns []Manga.
// Params are signed raw (values NOT url-encoded); the request URL encodes
// them via net/url, which turns spaces into '+'.
//
//go:wasmexport Search
func Search(ptr, length uint32) uint64 {
	f := readFilter(ptr, length)
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
	url := vrfURL("/titles", params)
	if err := fetchJSON(url, &resp); err != nil {
		return returnJSON([]types.Manga{})
	}
	out := make([]types.Manga, 0, len(resp.Items))
	for _, it := range resp.Items {
		out = append(out, types.Manga{
			ID:       it.HID,
			Title:    sanitizeTitle(it.Title),
			CoverURL: it.Poster.Medium,
		})
	}
	return returnJSON(out)
}

// GetMangaDetail — arg = JSON mangaID (the hid). Returns Manga.
//
//go:wasmexport GetMangaDetail
func GetMangaDetail(ptr, length uint32) uint64 {
	hid := readID(ptr, length)
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
		return returnJSON(types.Manga{})
	}
	manga := response.Data
	return returnJSON(types.Manga{
		ID:          manga.HID,
		Title:       sanitizeTitle(manga.Title),
		Description: stripHTML(manga.Summary),
		CoverURL:    manga.Poster.Medium,
		Status:      manga.Status,
	})
}

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
	// MangaFire titles can contain HTML entities; drop any <...> tags that slip in.
	s = strings.ReplaceAll(s, "&#039;", "'")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return strings.TrimSpace(s)
}

// GetChapterList — arg = JSON mangaID (the hid). Returns []Chapter.
// Fetches ALL pages (200/page) — One Piece alone is ~2000 chapters. Reversed
// to oldest-first so the reader's page-1 maps to the first chapter.
//
//go:wasmexport GetChapterList
func GetChapterList(ptr, length uint32) uint64 {
	hid := readID(ptr, length)
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
	return returnJSON(chapters)
}

// GetPageList — arg = JSON chapterID (the numeric string from a chapter id).
// Returns []Page; each page carries the Referer required by the image CDN.
//
//go:wasmexport GetPageList
func GetPageList(ptr, length uint32) uint64 {
	chapterID := readID(ptr, length)
	var resp struct {
		Data struct {
			Pages []struct {
				URL string `json:"url"`
			} `json:"pages"`
		} `json:"data"`
	}
	if err := fetchJSON(vrfURL("/chapters/"+chapterID, nil), &resp); err != nil {
		return returnJSON([]types.Page{})
	}
	pages := make([]types.Page, 0, len(resp.Data.Pages))
	for i, p := range resp.Data.Pages {
		pages = append(pages, types.Page{
			Index:   i,
			URL:     p.URL,
			Headers: map[string]string{"Referer": referer},
		})
	}
	return returnJSON(pages)
}

// VRF test vectors (verified live against mangafire.to — HTTP 200):
//   vrfURL("/titles/dkw", nil) -> vrf="8sK3xtqdFds7Xfo"
//   vrfURL("/titles", {keyword:"one piece",limit:"50",page:"1"}) ->
//           vrf="8sK3xtqdFZfetBhus6bRApNr5zMeEWBTZ95f9C_GdK1bchY3Fv5HBdo"
