//go:build wasip1

package main

import (
	"encoding/json"
	"strings"

	"goisekai/pkg/types"
)

// ─── site facts (loaded via param in init) ────────────────────────────────────

var (
	graphQLURL string // plugin.json param "api"
	siteURL    string // plugin.json param "site_url"
	sourceID   string // plugin.json param "source_id" (MangaHub source, "mn03")
	imgCDN     string // plugin.json param "img_cdn"
	thumbCDN   string // plugin.json param "thumb_cdn"

	// keyCounter walks the key-mint URL through chapter numbers that exist.
	keyCounter int

	// cachedKey holds the mhub_access key, dropped whenever the API refuses.
	cachedKey string
	// gqlError holds the last refusal message, so callers surface it instead
	// of a silent empty list.
	gqlError string
)

func init() {
	graphQLURL = param("api")
	siteURL = param("site_url")
	sourceID = param("source_id")
	imgCDN = param("img_cdn")
	thumbCDN = param("thumb_cdn")
}

// ─── 1manga-specific helpers ──────────────────────────────────────────────────

func fetchAccessKey() string {
	keyCounter = 1000 + keyCounter%2000
	keyCounter++
	url := siteURL + "/chapter/martial-peak/chapter-" + itoa(keyCounter) + "?reloadKey=1"
	headers := `{"Referer":"` + siteURL + `/manga/martial-peak","Accept":"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8","Sec-Fetch-Dest":"document","Sec-Fetch-Mode":"navigate","Sec-Fetch-Site":"same-origin","Upgrade-Insecure-Requests":"1","Cookie":"mhub_access=0000000000000000000000000000cafe"}`
	r, ok := httpGet(url, headers)
	if !ok || r.Status == 0 {
		return ""
	}
	cookie := r.Headers["Set-Cookie"]
	if cookie == "" {
		return ""
	}
	return hostString("regex.find", cookie, `mhub_access=([^;]+)`)
}

func encryptedPageList(blob, slug, number string) ([]types.Page, string) {
	material := func() (map[string]any, string) {
		key := cachedKey
		if key == "" {
			key = fetchAccessKey()
			cachedKey = key
		}
		if key == "" {
			return nil, "upstream_auth_failed"
		}
		headers := `{"Cookie":"mhub_access=` + key + `","Accept":"application/json","Referer":"` + siteURL + `/chapter/` + slug + `/chapter-` + number + `"}`
		r, ok := httpGet(siteURL+"/api/chapter-crypto", headers)
		if !ok || r.Status != 200 {
			return nil, "upstream_auth_failed: crypto endpoint status " + itoa(r.Status)
		}
		var parsed map[string]any
		if json.Unmarshal([]byte(r.Body), &parsed) != nil {
			return nil, "upstream_auth_failed: crypto endpoint unparseable"
		}
		if parsed["key"] == nil || parsed["keyId"] == nil {
			return nil, "upstream_auth_failed: crypto endpoint returned no usable key"
		}
		return parsed, ""
	}

	parts := strings.Split(blob, ":")
	if len(parts) != 6 || parts[0] != "enc" || parts[1] != "v1" {
		logError("1manga: unrecognised pages envelope")
		return nil, "envelope_unrecognised"
	}
	keyID, iv, tag, ciphertext := parts[2], parts[3], parts[4], parts[5]

	mat, why := material()
	if mat != nil {
		if kid, _ := mat["keyId"].(string); kid != keyID {
			cachedKey = ""
			mat, why = material()
		}
	}
	if mat == nil {
		logError("1manga: chapter-crypto unavailable: " + why)
		return nil, "upstream_auth_failed"
	}
	if kid, _ := mat["keyId"].(string); kid != keyID {
		logError("1manga: keyId mismatch, blob " + keyID + " vs crypto " + kid)
		return nil, "decrypt_key_mismatch"
	}

	key, _ := mat["key"].(string)
	plain := hostString("crypto.aes_gcm_decrypt", key, iv, tag, ciphertext)
	if plain == "" {
		logError("1manga: pages decrypt failed")
		return nil, "decrypt_failed"
	}
	var decoded struct {
		P string   `json:"p"`
		I []string `json:"i"`
	}
	if json.Unmarshal([]byte(plain), &decoded) != nil || decoded.P == "" || len(decoded.I) == 0 {
		logError("1manga: unexpected pages plaintext")
		return nil, "decrypt_failed"
	}

	out := make([]types.Page, 0, len(decoded.I))
	for i, name := range decoded.I {
		out = append(out, types.Page{Index: i, URL: imgCDN + decoded.P + name})
	}
	return out, ""
}

func graphqlQuery(query string) (string, bool) {
	gqlError = ""
	for attempt := 0; attempt < 2; attempt++ {
		key := cachedKey
		if key == "" {
			key = fetchAccessKey()
			cachedKey = key
		}
		if key == "" {
			logError("1manga: no mhub_access key available")
			return "", false
		}
		body, _ := json.Marshal(map[string]string{"query": query})
		headers := `{"Content-Type":"application/json","Origin":"` + siteURL + `","Referer":"` + siteURL + `/","x-mhub-access":"` + key + `"}`
		r, ok := httpPost(graphQLURL, string(body), headers)
		if !ok || r.Status < 200 || r.Status >= 300 {
			logInfo("1manga: GraphQL status " + itoa(r.Status) + ", refreshing key")
			cachedKey = ""
			continue
		}
		var parsed map[string]any
		if json.Unmarshal([]byte(r.Body), &parsed) != nil {
			logError("1manga: unparseable GraphQL response")
			return "", false
		}
		if errs, isArr := parsed["errors"].([]any); isArr && len(errs) > 0 {
			gqlError = "GraphQL error"
			if msg, isObj := errs[0].(map[string]any); isObj {
				if s, isS := msg["message"].(string); isS {
					gqlError = s
				}
			}
		} else if parsed["data"] == nil {
			gqlError = "GraphQL response carried no data"
		} else {
			return r.Body, true
		}
		logWarn("1manga: GraphQL refused (" + gqlError + "), retrying with a fresh key")
		cachedKey = ""
	}
	return "", false
}

func drill(doc string, keys ...string) *string {
	cur := doc
	for _, k := range keys {
		var m map[string]json.RawMessage
		if json.Unmarshal([]byte(cur), &m) != nil {
			return nil
		}
		raw, ok := m[k]
		if !ok {
			return nil
		}
		s := string(raw)
		cur = s
	}
	return &cur
}

// ─── ABI: search ──────────────────────────────────────────────────────────────

func search(arg string) []types.Manga {
	var f types.SearchFilter
	if json.Unmarshal([]byte(arg), &f) != nil {
		return nil
	}
	if f.Page < 1 {
		f.Page = 1
	}
	genre := "all"
	if len(f.Genres) > 0 {
		genre = f.Genres[0]
	}
	logInfo("1manga search: q=" + f.Query + " page=" + itoa(f.Page))

	offset := (f.Page - 1) * 30
	gql := `{search(x: "` + sourceID + `", q: "` + escapeGQL(f.Query) + `", genre: "` + escapeGQL(genre) + `", mod: POPULAR, offset: ` + itoa(offset) + `) {rows {title, slug, image}}}`

	data, ok := graphqlQuery(gql)
	if !ok {
		return nil
	}
	var rows []struct {
		Title string `json:"title"`
		Slug  string `json:"slug"`
		Image string `json:"image"`
	}
	if r := drill(data, "data", "search", "rows"); r != nil {
		_ = json.Unmarshal([]byte(*r), &rows)
	}
	out := make([]types.Manga, 0, len(rows))
	for _, row := range rows {
		cover := row.Image
		if cover != "" && !strings.HasPrefix(cover, "http") {
			cover = thumbCDN + cover
		}
		out = append(out, types.Manga{ID: row.Slug, Title: row.Title, CoverURL: cover})
	}
	logInfo("1manga search: found " + itoa(len(out)) + " results q=" + f.Query)
	return out
}

// ─── ABI: get_manga_detail ────────────────────────────────────────────────────

func detail(arg string) types.Manga {
	slug := decodeString(arg)
	m := types.Manga{ID: slug}
	if slug == "" {
		return m
	}
	logInfo("1manga detail: slug=" + slug)

	gql := `{manga(x: sourceID, slug: "` + escapeGQL(slug) + `") {title, slug, status, image, author, artist, genres, description, alternativeTitle}}`
	data, ok := graphqlQuery(gql)
	if !ok {
		return m
	}
	var man map[string]any
	if raw := drill(data, "data", "manga"); raw != nil {
		_ = json.Unmarshal([]byte(*raw), &man)
	}
	if man == nil {
		return m
	}
	cover, _ := man["image"].(string)
	if cover != "" && !strings.HasPrefix(cover, "http") {
		cover = thumbCDN + cover
	}
	rawStatus, _ := man["status"].(string)
	if rawStatus == "" {
		if n, isNum := man["status"].(float64); isNum {
			rawStatus = itoa(int(n))
		}
	}
	m.Title, _ = man["title"].(string)
	if s, isS := man["slug"].(string); isS && s != "" {
		m.ID = s
	}
	m.CoverURL = cover
	m.Author, _ = man["author"].(string)
	m.Description, _ = man["description"].(string)
	if g, isStr := man["genres"].(string); isStr {
		for _, part := range strings.Split(g, ",") {
			if name := strings.TrimSpace(part); name != "" {
				m.Genres = append(m.Genres, name)
			}
		}
	} else if g, isArr := man["genres"].([]any); isArr {
		for _, x := range g {
			if s, isS := x.(string); isS {
				m.Genres = append(m.Genres, s)
			}
		}
	}
	m.Status = hostString("text.normalize_status", rawStatus)
	return m
}

// ─── ABI: get_chapter_list ────────────────────────────────────────────────────

func chapters(arg string) []types.Chapter {
	slug := decodeString(arg)
	if slug == "" {
		return nil
	}
	logInfo("1manga chapters: slug=" + slug)

	gql := `{manga(x: sourceID, slug: "` + escapeGQL(slug) + `") {chapters {number, title, date}}}`
	data, ok := graphqlQuery(gql)
	if !ok {
		return nil
	}
	var raw []struct {
		Number any    `json:"number"`
		Title  string `json:"title"`
		Date   any    `json:"date"`
	}
	if r := drill(data, "data", "manga", "chapters"); r != nil {
		_ = json.Unmarshal([]byte(*r), &raw)
	}

	out := make([]types.Chapter, 0, len(raw))
	for _, ch := range raw {
		num := numStr(ch.Number)
		title := ch.Title
		if title == "" {
			title = "Chapter " + num
		}
		c := types.Chapter{
			ID:         slug + ":chapter-" + num,
			MangaID:    slug,
			Title:      title,
			ChapterNum: chapterNum(num),
			URL:        siteURL + "/chapter/" + slug + "/chapter-" + num,
		}
		c.ReleasedAt = parseTime(hostString("text.date_to_iso", numStr(ch.Date)))
		out = append(out, c)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].ChapterNum > out[i].ChapterNum {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	logInfo("1manga chapters: found " + itoa(len(out)) + " chapters slug=" + slug)
	return out
}

// ─── ABI: get_page_list ───────────────────────────────────────────────────────

func pages(arg string) []types.Page {
	chapterID := decodeString(arg)
	i := strings.Index(chapterID, ":chapter-")
	if i <= 0 {
		return nil
	}
	slug, number := chapterID[:i], chapterID[i+len(":chapter-"):]
	logInfo("1manga pages: slug=" + slug + " ch=" + number)

	gql := `{chapter(x: sourceID, slug: "` + escapeGQL(slug) + `", number: ` + number + `) {pages, mangaID, number}}`
	data, ok := graphqlQuery(gql)
	if !ok {
		return nil
	}
	var chapter map[string]any
	if raw := drill(data, "data", "chapter"); raw != nil {
		_ = json.Unmarshal([]byte(*raw), &chapter)
	}
	if chapter == nil {
		return nil
	}

	pagesField, _ := chapter["pages"].(string)
	if strings.HasPrefix(pagesField, "enc:v1:") {
		out, why := encryptedPageList(pagesField, slug, number)
		if out != nil {
			logInfo("1manga pages: found " + itoa(len(out)) + " pages " + chapterID + " (decrypted)")
			return out
		}
		logError("1manga: encrypted page list failed: " + why)
		return nil
	}
	if pagesField != "" {
		var decoded struct {
			P string   `json:"p"`
			I []string `json:"i"`
		}
		if json.Unmarshal([]byte(pagesField), &decoded) == nil && decoded.P != "" && len(decoded.I) > 0 {
			out := make([]types.Page, 0, len(decoded.I))
			for i, name := range decoded.I {
				out = append(out, types.Page{Index: i, URL: imgCDN + decoded.P + name})
			}
			logInfo("1manga pages: found " + itoa(len(out)) + " pages " + chapterID + " (plaintext)")
			return out
		}
		logWarn("1manga: unrecognised pages payload, falling back probe: " + pagesField[:min(len(pagesField), 60)])
	}

	referer := `{"Referer":"` + siteURL + `"}`
	ext := ""
	for _, c := range []string{"jpg", "jpeg", "png", "webp"} {
		r, ok := httpGet(imgCDN+slug+"/"+number+"/1."+c, referer)
		if ok && r.Status == 200 {
			ext = c
			break
		}
	}
	if ext == "" {
		logError("1manga pages: no usable page 1 " + chapterID + " (probe fallback, no pages field parse)")
		return nil
	}
	pageExists := func(i int) bool {
		r, ok := httpGet(imgCDN+slug+"/"+number+"/"+itoa(i)+"."+ext, referer)
		return ok && r.Status == 200
	}
	lo, hi := 1, 200
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if pageExists(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	out := make([]types.Page, 0, lo)
	for i := 1; i <= lo; i++ {
		out = append(out, types.Page{Index: i - 1, URL: imgCDN + slug + "/" + number + "/" + itoa(i) + "." + ext})
	}
	logInfo("1manga pages: found " + itoa(len(out)) + " pages " + chapterID + " (probe)")
	return out
}

// ─── ABI: get_genres ──────────────────────────────────────────────────────────

var genresTable = [][2]string{
	{"Action", "Action"}, {"Adaptation", "Adaptation"}, {"Adult", "Adult"},
	{"Comedy", "Comedy"}, {"Comic", "Comic"}, {"Doujinshi", "Doujinshi"},
	{"Drama", "Drama"}, {"Ecchi", "Ecchi"}, {"Fantasy", "Fantasy"},
	{"Gender bender", "Gender Bender"}, {"Harem", "Harem"}, {"Historical", "Historical"},
	{"Horror", "Horror"}, {"Isekai", "Isekai"}, {"Josei", "Josei"},
	{"Martial arts", "Martial Arts"}, {"Mecha", "Mecha"}, {"Mystery", "Mystery"},
	{"One shot", "One Shot"}, {"Psychological", "Psychological"}, {"Romance", "Romance"},
	{"School life", "School Life"}, {"Sci-fi", "Sci Fi"}, {"Seinen", "Seinen"},
	{"Shoujo", "Shoujo"}, {"Shounen", "Shounen"}, {"Slice of life", "Slice of Life"},
	{"Sports", "Sports"}, {"Supernatural", "Supernatural"}, {"Tragedy", "Tragedy"},
	{"Webtoons", "Webtoons"},
}

func genres() []map[string]string {
	out := make([]map[string]string, 0, len(genresTable))
	for _, g := range genresTable {
		out = append(out, map[string]string{"name": g[0], "slug": g[1]})
	}
	return out
}

// ─── ABI exports ──────────────────────────────────────────────────────────────

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
