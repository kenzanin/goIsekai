package database

import (
	"goisekai/internal/pluginutil"
)

// normalizeTitle delegates to the shared normalization rule in pluginutil.
func normalizeTitle(s string) string {
	return pluginutil.NormalizeTitle(s)
}

// spansPlugins reports whether the member indices come from at least two
// different plugin IDs.
func spansPlugins(members map[int]struct{}, all []Manga) bool {
	var seen string
	for idx := range members {
		if seen == "" {
			seen = all[idx].PluginID
		} else if seen != all[idx].PluginID {
			return true
		}
	}
	return false
}
