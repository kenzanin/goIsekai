// Demo Scriggo plugin for goIsekai.
//
// Scriggo plugins are sandboxed Go source programs. The host rewrites this
// file's `package main` clause to `package plugin`, places it in a virtual
// module alongside a generated dispatch shim, and compiles it with
// github.com/open2b/scriggo. Use this file as the starting point for real
// source plugins.
//
// Sandbox rules (what a Scriggo plugin CAN import — everything else fails at
// build time):
//   - hostnet: host.Get(url) / host.Post(url, body) — HTTP through the host's
//     TLS-fingerprinted proxy. A bare URL arg makes the demo Search below call
//     host.Get so the round trip is observable from the playground/sandbox.
//   - fmt: Println/Printf/Sprintf/Sprint/Errorf.
//   - hostapi: the implicit host bridge — never imported directly by plugins.
//
// Stdlib (strings, encoding/json, os, net/http, ...) is NOT available, so
// plugins build and read JSON by hand (fmt.Sprintf + string slicing). ABI
// functions MUST be exported with these exact names, take one string and
// return (string, error) — except Init, which takes no argument and returns
// the PluginMeta JSON string (optional, mirrors the WASM runtime). The arg is
// the raw JSON string the host dispatched (a search filter object for Search,
// a JSON-encoded plain string for the other functions); the result must be a
// JSON string the host can unmarshal into the matching pkg/types value.
package main

import (
	"hostnet"
	"fmt"
)

// Init returns the PluginMeta JSON (optional export). contract_version is
// assumed 1 by the host, exactly like the JS runtime.
func Init() string {
	return `{"name":"Scriggo Demo","verify_url":"https://example.com","needs_human_verify":false,"thumb_ratio":0.703,"search_page_size":24}`
}

// hasPrefix is a tiny local helper — the strings package is not importable.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// Search(arg) — arg is a JSON SearchFilter object like {"query":"...","page":1}.
// Returns: JSON array of {id, title, author, cover_url, status}.
//
// When arg is a bare http(s) URL (playground/sandbox usage), this demonstrates
// the hostnet.Get round trip and embeds the response head into the result so
// the fetch is observable without breaking the []Manga ABI shape.
func Search(arg string) (string, error) {
	if hasPrefix(arg, "http://") || hasPrefix(arg, "https://") {
		body, err := hostnet.Get(arg)
		title := "hostnet.Get failed: " + err.Error()
		if err == nil {
			if len(body) > 80 {
				body = body[:80]
			}
			title = "Fetched " + arg + " -> " + body
		}
		// Keep the title JSON-safe for the demo: strip double quotes and
		// backslashes that may appear in a raw HTML response head.
		var clean string
		for i := 0; i < len(title); i++ {
			switch title[i] {
			case '"', '\\', '\n', '\r', '\t':
				clean += " "
			default:
				clean += string(title[i])
			}
		}
		return `[{"id":"fetch","title":"` + clean + `","cover_url":""}]`, nil
	}

	return `[
  {"id":"sd-1","title":"Scriggo: The Sandboxed Chronicles","cover_url":"https://picsum.photos/seed/scriggo-demo-1/400/560","author":"Demo Author","description":"A dummy isekai action series used as the Scriggo plugin reference.","status":"ongoing"},
  {"id":"sd-2","title":"My Demo Girlfriend Is a Go Routine","cover_url":"https://picsum.photos/seed/scriggo-demo-2/400/560","author":"Demo Author","description":"A dummy slice-of-life romance series.","status":"completed"},
  {"id":"sd-3","title":"The Interpreter Below","cover_url":"https://picsum.photos/seed/scriggo-demo-3/400/560","author":"Demo Author","description":"A dummy horror/mystery series.","status":"ongoing"}
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
		return `{"id":"sd-2","title":"My Demo Girlfriend Is a Go Routine","author":"Demo Author","description":"A dummy slice-of-life romance series.","cover_url":"https://picsum.photos/seed/scriggo-demo-2/400/560","genres":["romance","slice of life"],"status":"completed"}`, nil
	case "sd-3":
		return `{"id":"sd-3","title":"The Interpreter Below","author":"Demo Author","description":"A dummy horror/mystery series.","cover_url":"https://picsum.photos/seed/scriggo-demo-3/400/560","genres":["horror","mystery"],"status":"ongoing"}`, nil
	default:
		return `{"id":"sd-1","title":"Scriggo: The Sandboxed Chronicles","author":"Demo Author","description":"A dummy isekai action series used as the Scriggo plugin reference.","cover_url":"https://picsum.photos/seed/scriggo-demo-1/400/560","genres":["action","fantasy","isekai"],"status":"ongoing"}`, nil
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
		out += fmt.Sprintf(`{"index":%d,"url":"https://picsum.photos/seed/scriggo-page-%d/600/900"}`, i, i)
	}
	out += "]"
	return out, nil
}
