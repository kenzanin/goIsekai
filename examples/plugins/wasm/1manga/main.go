//go:build wasip1

// 1Manga source plugin for goIsekai — a Go WASM reactor
// (TinyGo wasi, -buildmode=c-shared), port of
// examples/plugins/lua/1manga/main.lua (archive reference).
//
// Site: https://1manga.co — MangaHub GraphQL API at api.mghcdn.com, source id
// mn03. The API answers 404 without Origin/Referer on the POST; the mhub_access
// key is only needed by the chapter (pages) resolver and the encrypted
// page-list decryption.
//
// Every site fact is read from this plugin's own plugin.json through
// host.plugin.param — nothing is hardcoded. Only the protocol-level constants
// (the key-mint trick, the GraphQL string escape, the enc:v1 envelope layout)
// live in the source, because those are logic, not config.

package main

import (
	"encoding/json"
	"strings"
	"time"
	"unsafe"

	"goisekai/pkg/types"
)

var (
	graphQLURL string // plugin.json param "api"
	siteURL    string // plugin.json param "site_url"
	sourceID   string // plugin.json param "source_id" (MangaHub source, "mn03")
	imgCDN     string // plugin.json param "img_cdn"
	thumbCDN   string // plugin.json param "thumb_cdn"

	// keyCounter walks the key-mint URL through chapter numbers that exist.
	// The Lua original randomises it; a counter serves the same purpose
	// without pulling in RNG.
	keyCounter int

	// cachedKey holds the mhub_access key, dropped whenever the API refuses.
	cachedKey string
	// gqlError holds the last refusal message, so callers surface it instead
	// of a silent empty list.
	gqlError string
)

// ─── ABI memory ─────────────────────────────────────────────────────────────
// malloc/free/allocCopy live in abi_std.go / abi_tinygo.go: TinyGo's runtime
// already exports malloc/free, so that pair is selected by build tag.

//go:wasmimport env host_call
func hostCallRaw(ptr, length uint32) uint64

// readString reads a string straight out of linear memory. The host allocates
// the input buffer through the module's malloc and writes it there, so the
// bytes are always live at ptr — and with TinyGo's malloc that pointer is not
// in any map of ours, which is why this reads memory instead of a table.
func readString(ptr, n uint32) string {
	if n == 0 || ptr == 0 {
		return ""
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), n))
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

// param reads one of this plugin's own plugin.json params. The host serves it
// through the shared surface, so the value travels with the plugin folder.
func param(name string) string {
	var s string
	if json.Unmarshal([]byte(hostCall("plugin.param", name)), &s) != nil {
		return ""
	}
	return s
}

// hostString is a kindStr1/kindStr1Err helper result: a JSON string literal,
// or "" when the call failed ({...} envelope or null).
func hostString(fn string, args ...string) string {
	raw := hostCall(fn, args...)
	if raw == "" || raw == "null" || raw[0] == '{' {
		return ""
	}
	var s string
	if json.Unmarshal([]byte(raw), &s) != nil {
		return ""
	}
	return s
}

// httpResp is the hostnet response object the plain http.get/http.post forms
// hand back (a transport failure answers {status:0, body:"..."}, which is how
// a refusal is told from a 4xx).
type httpResp struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

func httpGet(url, headersJSON string) (httpResp, bool) {
	var r httpResp
	if json.Unmarshal([]byte(hostCall("http.get", url, headersJSON)), &r) != nil {
		return httpResp{}, false
	}
	return r, true
}

func httpPost(url, body, headersJSON string) (httpResp, bool) {
	var r httpResp
	if json.Unmarshal([]byte(hostCall("http.post", url, body, headersJSON)), &r) != nil {
		return httpResp{}, false
	}
	return r, true
}

// chapterNum and dateToISO are the shared pure surface helpers
// (host_surface.go kindFloat1 / kindStr1), reachable from wasm through the
// same host_call.
func chapterNum(s string) float64 {
	var f float64
	if json.Unmarshal([]byte(hostCall("text.chapter_num", s)), &f) != nil {
		return 0
	}
	return f
}

// parseTime turns the host-normalized ISO string back into the time.Time the
// Chapter struct carries; an unparseable date leaves the zero value, which the
// host stores as NULL on purpose.
func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func logInfo(msg string)  { hostCall("log.info", msg) }
func logWarn(msg string)  { hostCall("log.warn", msg) }
func logError(msg string) { hostCall("log.error", msg) }

func init() {
	graphQLURL = param("api")
	siteURL = param("site_url")
	sourceID = param("source_id")
	imgCDN = param("img_cdn")
	thumbCDN = param("thumb_cdn")
}

// ─── helpers ────────────────────────────────────────────────────────────────

// A bare GET only hands back the key this session/IP already exhausted, so
// every chapter comes back rate limited. Presenting a cookie value the server
// has never issued makes it mint a genuinely new key, and ?reloadKey=1 is what
// triggers the re-issue. Each key so obtained carries four chapters of quota,
// and an explicit Cookie header overrides whatever the host cookie jar holds.
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
	// host.regex.find returns the first capture group as a plain string.
	return hostString("regex.find", cookie, `mhub_access=([^;]+)`)
}

// escapeGQL escapes a string for embedding in a GraphQL string literal.
func escapeGQL(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

// encryptedPageList decrypts the authoritative page list. MangaHub encrypts it
// and hands the key out separately:
//
//	pages = "enc:v1:<keyId>:<iv>:<tag>:<ciphertext>"   (all base64url)
//	key   = GET /api/chapter-crypto -> {keyId, key, expiresAt}
//	plain = AES-256-GCM(key, iv, ciphertext+tag) -> {"p":"<prefix>","i":["1a.jpg",...]}
//
// Decryption runs in the host (crypto.aes_gcm_decrypt), so the plugin carries
// no crypto. The key and the message rotate independently, so a keyId
// mismatch is retried once with a fresh key rather than treated as a failure.
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
		// This endpoint authenticates on the cookie, not the x-mhub-access
		// header the GraphQL call uses: the header alone answers 403.
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
			// The key is served per session, so a stale cookie yields a key the
			// message was not encrypted under. Drop it and ask again once.
			cachedKey = ""
			mat, why = material()
		}
	}
	if mat == nil {
		logError("1manga: chapter-crypto unavailable: " + why)
		return nil, "upstream_auth_failed"
	}
	if kid, _ := mat["keyId"].(string); kid != keyID {
		// The key ids are logged for diagnosis but deliberately not returned to
		// the reader: the user-facing text is the host's.
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

// graphqlQuery runs a GraphQL query, dropping the cached key and retrying once
// when the API refuses. MangaHub reports both expired keys and its "API rate
// limit excessed" refusal as HTTP 200 with an errors array and a null payload,
// so the status code alone cannot tell a good answer from a refused one. The
// message is kept in gqlError so callers can surface it.
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

// drill walks a chain of object keys out of a JSON document and returns the
// still-encoded raw text at the leaf, so callers can unmarshal into their own
// typed struct without the intermediate maps.
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

// numStr renders the chapter-number value the API sends (a JSON number or a
// string) in the shortest decimal form, matching how the Lua original used
// tostring() on it inside the GraphQL query and the id.
func numStr(v any) string {
	switch x := v.(type) {
	case float64:
		if x == float64(int(x)) {
			return itoa(int(x))
		}
		return strings.TrimRight(strings.TrimRight(
			strings.TrimRight(fmtSprintfF(x), "0"), "."), "")
	case string:
		return x
	}
	return ""
}

// fmtSprintfF is the stdlib shortest-round-trip float format, isolated so the
// file's only fmt usage is one obvious call (kept out of the hot paths).
func fmtSprintfF(f float64) string {
	return strings.TrimSuffix(jsonFloatShortest(f), "")
}

// jsonFloatShortest delegates to encoding/json's shortest-round-trip formatter
// so the exact number the API sent (23.1, 44) comes back verbatim.
func jsonFloatShortest(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// itoa avoids fmt (which drags its whole machinery into the wasm).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func decodeString(arg string) string {
	var s string
	_ = json.Unmarshal([]byte(arg), &s)
	return strings.TrimSpace(s)
}

// ─── ABI: search_manga ──────────────────────────────────────────────────────
// arg: {"query":"...","page":1}
func search(arg string) []types.Manga {
	var f types.SearchFilter
	if json.Unmarshal([]byte(arg), &f) != nil {
		return nil
	}
	if f.Page < 1 {
		f.Page = 1
	}
	// GraphQL takes one genre name per call; multi-genre means intersecting
	// result sets, which one query cannot express. First genre wins.
	genre := "all"
	if len(f.Genres) > 0 {
		genre = f.Genres[0]
	}
	logInfo("1manga search: q=" + f.Query + " page=" + itoa(f.Page))

	offset := (f.Page - 1) * 30
	gql := `{search(x: ` + sourceID + `, q: "` + escapeGQL(f.Query) +
		`", genre: "` + escapeGQL(genre) + `", mod: POPULAR, offset: ` +
		itoa(offset) + `) {rows {title, slug, image}}}`

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

// ─── ABI: get_manga_detail ─────────────────────────────────────────────────
// arg: '"slug"'. A failed lookup still returns an object: an empty result would
// encode as null and cannot decode into a detail record.
func detail(arg string) types.Manga {
	slug := decodeString(arg)
	m := types.Manga{ID: slug}
	if slug == "" {
		return m
	}
	logInfo("1manga detail: slug=" + slug)

	gql := `{manga(x: ` + sourceID + `, slug: "` + escapeGQL(slug) +
		`") {title, slug, status, image, author, artist, genres, description, alternativeTitle}}`
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
	// status may arrive as an enum string or a numeric code; the host wants a
	// string either way.
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
	// genres arrives as one comma-separated string; the ABI wants a list.
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

// ─── ABI: get_chapter_list ─────────────────────────────────────────────────
// arg: '"slug"'
func chapters(arg string) []types.Chapter {
	slug := decodeString(arg)
	if slug == "" {
		return nil
	}
	logInfo("1manga chapters: slug=" + slug)

	gql := `{manga(x: ` + sourceID + `, slug: "` + escapeGQL(slug) + `") {chapters {number, title, date}}}`
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
		// An absent title arrives as an empty string, not nil.
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
		// MangaHub sends epoch seconds, epoch millis or a date string; the host
		// normalizes all three. An unparseable date leaves the field at zero,
		// which the host stores as NULL on purpose.
		c.ReleasedAt = parseTime(hostString("text.date_to_iso", numStr(ch.Date)))
		out = append(out, c)
	}
	// GraphQL returns ascending; the ABI wants newest-first.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].ChapterNum > out[i].ChapterNum {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	logInfo("1manga chapters: found " + itoa(len(out)) + " chapters for " + slug)
	return out
}

// ─── ABI: get_page_list ────────────────────────────────────────────────────
// arg: '"slug:chapter-NUMBER"'
func pages(arg string) []types.Page {
	chapterID := decodeString(arg)
	// Split on the first ":chapter-" — the slug cannot contain one, the tail
	// can (chapter 23.1 vs volume 5).
	i := strings.Index(chapterID, ":chapter-")
	if i < 0 {
		return nil
	}
	slug, number := chapterID[:i], chapterID[i+len(":chapter-"):]
	logInfo("1manga pages: slug=" + slug + " ch=" + number)

	gql := `{chapter(x: ` + sourceID + `, slug: "` + escapeGQL(slug) +
		`", number: ` + number + `) {pages, mangaID, number}}`
	data, ok := graphqlQuery(gql)
	var chapter map[string]any
	if ok {
		if raw := drill(data, "data", "chapter"); raw != nil {
			_ = json.Unmarshal([]byte(*raw), &chapter)
		}
	}
	if chapter == nil {
		// A refusal (rate limit, expired key) must fail loudly: returning an
		// empty list renders a blank chapter with no explanation.
		if gqlError != "" {
			logError("1manga: graphql refused: " + gqlError)
		}
		return nil
	}

	// Authoritative page list: the API hands back the exact file names, which
	// the CDN probe below cannot do - this manga's chapters name their pages
	// "1a.jpg 2b.jpg 3b.jpg ..." (chapter 59 is the same), so both the
	// extension probe and the binary search fail on them. The payload arrives
	// in two shapes:
	//   "enc:v1:<keyId>:<iv>:<tag>:<ciphertext>" -> AES-256-GCM, key from
	//   GET /api/chapter-crypto (host crypto.aes_gcm_decrypt), or
	//   {"p":"<prefix>/","i":["1a.jpg",...]}    -> plaintext JSON (what the
	//   API answers today for letter-suffixed pages, verified in 1manga.har).
	// Both name every file exactly; neither falls through to the probe.
	pagesField, _ := chapter["pages"].(string)
	if strings.HasPrefix(pagesField, "enc:v1:") {
		out, why := encryptedPageList(pagesField, slug, number)
		if out != nil {
			logInfo("1manga pages: found " + itoa(len(out)) + " pages for " + chapterID + " (decrypted)")
			return out
		}
		// Fail loudly instead of probing. An encrypted pages field means this
		// chapter is served from the authoritative list, so the CDN pattern is
		// already known to be wrong for it: probing then produced a list that
		// looked plausible and silently dropped pages (observed on chapter 58,
		// which reported 18 pages it had only guessed at). An empty chapter with
		// a stated cause beats a wrong chapter that reads as complete.
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
			logInfo("1manga pages: found " + itoa(len(out)) + " pages for " + chapterID + " (plaintext)")
			return out
		}
		logWarn("1manga: unrecognised pages payload, falling back to probe: " +
			pagesField[:min(len(pagesField), 60)])
	}

	// Fallback for a missing or unparseable pages field: find the page count by
	// probing (page N exists with 200, N+1 404s). Binary search keeps it to
	// ~9 cheap requests; the list lands in the chapters_pages cache afterward,
	// so the cost is paid once per chapter. This assumes contiguous page
	// numbers, which is exactly what the authoritative paths above exist to
	// avoid. The extension varies per chapter (older chapters are .jpg, newer
	// ones .jpeg), so probe candidates once on page 1 instead of hardcoding one.
	referer := `{"Referer":"` + siteURL + `/"}`
	ext := ""
	for _, c := range []string{"jpg", "jpeg", "png", "webp"} {
		r, ok := httpGet(imgCDN+slug+"/"+number+"/1."+c, referer)
		if ok && r.Status == 200 {
			ext = c
			break
		}
	}
	if ext == "" {
		// No usable page 1 on the CDN: this chapter's files are letter-suffixed
		// (1a.jpg ...) and the authoritative payload was missing or unreadable,
		// so the probe is useless. Fail loudly: an empty chapter with a stated
		// cause beats a silently blank one.
		logError("1manga pages: no usable page 1 for " + chapterID + " (probe fallback, no pages field to parse)")
		return nil
	}
	pageExists := func(i int) bool {
		r, ok := httpGet(imgCDN+slug+"/"+number+"/"+itoa(i)+"."+ext, referer)
		return ok && r.Status == 200
	}
	lo, hi := 1, 200
	for lo < hi {
		mid := (lo + hi + 1 + 1) / 2 // ceil((lo+hi+1)/2), the binary-search pivot
		if pageExists(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	out := make([]types.Page, 0, lo)
	for i := 1; i <= lo; i++ {
		// No per-page headers: the field must be left out, because the host
		// decodes an empty table as a null map.
		out = append(out, types.Page{Index: i - 1, URL: imgCDN + slug + "/" + number + "/" + itoa(i) + "." + ext})
	}
	logInfo("1manga pages: found " + itoa(len(out)) + " pages for " + chapterID)
	return out
}

// ─── get_genres (optional export) ──────────────────────────────────────────
// MangaHub GraphQL takes genre names directly (genre: "Action"); the list
// below mirrors the site's genre facet.
var genresTable = [][2]string{
	{"Action", "Action"}, {"Adventure", "Adventure"}, {"Comedy", "Comedy"},
	{"Cooking", "Cooking"}, {"Doujinshi", "Doujinshi"}, {"Drama", "Drama"},
	{"Fantasy", "Fantasy"}, {"Gender bender", "Gender Bender"},
	{"Harem", "Harem"}, {"Historical", "Historical"}, {"Horror", "Horror"},
	{"Isekai", "Isekai"}, {"Josei", "Josei"}, {"Martial arts", "Martial Arts"},
	{"Mecha", "Mecha"}, {"Mystery", "Mystery"}, {"One shot", "One Shot"},
	{"Psychological", "Psychological"}, {"Romance", "Romance"},
	{"School life", "School Life"}, {"Sci-fi", "Sci Fi"}, {"Seinen", "Seinen"},
	{"Shoujo", "Shoujo"}, {"Shounen", "Shounen"}, {"Slice of life", "Slice of Life"},
	{"Sports", "Sports"}, {"Supernatural", "Supernatural"}, {"Tragedy", "Tragedy"},
	{"Webtoons", "Webtoons"},
}

type genre struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func genres() []genre {
	out := make([]genre, 0, len(genresTable))
	for _, g := range genresTable {
		out = append(out, genre{Name: g[0], Slug: g[1]})
	}
	return out
}

// ─── ABI exports ────────────────────────────────────────────────────────────

// main is required by the Go linker for package main; -buildmode=c-shared
// makes the module a reactor (initialised through _initialize, main never
// runs).
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
func GetGenres(ptr, n uint32) uint64 { return reply(genres()) }
