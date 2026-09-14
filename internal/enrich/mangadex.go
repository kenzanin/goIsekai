package enrich

import (
	"bytes"
	"context"
	"fmt"
	"github.com/goccy/go-json"
	"net/http"
)

// MangaDexProvider fetches enrichment data from MangaDex (api.mangadex.org).
type MangaDexProvider struct{}

// NewMangaDexProvider returns a new MangaDexProvider.
func NewMangaDexProvider() *MangaDexProvider {
	return &MangaDexProvider{}
}

// mangaDexBase is the base URL for MangaDex API.
var mangaDexBase = "https://api.mangadex.org"

// muBase is the base URL for MangaUpdates API.
var muBase = "https://api.mangaupdates.com"

func (p *MangaDexProvider) ID() string   { return "mangadex" }
func (p *MangaDexProvider) Name() string { return "MangaDex" }
func (p *MangaDexProvider) Kinds() []Kind {
	return []Kind{KindTitles, KindCategories, KindRelated}
}

// mangaDexSearchResponse mirrors the top-level shape of the MangaDex search API.
type mangaDexSearchResponse struct {
	Result string          `json:"result"`
	Data   []mangaDexManga `json:"data"`
}

type mangaDexManga struct {
	ID            string             `json:"id"`
	Attributes    mangaDexMangaAttrs `json:"attributes"`
	Relationships []mangaDexRelation `json:"relationships"`
}

type mangaDexMangaAttrs struct {
	Title     map[string]string   `json:"title"`     // {en: "...", ko: "..."}
	AltTitles []map[string]string `json:"altTitles"` // [{ko:"..."}, {en:"..."}]
	Tags      []mangaDexTag       `json:"tags"`
}

type mangaDexTag struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Attributes mangaDexTagAttrs `json:"attributes"`
}

type mangaDexTagAttrs struct {
	Name  map[string]string `json:"name"`  // {en: "Action"}
	Group string            `json:"group"` // "genre", "format", "content"
}

type mangaDexRelation struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes"`
}

// mangaDexRelInfo holds the parsed relation data (id, name).
type mangaDexRelInfo struct {
	ID   string `json:"id"`
	Name string `json:"title"`
}

// mangaDexRelation parses a MangaDex relationships entry. Only "manga"
// relationships are returned; others are silently skipped.
func mangaDexRelationParse(r mangaDexRelation) (*mangaDexRelInfo, bool) {
	if r.Type != "manga" {
		return nil, false
	}
	var info mangaDexRelInfo
	if err := json.Unmarshal(r.Attributes, &info); err != nil {
		return nil, false
	}
	if info.Name == "" {
		return nil, false
	}
	return &info, true
}

func (p *MangaDexProvider) Fetch(ctx context.Context, httpc *http.Client, title string, k Kind) ([]Item, error) {
	normalized := normalizeTitle(title)
	url := fmt.Sprintf("%s/manga?title=%s&limit=5", mangaDexBase, urlQuery(normalized))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("mangadex: build request: %w", err)
	}
	req.Header.Set("Accept-Language", "en")

	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mangadex: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mangadex: status %d", resp.StatusCode)
	}

	var result mangaDexSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("mangadex: decode: %w", err)
	}

	switch k {
	case KindTitles:
		return p.fetchTitles(result), nil
	case KindCategories:
		return p.fetchCategories(result), nil
	case KindRelated:
		return p.fetchRelated(result), nil
	default:
		return nil, fmt.Errorf("mangadex: unknown kind %q", k)
	}
}

// fetchTitles returns the first (best-matching) manga's alt titles.
func (p *MangaDexProvider) fetchTitles(resp mangaDexSearchResponse) []Item {
	if len(resp.Data) == 0 {
		return nil
	}
	d := resp.Data[0]
	var titles []Item
	// Collect all alt titles, preferring English.
	var primary string
	for lang, name := range d.Attributes.Title {
		if lang == "en" {
			primary = name
		} else if primary == "" {
			primary = name
		}
	}
	for _, at := range d.Attributes.AltTitles {
		if en, ok := at["en"]; ok && en != "" {
			titles = append(titles, Item{Value: en})
		} else if len(at) > 0 {
			for _, v := range at {
				titles = append(titles, Item{Value: v})
				break
			}
		}
	}
	if len(titles) == 0 && primary != "" {
		titles = append(titles, Item{Value: primary})
	}
	return titles
}

// fetchCategories returns unique genre tag names from the first (best-matching) result.
func (p *MangaDexProvider) fetchCategories(resp mangaDexSearchResponse) []Item {
	if len(resp.Data) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var items []Item
	for _, tag := range resp.Data[0].Attributes.Tags {
		if tag.Attributes.Group != "genre" {
			continue
		}
		name := tag.Attributes.Name["en"]
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		items = append(items, Item{Value: name})
	}
	return items
}

// fetchRelated returns related manga from the first search result.
func (p *MangaDexProvider) fetchRelated(resp mangaDexSearchResponse) []Item {
	if len(resp.Data) == 0 {
		return nil
	}
	var items []Item
	for _, r := range resp.Data[0].Relationships {
		info, ok := mangaDexRelationParse(r)
		if !ok {
			continue
		}
		items = append(items, Item{Value: info.Name, URL: ""})
	}
	return items
}

// urlQuery is a minimal URL query encoder that escapes special characters.
func urlQuery(s string) string {
	var buf bytes.Buffer
	for _, b := range []byte(s) {
		switch b {
		case '-', '_', '.', '~', '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', '/', ':', '@', '?', '%':
			buf.WriteByte(b)
		default:
			if b >= 0x80 {
				// Non-ASCII: use UTF-8 percent-encoding.
				buf.WriteByte('%')
				h, l := b>>4, b&0xF
				buf.WriteByte("0123456789ABCDEF"[h])
				buf.WriteByte("0123456789ABCDEF"[l])
			} else if b > 0x7E || b <= 0x20 {
				buf.WriteByte('%')
				h, l := b>>4, b&0xF
				buf.WriteByte("0123456789ABCDEF"[h])
				buf.WriteByte("0123456789ABCDEF"[l])
			} else {
				buf.WriteByte(b)
			}
		}
	}
	return buf.String()
}
