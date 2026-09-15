package types

// ContractVersion is the current version of the host/plugin ABI. The host
// reads a plugin's exported contract_version symbol and rejects it when it does
// not match this value.
const ContractVersion int32 = 1

// Host function names exported by every plugin. Each accepts a JSON string
// argument and returns a JSON string result over WASM memory.
const (
	// SearchFunc accepts a SearchFilter JSON and returns a JSON array of Manga.
	SearchFunc = "Search"
	// GetMangaDetailFunc accepts a manga id and returns a Manga JSON object.
	GetMangaDetailFunc = "GetMangaDetail"
	// GetChapterListFunc accepts a manga id and returns a JSON array of Chapter.
	GetChapterListFunc = "GetChapterList"
	// GetMangaDetailWithChaptersFunc accepts a manga id and returns both
	// Manga detail and Chapter list in a single call for efficiency.
	GetMangaDetailWithChaptersFunc = "GetMangaDetailWithChapters"
	// GetPageListFunc accepts a chapter id and returns a JSON array of Page.
	GetPageListFunc = "GetPageList"
	// ContractVersionFunc is exported by each plugin and returns the ABI
	// version it was compiled against.
	ContractVersionFunc = "contract_version"
	// InitFunc is an optional export. When present, the host calls it once at
	// load time with no arguments; it returns a PluginMeta JSON object.
	InitFunc = "Init"
	// GetEnrichmentFunc is an OPTIONAL enrichment export. When present, the
	// plugin can fetch enrichment items (titles, summaries, categories,
	// authors, related) for a manga title. Input: JSON
	// {"title":string,"kind":string,"source":string}.
	// Output: JSON array of items [{value,url,cover_url,source}, ...].
	GetEnrichmentFunc = "GetEnrichment"
)

// HostHTTPRequestFunc is the host-imported function available to plugins for
// all network access. It accepts an HTTPRequest JSON and returns an
// HTTPResponse JSON.
const HostHTTPRequestFunc = "host_http_request"
