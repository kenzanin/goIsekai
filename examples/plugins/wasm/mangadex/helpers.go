//go:build wasip1

package main

import (
	"goisekai/pkg/types"
	"net/url"
	"strconv"
	"time"
)

func defaultHeaders() map[string]string {
	return map[string]string{
		"Referer": cdnURL + "/",
	}
}

func firstTitle(attrs mangaAttrs) string {
	if t, ok := attrs.Title[lang]; ok && t != "" {
		return t
	}
	for _, at := range attrs.AltTitles {
		if t, ok := at[lang]; ok && t != "" {
			return t
		}
	}
	for _, at := range attrs.AltTitles {
		for _, t := range at {
			if t != "" {
				return t
			}
		}
	}
	for _, t := range attrs.Title {
		if t != "" {
			return t
		}
	}
	return ""
}

// firstLang returns the first non-empty value from a localized string map,
// preferring English, mirroring firstTitle.
func firstLang(m map[string]string) string {
	if v, ok := m[lang]; ok && v != "" {
		return v
	}
	for _, v := range m {
		if v != "" {
			return v
		}
	}
	return ""
}

func coverURL(md mangaData) string {
	for _, r := range md.Relationships {
		if r.Type == "cover_art" && r.Attributes != nil {
			return cdnURL + "/covers/" + md.ID + "/" + r.Attributes.FileName + ".256.jpg"
		}
	}
	return ""
}

func toManga(md mangaData) types.Manga {
	tags := make([]string, 0, len(md.Attributes.Tags))
	for _, t := range md.Attributes.Tags {
		if n := firstLang(t.Attributes.Name); n != "" {
			tags = append(tags, n)
		}
	}
	return types.Manga{
		ID:          md.ID,
		Title:       firstTitle(md.Attributes),
		CoverURL:    coverURL(md),
		Description: firstLang(md.Attributes.Description),
		Status:      md.Attributes.Status,
		Genres:      tags,
	}
}

func parseFloat64(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func contentRatingQuery(q url.Values) {
	q.Add("contentRating[]", "safe")
	q.Add("contentRating[]", "suggestive")
	q.Add("contentRating[]", "erotica")
}
