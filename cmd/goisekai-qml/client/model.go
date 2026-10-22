package client

import (
	"net/url"
)

// Item is one library row as the QML views consume it. Field names become
// model roles, so they are lowerCamelCase by convention.
type Item struct {
	Title string `json:"title"`
	// Cover is always a server-proxied /image URL, never the upstream CDN.
	Cover      string `json:"cover"`
	Read       int    `json:"read"`
	Total      int    `json:"total"`
	HasNew     bool   `json:"hasNew"`
	UpdatedAt  string `json:"updatedAt"`
	LastReadAt string `json:"lastReadAt"`
	// DupGroup is -1 when the manga is not part of a duplicate group.
	DupGroup int    `json:"dupGroup"`
	Href     string `json:"href"`
	// Key is plugin_id:source_manga_id, the key the facets' tags map uses.
	Key      string `json:"key"`
	PluginID string `json:"pluginId"`
}

// Progress returns read/total as a 0..1 fraction, 0 when there is nothing to
// read. Kept here so the QML does not repeat the divide-by-zero guard.
func (i Item) Progress() float64 {
	if i.Total <= 0 {
		return 0
	}
	r := float64(i.Read) / float64(i.Total)
	if r > 1 {
		return 1
	}
	return r
}

// coverProxyURL routes a cover through the server's /image proxy. Binding the
// upstream URL directly would 403 (the CDN wants the host's Referer) and would
// bypass the on-disk cache and AVIF downscale.
func coverProxyURL(base, pluginID, coverURL string) string {
	if coverURL == "" {
		return ""
	}
	return base + "/image?pluginID=" + url.QueryEscape(pluginID) +
		"&url=" + url.QueryEscape(coverURL)
}

// mangaHref builds the detail-page link. pluginID is a single path segment, but
// sourceMangaID may itself contain slashes (batcave uses full permalinks), so
// both go through PathEscape rather than being concatenated raw.
func mangaHref(base, pluginID, sourceMangaID string) string {
	return base + "/view/manga/" + url.PathEscape(pluginID) +
		"/" + url.PathEscape(sourceMangaID)
}
