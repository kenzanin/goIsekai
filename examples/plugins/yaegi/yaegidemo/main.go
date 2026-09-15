// Demo Yaegi plugin for goIsekai.
//
// Yaegi plugins are sandboxed Go source programs. The host interprets this
// file as-is with github.com/traefik/yaegi. Use this file as the starting point for real
// source plugins.
//
// Note:
// plugins build and read JSON by hand (fmt.Sprintf + string slicing). ABI
// ABI functions MUST be exported with these exact names, take one string and
// return (string, error) — except Init, which takes no argument and returns
// the PluginMeta JSON string (optional, mirrors the WASM runtime). The arg is
// the raw JSON string the host dispatched (a search filter object for Search,
// a JSON-encoded plain string for the other functions); the result must be a
// JSON string the host can unmarshal into the matching pkg/types value.
package main

import (
	"fmt"
	"hostnet"
)

// Init returns the PluginMeta JSON (optional export). contract_version is
// assumed 1 by the host, exactly like the JS runtime.
func Init() string {
	return `{"name":"Yaegi Demo","site_url":"https://github.com/traefik/yaegi","logo":"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Ctext y='28' font-size='28'%3E🧪%3C/text%3E%3C/svg%3E","verify_url":"https://example.com","needs_human_verify":false,"thumb_ratio":0.703,"search_page_size":24}`
}

// hasPrefix is a tiny local helper — the strings package is not importable.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// Search(arg) — arg is a JSON SearchFilter object like {"query":"...","page":1}.
// Returns: JSON array of {id, title, author, cover_url, status}.
func Search(arg string) (string, error) {
	return `[
  {"id":"sd-1","title":"Yaegi: The Interpreted Chronicles","cover_url":"https://picsum.photos/seed/yaegi-demo-1/400/560","author":"Demo Author","description":"A dummy isekai action series used as the Yaegi plugin reference.","status":"ongoing"},
  {"id":"sd-2","title":"My Demo Girlfriend Is a Go Routine","cover_url":"https://picsum.photos/seed/yaegi-demo-2/400/560","author":"Demo Author","description":"A dummy slice-of-life romance series.","status":"completed"},
  {"id":"sd-3","title":"The Interpreter Below","cover_url":"https://picsum.photos/seed/yaegi-demo-3/400/560","author":"Demo Author","description":"A dummy horror/mystery series.","status":"ongoing"}
]`, nil
}

// GetMangaDetail(arg) — arg is a JSON-encoded plain string (e.g. '"sd-1"').
// Returns: {id, title, author, description, cover_url, genres, status}.
func GetMangaDetail(arg string) (string, error) {
	id := arg
	if len(id) >= 2 {
		id = id[1 : len(id)-1] // strip the surrounding JSON quotes
	}
	switch id {
	case "sd-2":
		return `{"id":"sd-2","title":"My Demo Girlfriend Is a Go Routine","author":"Demo Author","description":"A dummy slice-of-life romance series.","cover_url":"https://picsum.photos/seed/yaegi-demo-2/400/560","genres":["romance","slice of life"],"status":"completed"}`, nil
	case "sd-3":
		return `{"id":"sd-3","title":"The Interpreter Below","author":"Demo Author","description":"A dummy horror/mystery series.","cover_url":"https://picsum.photos/seed/yaegi-demo-3/400/560","genres":["horror","mystery"],"status":"ongoing"}`, nil
	default:
		return `{"id":"sd-1","title":"Yaegi: The Interpreted Chronicles","author":"Demo Author","description":"A dummy isekai action series used as the Yaegi plugin reference.","cover_url":"https://picsum.photos/seed/yaegi-demo-1/400/560","genres":["action","fantasy","isekai"],"status":"ongoing"}`, nil
	}
}

// GetChapterList(arg) — arg is a JSON-encoded plain string (manga id).
// Returns: JSON array of {id, manga_id, title, chapter_num, released_at, url}.
// Demonstrates fmt.Sprintf building JSON from loop state.
func GetChapterList(arg string) (string, error) {
	mangaID := arg
	if len(mangaID) >= 2 {
		mangaID = mangaID[1 : len(mangaID)-1]
	}
	var out string
	out += "["
	for i := 1; i <= 3; i++ {
		if i > 1 {
			out += ","
		}
		title := fmt.Sprintf("Chapter %d", i)
		out += fmt.Sprintf(`{"id":"%s:chapter-%d","manga_id":"%s","title":"%s","chapter_num":%d,"released_at":"2026-01-0%dT00:00:00Z","url":"https://example.com/chapter/%d"}`, mangaID, i, mangaID, title, i, i, i)
	}
	out += "]"
	return out, nil
}

// GetPageList(arg) — arg is a JSON-encoded plain string (chapter id).
// Returns: JSON array of {index, url}.
func GetPageList(arg string) (string, error) {
	var out string
	out += "["
	for i := 0; i < 3; i++ {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf(`{"index":%d,"url":"https://picsum.photos/seed/yaegi-page-%d/600/900"}`, i, i)
	}
	out += "]"
	return out, nil
}

// ExampleHTMLDemo demonstrates using the hostnet HTML helpers.
// This is not part of the plugin ABI but shows how a plugin would use the
// HTML parsing functionality.
func ExampleHTMLDemo(markup string) (string, error) {
	doc, err := hostnet.Parse(markup)
	if err != nil {
		return "", err
	}
	title, err := hostnet.FindText(doc, "h1")
	if err != nil {
		return "", err
	}
	srcs, err := hostnet.FindListAttr(doc, "img", "src")
	if err != nil {
		return "", err
	}
	out := fmt.Sprintf(`{"title":"%s","srcs":[`, title)
	for i, src := range srcs {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf(`%q`, src)
	}
	out += "]}"
	return out, nil
}
