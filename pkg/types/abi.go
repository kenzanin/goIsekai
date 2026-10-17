package types

// ContractVersion is the current version of the host/plugin ABI. A plugin
// declares the version it was written against; the host rejects a mismatch so
// an incompatible plugin never gets instantiated.
const ContractVersion int32 = 1

// Host function names exported by every plugin. Each accepts a JSON string
// argument and returns a JSON string result over WASM memory.
//
// The same five entry points exist in every runtime, differing only in case
// convention: Lua defines snake_case globals (search, get_manga_detail, ...),
// JavaScript camelCase globals (search, getMangaDetail, ...), and Go, Yaegi and
// WASM export the PascalCase names below.
const (
	// SearchFunc accepts a SearchFilter JSON and returns a JSON array of Manga.
	SearchFunc = "Search"
	// GetMangaDetailFunc accepts a manga id and returns a Manga JSON object.
	GetMangaDetailFunc = "GetMangaDetail"
	// GetChapterListFunc accepts a manga id and returns a JSON array of Chapter.
	GetChapterListFunc = "GetChapterList"
	// GetPageListFunc accepts a chapter id and returns a JSON array of Page.
	GetPageListFunc = "GetPageList"
	// ContractVersionFunc is the per-runtime export carrying the ABI version.
	// Which symbol answers it differs by runtime: a function for WASM, an
	// exported int32 variable for compiled Go plugins, PLUGIN.contract_version
	// for Lua, and nothing at all for JS/Yaegi (they assume the current ABI).
	ContractVersionFunc = "contract_version"
	// InitFunc is a legacy export name. Metadata lives in plugin.json for every
	// runtime, and no runtime reads an Init export anymore — a plugin that still
	// defines one is compiled but never called.
	InitFunc = "Init"
	// GetEnrichmentFunc is the entry point an enrichment script (a folder in
	// the info directory) defines. Such a script serves metadata only — it
	// never scrapes a site and is never offered as a manga source — so it
	// exports this instead of the source ABI above. Input: JSON
	// {"title":string,"kind":string,"source":string}.
	// Output: JSON array of items [{value,url,cover_url,source}, ...].
	GetEnrichmentFunc = "GetEnrichment"
	// GetGenresFunc is an OPTIONAL export. When present, the host calls it
	// with no arguments and expects a JSON array of
	// [{name:string, slug:string}] describing the source's browsable genres.
	GetGenresFunc = "GetGenres"
)

// HostHTTPRequestFunc is the host-imported function available to plugins for
// all network access. It accepts an HTTPRequest JSON and returns an
// HTTPResponse JSON.
const HostHTTPRequestFunc = "host_http_request"

// HostCallFunc is the generic host-helper import available to wasm plugins:
// {"fn":"<namespace>.<name>","args":[...]} in, the JSON result out, or
// {"error":"..."} on failure. It is the wasm equivalent of the Lua/JS `host`
// object and reaches the same Go implementations. See
// docs/plugin-wasm-helpers.md.
const HostCallFunc = "host_call"
