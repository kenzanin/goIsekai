// madarascans — native .so plugin for goIsekai (C++17).
//
// madarascans.net is a WordPress manga site on a Madara-like theme with the
// WP REST API disabled (wp-json 404s), so every read is HTML scraping. HTTP
// goes through the host bridge (native_abi.h) rather than libcurl, so requests
// inherit the host's TLS fingerprinting, cookie jar, rate limiting and CDP
// challenge solving — the same proxy the Lua/JS/WASM plugins use.
//
// Endpoints (from madarascans.har + live probes):
//   search:  GET /?s=<q>;           page N: GET /page/N/?s=<q>
//   browse:  GET /series/?page=N;   genre:  GET /series/?genre[]=<id>&page=N
//   detail:  GET /series/<slug>/
//   chapter: GET /<slug>-chap-N/    -> ts_reader.run({... sources[].images[]})
//   genres:  GET /series/ (the filter form lists every genre with its id)
//
// ID scheme:
//   Manga.ID   = series slug, e.g. "dungeon-haishin-...-musou"
//   Chapter.ID = chapter URL slug, e.g. "dungeon-haishin-...-musou-chap-41"
//
// Pagination: the host slices the plugin's result list itself
// (results[(page-1)*30 : page*30]), so Search must return a cumulative window,
// not just the site's Nth page. Site pages are 10 (search) / 30 (browse) per
// page, so we keep pulling site pages until the window for `page` is covered.

#include <string>
#include <vector>

#include "util.h"

static const char *kBase = "https://madarascans.net";
static const int kHostPageSize = 30; // must match httpserver.searchPageSize

// ─── patterns ───────────────────────────────────────────────────────────────
// NB: custom raw-string delimiter R"re(...)re" — several patterns contain
// `)"`, which would terminate the default R"( ... )" early.

static const std::regex reCard(R"re(<a href="(https://madarascans\.net/series/[^"]+)" title="([^"]*)"[\s\S]{0,800}?<img src="([^"]+)")re");
static const std::regex reTitleH1(R"re(<h1 class="entry-title"[^>]*>([\s\S]*?)</h1>)re");
static const std::regex reOGImage(R"re(<meta property="og:image" content="([^"]+)")re");
static const std::regex reDesc(R"re(<div class="entry-content entry-content-single"[^>]*>([\s\S]*?)</div>)re");
static const std::regex reStatusDot(R"re(<span class="status-dot ([A-Za-z]+)")re");
static const std::regex reGenre(R"re(<a href="https://madarascans\.net/genres/[^"/]+/" rel="tag">([^<]+)</a>)re");
static const std::regex reAuthor(R"re(<i itemprop="name">([^<]*)</i>)re");
static const std::regex reChapLi(R"re(<li data-num="([^"]+)">([\s\S]*?)</li>)re");
static const std::regex reHref(R"re(href="([^"]+)")re");
static const std::regex reChapNum(R"re(<span class="chapternum">([\s\S]*?)</span>)re");
static const std::regex reChapDate(R"re(<span class="chapterdate">([^<]*)</span>)re");

// ─── genre table ────────────────────────────────────────────────────────────
// Scraped once from the /series filter form (input[name="genre[]"]). The value
// the site's own filter posts is the numeric id, so that is what a genre's slug
// carries back through SearchFilter.Genres.

struct GenreTag {
	const char *name;
	const char *id;
};
static const GenreTag kGenres[] = {
    {"Academy", "296"},        {"Action", "2"},         {"Adult", "25"},
    {"Adventure", "3"},        {"Bloody", "111"},       {"Comedy", "66"},
    {"Crime", "144"},          {"Death Game", "108"},   {"Demon", "32"},
    {"Demons", "152"},         {"Doujinshi", "55"},     {"Drama", "4"},
    {"Ecchi", "13"},           {"Fantasy", "5"},        {"Harem", "14"},
    {"Hentai", "56"},          {"Historical", "9"},     {"Horror", "63"},
    {"Idols", "278"},          {"Josei", "68"},         {"Magic", "33"},
    {"Manga", "34"},           {"Manhwa", "36"},        {"Martial Arts", "10"},
    {"Mature", "28"},          {"Murim", "37"},         {"Mystery", "85"},
    {"Plot", "143"},           {"Psychological", "174"}, {"Reincarnation", "39"},
    {"Romance", "69"},         {"School", "107"},       {"School Life", "86"},
    {"Sci-fi", "71"},          {"Seinen", "11"},        {"Sexual Violence", "172"},
    {"Shoujo", "77"},          {"Shounen", "15"},       {"Slice of Life", "105"},
    {"Sports", "217"},         {"Supernatural", "18"},  {"Survival", "110"},
    {"System", "38"},          {"Tragedy", "242"},      {"Vampire", "301"},
    {"Webtoon", "100"},
};

static std::string genre_id_for(const std::string &slug) {
	for (const auto &g : kGenres) {
		if (slug == g.id) return g.id;
	}
	return "";
}

// ─── card parsing (shared by search and browse) ─────────────────────────────

// parse_cards appends every unique series card in `body` to `out`. The same
// markup backs both the search results grid and the /series browse archive.
static void parse_cards(const std::string &body, rapidjson::Value &out,
                        rapidjson::Document::AllocatorType &alloc, std::vector<std::string> &seen) {
	for (std::sregex_iterator it(body.begin(), body.end(), reCard), end; it != end; ++it) {
		std::string href = (*it)[1].str();
		std::string slug = slug_of(href);
		if (slug.empty()) continue;
		bool dup = false;
		for (const auto &s : seen)
			if (s == slug) dup = true;
		if (dup) continue;
		seen.push_back(slug);

		rapidjson::Value m(rapidjson::kObjectType);
		add_str(m, "id", slug, alloc);
		add_str(m, "title", clean((*it)[2].str()), alloc);
		add_str(m, "cover_url", trim((*it)[3].str()), alloc);
		out.PushBack(m, alloc);
	}
}

// site_url_for builds the listing URL for one *site* page (1-based). A text
// query uses WordPress search (/page/N/?s=); an empty query browses /series,
// optionally narrowed to one genre id.
static std::string site_url_for(const std::string &query, const std::string &genreID, int sitePage) {
	if (!query.empty()) {
		std::string u = std::string(kBase) + "/?s=" + url_encode(query);
		if (sitePage > 1) u = std::string(kBase) + "/page/" + std::to_string(sitePage) + "/?s=" + url_encode(query);
		return u;
	}
	std::string u = std::string(kBase) + "/series/?page=" + std::to_string(sitePage);
	if (!genreID.empty()) u += "&genre%5B%5D=" + url_encode(genreID);
	return u;
}

// ─── Search ─────────────────────────────────────────────────────────────────

EXPORT char *Search(const char *json) {
	rapidjson::Document in;
	in.Parse(json ? json : "");
	if (in.HasParseError()) return error_json("bad_input");

	std::string query = trim(jstr(in, "query"));
	int page = jint(in, "page", 1);
	if (page < 1) page = 1;

	std::string genreID;
	if (auto it = in.FindMember("genres"); it != in.MemberEnd() && it->value.IsArray() && !it->value.Empty()) {
		if (it->value[0].IsString()) genreID = genre_id_for(it->value[0].GetString());
	}
	// Browse with neither a query nor a recognised genre: show the catalogue.
	if (query.empty() && genreID.empty()) genreID = "";

	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	std::vector<std::string> seen;

	// The host renders results[(page-1)*30 : page*30], so collect at least
	// `page*30` items. Site pages are smaller than a host page, hence the loop.
	int need = page * kHostPageSize;
	for (int sitePage = 1; sitePage <= 15 && (int)out.Size() < need; sitePage++) {
		std::string body;
		if (!get_body(site_url_for(query, genreID, sitePage), body)) break;
		size_t before = out.Size();
		parse_cards(body, out, alloc, seen);
		if (out.Size() == before) break; // no new cards: past the last page
	}
	return dup_c(document_json(out));
}

// ─── Detail ─────────────────────────────────────────────────────────────────

EXPORT char *GetMangaDetail(const char *json) {
	std::string slug = parse_string_arg(json);
	if (slug.empty()) return error_json("bad_input");

	rapidjson::Document out;
	out.SetObject();
	auto &alloc = out.GetAllocator();
	add_str(out, "id", slug, alloc);

	std::string body;
	if (!get_body(std::string(kBase) + "/series/" + url_encode(slug) + "/", body))
		return dup_c(document_json(out));

	add_str(out, "title", clean(first_sub(reTitleH1, body)), alloc);
	add_str(out, "cover_url", trim(first_sub(reOGImage, body)), alloc);
	add_str(out, "description", clean(first_sub(reDesc, body)), alloc);
	add_str(out, "author", clean(first_sub(reAuthor, body)), alloc);
	add_str(out, "status", normalize_status(first_sub(reStatusDot, body)), alloc);

	rapidjson::Value garr(rapidjson::kArrayType);
	std::vector<std::string> gseen;
	for (std::sregex_iterator it(body.begin(), body.end(), reGenre), end; it != end; ++it) {
		std::string name = clean((*it)[1].str());
		if (name.empty()) continue;
		bool dup = false;
		for (const auto &g : gseen)
			if (g == name) dup = true;
		if (dup) continue;
		gseen.push_back(name);
		garr.PushBack(rapidjson::Value(name.c_str(), (rapidjson::SizeType)name.size(), alloc), alloc);
	}
	out.AddMember("genres", garr, alloc);
	return dup_c(document_json(out));
}

// ─── Chapter list ───────────────────────────────────────────────────────────

EXPORT char *GetChapterList(const char *json) {
	std::string slug = parse_string_arg(json);
	if (slug.empty()) return error_json("bad_input");

	std::string body;
	if (!get_body(std::string(kBase) + "/series/" + url_encode(slug) + "/", body))
		return error_json("upstream_unavailable");

	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	std::vector<std::string> seen;
	for (std::sregex_iterator it(body.begin(), body.end(), reChapLi), end; it != end; ++it) {
		std::string num = (*it)[1].str();
		std::string li = (*it)[2].str();
		std::string href = first_sub(reHref, li);
		std::string cslug = slug_of(href);
		if (cslug.empty()) continue;
		bool dup = false;
		for (const auto &s : seen)
			if (s == cslug) dup = true;
		if (dup) continue;
		seen.push_back(cslug);

		rapidjson::Value ch(rapidjson::kObjectType);
		add_str(ch, "id", cslug, alloc);
		add_str(ch, "manga_id", slug, alloc);
		add_str(ch, "title", clean(first_sub(reChapNum, li)), alloc);
		add_num(ch, "chapter_num", atof(num.c_str()), alloc);
		add_str(ch, "released_at", parse_time(trim(first_sub(reChapDate, li))), alloc);
		add_str(ch, "url", href, alloc);
		out.PushBack(ch, alloc);
	}
	return dup_c(document_json(out));
}

// ─── Page list ──────────────────────────────────────────────────────────────

// reader_json extracts the single argument of ts_reader.run({...}) from a
// reader page. Brace matching skips string contents so a `}` inside the
// noimagehtml/url strings cannot end the object early.
static std::string reader_json(const std::string &body) {
	size_t at = body.find("ts_reader.run(");
	if (at == std::string::npos) return "";
	size_t i = body.find('{', at);
	if (i == std::string::npos) return "";
	int depth = 0;
	bool in_str = false, esc = false;
	for (size_t k = i; k < body.size(); k++) {
		char c = body[k];
		if (in_str) {
			if (esc) esc = false;
			else if (c == '\\') esc = true;
			else if (c == '"') in_str = false;
			continue;
		}
		if (c == '"') in_str = true;
		else if (c == '{') depth++;
		else if (c == '}' && --depth == 0) return body.substr(i, k - i + 1);
	}
	return "";
}

EXPORT char *GetPageList(const char *json) {
	std::string cslug = parse_string_arg(json);
	if (cslug.empty()) return error_json("bad_input");

	std::string body;
	if (!get_body(std::string(kBase) + "/" + url_encode(cslug) + "/", body))
		return error_json("upstream_unavailable");

	std::string raw = reader_json(body);
	if (raw.empty()) return error_json("no_pages");
	rapidjson::Document d;
	d.Parse(raw.c_str(), raw.size());
	if (d.HasParseError() || !d.IsObject()) return error_json("bad_upstream_response");

	auto src = d.FindMember("sources");
	if (src == d.MemberEnd() || !src->value.IsArray() || src->value.Empty()) return error_json("no_pages");
	auto imgs = src->value[0].FindMember("images");
	if (imgs == src->value[0].MemberEnd() || !imgs->value.IsArray()) return error_json("no_pages");

	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	int idx = 0;
	for (const auto &img : imgs->value.GetArray()) {
		if (!img.IsString()) continue;
		rapidjson::Value pg(rapidjson::kObjectType);
		pg.AddMember("index", idx++, alloc);
		add_str(pg, "url", img.GetString(), alloc);
		out.PushBack(pg, alloc);
	}
	if (idx == 0) return error_json("no_pages");
	return dup_c(document_json(out));
}

// ─── Genres ─────────────────────────────────────────────────────────────────

// GetGenres advertises the site's genre ids. slug carries the numeric id the
// /series filter expects, so Search can pass it straight through.
EXPORT char *GetGenres(const char *json) {
	(void)json;
	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	for (const auto &g : kGenres) {
		rapidjson::Value o(rapidjson::kObjectType);
		add_str(o, "name", g.name, alloc);
		add_str(o, "slug", g.id, alloc);
		out.PushBack(o, alloc);
	}
	return dup_c(document_json(out));
}

EXPORT void plugin_free(char *p) { free(p); }
