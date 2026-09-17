// Bato1 (formerly Bato.to) plugin for goIsekai, running under the Yaegi
// interpreter. Only the Go standard library and the synthetic hostnet package
// are importable; all network access and HTML parsing goes through hostnet.
//
// Site notes gathered from live pages:
//   - Everything is server-rendered: search, detail, chapter list and reader
//     images all work without JavaScript.
//   - Search results are the cards under div.original.card-lg. The cover sits
//     inside an <a class="poster">, so the cover selector matches an anchor,
//     not a div.
//   - The detail page only embeds the 20 newest chapters. The complete chapter
//     list exists only on a reader page, in select#chapter-dropdown.
//   - Reader <img> tags keep the real URL in src for the first two pages and in
//     data-src for the rest, so both attribute sets have to be collected and
//     re-ordered by each image's data-number.
package main

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"hostnet"
)

const base = "https://bato1.com"

// Init returns the PluginMeta JSON (optional export).
func Init() string {
	// The site logo is a 512x100 wordmark, which would be squashed into the UI's
	// square slot, so the icon is an inline SVG tile instead.
	return `{"name":"Bato1","site_url":"https://bato1.com",` +
		`"logo":"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='7' fill='%232b2f3a'/%3E%3Ctext x='16' y='24' font-family='sans-serif' font-size='20' font-weight='bold' text-anchor='middle' fill='%23ffffff'%3EB%3C/text%3E%3C/svg%3E",` +
		`"thumb_ratio":0.703}`
}

// unquote strips the surrounding JSON quotes from a plain-string argument.
// Manga and chapter ids are site slugs, so no escape handling is needed.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// jsonValue returns the raw value of key in a flat JSON object. The filter is
// small and its shape is fixed, so it is read with string scanning rather than
// encoding/json (whose reflection path is fragile under Yaegi).
func jsonValue(s, key string) string {
	i := strings.Index(s, `"`+key+`":`)
	if i < 0 {
		return ""
	}
	v := strings.TrimSpace(s[i+len(`"`+key+`":`):])
	if !strings.HasPrefix(v, `"`) {
		if j := strings.IndexAny(v, ",}"); j >= 0 {
			return strings.TrimSpace(v[:j])
		}
		return v
	}
	var b strings.Builder
	for j := 1; j < len(v); j++ {
		if v[j] == '\\' && j+1 < len(v) {
			j++
			b.WriteByte(v[j])
			continue
		}
		if v[j] == '"' {
			break
		}
		b.WriteByte(v[j])
	}
	return b.String()
}

// jsonList renders a []string as a JSON array.
func jsonList(vals []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range vals {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Quote(v))
	}
	b.WriteByte(']')
	return b.String()
}

// chapterPath splits a reader URL into the segment after the slug (for example
// "chapter-84") and its chapter number. The segment is carried in the chapter
// id and rebuilt verbatim by GetPageList, so chapters whose URL does not follow
// the plain chapter-<n> shape still resolve to the page the site served.
func chapterPath(readerURL, slug string) (string, string, bool) {
	prefix := base + "/read/" + slug + "/"
	if !strings.HasPrefix(readerURL, prefix) {
		return "", "", false
	}
	path := strings.TrimPrefix(readerURL, prefix)
	num := strings.TrimPrefix(path, "chapter-")
	if path == num {
		return "", "", false
	}
	if _, err := strconv.ParseFloat(num, 64); err != nil {
		return "", "", false
	}
	return path, num, true
}

// collectPages merges one selector's number/src pair into pages. Both lists
// come from the same selector, so element i of each describes the same tag.
func collectPages(pages map[int]string, nums, srcs []string) {
	for i, numStr := range nums {
		if i >= len(srcs) {
			break
		}
		idx, err := strconv.Atoi(numStr)
		if err != nil {
			continue // the injected banner carries data-number="notice"
		}
		pages[idx] = srcs[i]
	}
}

// Search(arg) takes a JSON SearchFilter like {"query":"solo"} and returns a
// JSON array of Manga. The host slices search results into its own pages, so
// only the site's first result page is read and the filter's page is ignored,
// the same way the other source plugins behave.
func Search(arg string) (string, error) {
	requestURL := base + "/filter?keyword=" + url.QueryEscape(jsonValue(arg, "query"))
	body, err := hostnet.Get(requestURL)
	if err != nil {
		return "", err
	}
	doc, err := hostnet.Parse(body)
	if err != nil {
		return "", err
	}
	const card = "div.original.card-lg "
	titles, err := hostnet.FindListText(doc, card+".info > a")
	if err != nil {
		return "", err
	}
	hrefs, err := hostnet.FindListAttr(doc, card+".info > a", "href")
	if err != nil {
		return "", err
	}
	covers, err := hostnet.FindListAttr(doc, card+".poster img", "data-src")
	if err != nil {
		return "", err
	}
	// Each field comes from its own query, so a card shape the selectors do not
	// expect would misalign the lists; report that instead of emitting rows
	// whose titles and covers belong to different series.
	if len(hrefs) != len(titles) || len(covers) != len(titles) {
		return "", fmt.Errorf("bato1: search cards misaligned (titles=%d hrefs=%d covers=%d)",
			len(titles), len(hrefs), len(covers))
	}
	var out strings.Builder
	out.WriteByte('[')
	for i, title := range titles {
		id := strings.TrimPrefix(hrefs[i], base+"/manga/")
		if i > 0 {
			out.WriteByte(',')
		}
		fmt.Fprintf(&out, `{"id":%s,"title":%s,"cover_url":%s}`,
			strconv.Quote(id), strconv.Quote(title), strconv.Quote(covers[i]))
	}
	out.WriteByte(']')
	return out.String(), nil
}

// GetMangaDetail(arg) takes a JSON-encoded slug and returns a Manga JSON
// object. The synopsis is read from div.description because the JSON-LD
// description on the same page is truncated at a fixed length.
func GetMangaDetail(arg string) (string, error) {
	slug := unquote(arg)
	if slug == "" {
		return "", fmt.Errorf("bato1: empty manga id")
	}
	detailURL := base + "/manga/" + slug
	body, err := hostnet.Get(detailURL)
	if err != nil {
		return "", err
	}
	doc, err := hostnet.Parse(body)
	if err != nil {
		return "", err
	}
	title, err := hostnet.FindText(doc, `h1[itemprop="name"]`)
	if err != nil {
		return "", err
	}
	if title == "" {
		return "", fmt.Errorf("bato1: %s carries no title", detailURL)
	}
	cover, _ := hostnet.FindAttr(doc, "#manga-page div.poster img", "data-src")
	status, _ := hostnet.FindText(doc, "#manga-page div.info > p")
	description, _ := hostnet.FindText(doc, "div.description")
	authors, _ := hostnet.FindListText(doc, `#info-rating a[itemprop="author"]`)
	genres, _ := hostnet.FindListText(doc, `#info-rating a[href*="/genre/"]`)
	return fmt.Sprintf(`{"id":%s,"title":%s,"cover_url":%s,"author":%s,"description":%s,"status":%s,"genres":%s}`,
		strconv.Quote(slug), strconv.Quote(title), strconv.Quote(cover),
		strconv.Quote(strings.Join(authors, ", ")), strconv.Quote(description),
		strconv.Quote(status), jsonList(genres)), nil
}

// GetChapterList(arg) takes a JSON-encoded slug and returns a JSON array of
// Chapter. The detail page stops at 20 chapters, so its newest reader link is
// used as an entry point and the full list is read from that page's dropdown.
func GetChapterList(arg string) (string, error) {
	slug := unquote(arg)
	if slug == "" {
		return "", fmt.Errorf("bato1: empty manga id")
	}
	body, err := hostnet.Get(base + "/manga/" + slug)
	if err != nil {
		return "", err
	}
	doc, err := hostnet.Parse(body)
	if err != nil {
		return "", err
	}
	newest, err := hostnet.FindAttr(doc, `#chapter-list a[href*="/read/"]`, "href")
	if err != nil {
		return "", err
	}
	if newest == "" {
		return "[]", nil
	}
	body, err = hostnet.Get(newest)
	if err != nil {
		return "", err
	}
	doc, err = hostnet.Parse(body)
	if err != nil {
		return "", err
	}
	urls, err := hostnet.FindListAttr(doc, "#chapter-dropdown option", "value")
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteByte('[')
	first := true
	for _, readerURL := range urls {
		path, num, ok := chapterPath(readerURL, slug)
		if !ok {
			continue // the placeholder option has an empty value
		}
		if !first {
			out.WriteByte(',')
		}
		first = false
		fmt.Fprintf(&out, `{"id":%s,"manga_id":%s,"title":%s,"chapter_num":%s,"url":%s}`,
			strconv.Quote(slug+":"+path), strconv.Quote(slug),
			strconv.Quote("Chapter "+num), num, strconv.Quote(readerURL))
	}
	out.WriteByte(']')
	return out.String(), nil
}

// GetPageList(arg) takes a JSON-encoded "<slug>:<chapter path>" id, the same
// shape the chapter list emits, and returns a JSON array of Page.
func GetPageList(arg string) (string, error) {
	chapterID := unquote(arg)
	slug, path, ok := strings.Cut(chapterID, ":")
	if !ok || slug == "" || !strings.HasPrefix(path, "chapter-") {
		return "", fmt.Errorf("bato1: malformed chapter id %q", chapterID)
	}
	readerURL := base + "/read/" + slug + "/" + path
	body, err := hostnet.Get(readerURL)
	if err != nil {
		return "", err
	}
	doc, err := hostnet.Parse(body)
	if err != nil {
		return "", err
	}
	lazySrcs, err := hostnet.FindListAttr(doc, "div.page img[data-src]", "data-src")
	if err != nil {
		return "", err
	}
	lazyNums, err := hostnet.FindListAttr(doc, "div.page img[data-src]", "data-number")
	if err != nil {
		return "", err
	}
	eagerSrcs, err := hostnet.FindListAttr(doc, "div.page img:not([data-src])", "src")
	if err != nil {
		return "", err
	}
	eagerNums, err := hostnet.FindListAttr(doc, "div.page img:not([data-src])", "data-number")
	if err != nil {
		return "", err
	}
	pages := map[int]string{}
	collectPages(pages, lazyNums, lazySrcs)
	collectPages(pages, eagerNums, eagerSrcs)
	indexes := make([]int, 0, len(pages))
	for idx := range pages {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	var out strings.Builder
	out.WriteByte('[')
	for i, idx := range indexes {
		if i > 0 {
			out.WriteByte(',')
		}
		fmt.Fprintf(&out, `{"index":%d,"url":%s}`, i, strconv.Quote(pages[idx]))
	}
	out.WriteByte(']')
	return out.String(), nil
}
