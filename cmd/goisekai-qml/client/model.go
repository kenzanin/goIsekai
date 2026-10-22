package client

import (
	"net/url"
	"strconv"
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

// StatusFilter values, matching the server's libraryStatuses vocabulary in
// internal/httpserver/library_sort.go. Client-side filtering must use the same
// predicates or the two front ends disagree on the counts.
const (
	StatusAll     = "all"
	StatusReading = "reading"
	StatusUnread  = "unread"
	StatusDone    = "done"
	StatusNew     = "new"
)

// MatchesStatus reports whether item belongs to the given status filter.
func MatchesStatus(i Item, status string) bool {
	switch status {
	case StatusReading:
		return i.Read > 0 && i.Total > 0 && i.Read < i.Total
	case StatusUnread:
		return i.Read == 0
	case StatusDone:
		return i.Total > 0 && i.Read >= i.Total
	case StatusNew:
		return i.HasNew
	default:
		return true
	}
}

// Sort keys, matching the server's librarySorts.
const (
	SortUpdated    = "updated"
	SortTitle      = "title"
	SortRead       = "read"
	SortProgress   = "progress"
	SortRecentRead = "recent_read"
	SortDuplicates = "duplicates"
)

// SortOrders lists the sort keys in the order the toolbar shows them.
var SortOrders = []string{SortUpdated, SortTitle, SortRead, SortProgress, SortRecentRead, SortDuplicates}

// DupGroupKey turns a dup group index into a string, or "" when the item is not
// a duplicate. QML compares strings more predictably than -1 sentinels across
// the model boundary.
func DupGroupKey(i Item) string {
	if i.DupGroup < 0 {
		return ""
	}
	return strconv.Itoa(i.DupGroup)
}
