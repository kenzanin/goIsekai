//go:build wasip1

package main

import (
	"encoding/json"
	"strings"

	"goisekai/pkg/types"
)

// ─── site facts ──────────────────────────────────────────────────────────────

var base string // plugin.json param "site_url"

func init() {
	base = param("site_url")
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
	var url string
	if len(f.Genres) > 0 {
		// Genre browsing: first genre wins. /manga-genre/<slug>/, page N under /page/N/.
		url = base + "/manga-genre/" + urlEncode(f.Genres[0]) + "/"
		if f.Page > 1 {
			url = url + "page/" + itoa(f.Page) + "/"
		}
	} else {
		url = base + "/?s=" + urlEncode(f.Query) + "&post_type=wp-manga"
		if f.Page > 1 {
			url = url + "&paged=" + itoa(f.Page)
		}
	}

	body := htmlGet(url)
	out := []types.Manga{}
	seen := map[string]bool{}
	// Match /manga/<slug>/ — capture the slug. The host's regex.find_all with one
	// capture group returns [capture] per match, so gAll yields slug strings.
	for _, slug := range gAll(body, `/manga/([^/]+/)`) {
		slug = strings.TrimSuffix(slug, "/")
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, types.Manga{ID: slug, Title: slug})
	}
	return out
}

// ─── ABI: get_manga_detail ───────────────────────────────────────────────────

func detail(arg string) types.Manga {
	slug := decodeString(arg)
	m := types.Manga{ID: slug}
	if slug == "" {
		return m
	}

	body := htmlGet(base + "/manga/" + slug + "/")
	if body == "" {
		return m
	}

	coverBlock := firstCap(body, `(?s)class="summary_image".*?</a>`)
	cover := firstCap(coverBlock, `data-src="([^"]+)"`)
	if cover == "" {
		cover = firstCap(coverBlock, `\ssrc="([^"]+)"`)
	}
	m.CoverURL = strings.TrimSpace(cover)

	tblock := firstCap(body, `(?s)class="post-title[^"]*">(.*?)</div>`)
	m.Title = unescape(strings.TrimSpace(firstCap(tblock, `(?s)<h[1-6][^>]*>(.*?)</h[1-6]>`)))

	ablock := firstCap(body, `(?s)class="author-content">(.*?)</div>`)
	m.Author = strings.ReplaceAll(stripHTML(ablock), ",", ", ")

	gblock := firstCap(body, `(?s)class="genres-content">(.*?)</div>`)
	for _, g := range gAll(gblock, `<a[^>]*>([^<]+)</a>`) {
		name := unescape(strings.TrimSpace(g))
		if name != "" {
			m.Genres = append(m.Genres, name)
		}
	}

	sblock := firstCap(body, `(?s)class="post-status">(.*?)$`)
	if sblock != "" {
		raw := firstCap(sblock, `(?s)Status\s*</h5>.*?class="summary-content">\s*([^<]+)`)
		if raw != "" {
			m.Status = hostString("text.normalize_status", strings.TrimSpace(raw))
		}
	}

	dblock := firstCap(body, `(?s)class="summary__content[^"]*">(.*?)<span\s+class="[^"]*content-readmore"`)
	if dblock == "" {
		dblock = firstCap(body, `(?s)class="summary__content[^"]*">(.*?)</div>`)
	}
	m.Description = hostString("text.strip_html", dblock)

	return m
}

// ─── ABI: get_chapter_list ──────────────────────────────────────────────────

func chapters(arg string) []types.Chapter {
	slug := decodeString(arg)
	if slug == "" {
		return nil
	}
	body := htmlPost(base+"/manga/"+slug+"/ajax/chapters/", "", `{"X-Requested-With":"xmlhttprequest"}`)
	out := []types.Chapter{}
	for _, li := range gAll(body, `(?s)<li\s+class="wp-manga-chapter[^>]*>(.*?)</li>`) {
		href := firstCap(li, `href="([^"]+)"`)
		label := strings.TrimSpace(firstCap(li, `>([^<]*[Cc]hapter[^<]*)<`))
		date := strings.TrimSpace(firstCap(li, `chapter-release-date[^>]*>\s*<i[^>]*>([^<]+)</i>`))
		if date == "" {
			date = strings.TrimSpace(firstCap(li, `chapter-release-date[^>]*>\s*<a[^>]*>([^<]+)</a>`))
		}
		if href == "" || label == "" {
			continue
		}
		cid := firstCap(href, `/manga/[^/]+/([^/]+)/$`)
		if cid == "" {
			cid = label
		}
		c := types.Chapter{
			ID:         slug + ":" + cid,
			MangaID:    slug,
			Title:      label,
			ChapterNum: chapterNum(label),
			URL:        href,
		}
		if iso := dateToISO(date); iso != "" {
			c.ReleasedAt = parseTime(iso)
		}
		out = append(out, c)
	}
	// Madara's /ajax/chapters/ returns newest-first already; keep site order.
	return out
}

// ─── ABI: get_page_list ─────────────────────────────────────────────────────

func pages(arg string) []types.Page {
	composite := decodeString(arg)
	if composite == "" {
		return nil
	}
	// "slug:chapter-N" → /manga/slug/chapter-N/
	path := strings.ReplaceAll(composite, ":", "/")

	body := htmlGet(base + "/manga/" + path + "/")
	out := []types.Page{}
	for _, src := range firstCapAll(body, `data-src="([^"]*)"`) {
		if src != "" {
			out = append(out, types.Page{Index: len(out), URL: strings.TrimSpace(src)})
		}
	}
	return out
}

// ─── get_genres ──────────────────────────────────────────────────────────────

var genreSlugs = []string{
	"Action", "Adult", "Adventure", "Anime", "Comedy", "Comic", "Cooking",
	"Drama", "Ecchi", "Fantasy", "Game", "Gender Bender", "Harem", "Historical",
	"Horror", "Isekai", "Josei", "LGBTQ", "Love", "Magic", "Mecha", "Medical",
	"Music", "Mystery", "Parody", "Philosophical", "Political", "Post-apocalypse",
	"Psychological", "Romance", "School Life", "Sci-Fi", "Seinen", "Shoujo",
	"Shounen", "Slice of Life", "Smut", "Sports", "Super Power", "Supernatural",
	"Survival", "Tragedy", "War", "Wuxia", "Yaoi", "Yuri",
}

func genres() []map[string]string {
	out := make([]map[string]string, 0, len(genreSlugs))
	for _, slug := range genreSlugs {
		out = append(out, map[string]string{"name": slug, "slug": slug})
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
