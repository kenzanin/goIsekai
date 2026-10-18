// roliascan — native .so plugin for goIsekai (C++17).
//
// Everything external goes through the host bridge (native_abi.h): HTTP, text
// stripping, the page's JSON-LD block and the chapter-list MD5 token. So the
// plugin links only header-only RapidJSON — no libcurl, libxml2 or OpenSSL —
// and the helper vocabulary matches the Lua/JS/WASM runtimes exactly.
//
// ABI (see internal/pluginmanager/goplug.go): each entry point takes a
// NUL-terminated JSON string and returns one. The returned buffer belongs to
// this library: the host copies it, then calls plugin_free on that pointer.
//
// Endpoints, cross-checked against roliascan.har:
//   POST /wp-json/manga/v1/load            search / browse
//   GET  /wp-json/wp/v2/manga/{id}?_embed  detail (terms embedded)
//   GET  /manga/{slug}/                    JSON-LD, the only place status lives
//   GET  /auth/manga-chapters              chapter list, needs the md5 _t token
//   GET  /auth/chapter-content             page images
#include "util.h"

#include <ctime>
#include <string>
#include <vector>

struct GenreTag {
	const char *name;
	int id;
};
#include "roliascan_genres.h"

static const char *kBase = "https://roliascan.com";
static const char *kJSONType = "Content-Type: application/json";

// The host proxy sends an empty Referer unless the plugin sets one, so every
// request here names the site explicitly (the older libcurl helper did the
// same through its referer argument).
static const char *kRefHeader = R"re({"Referer":"https://roliascan.com/"})re";
static const char *kRefJSONHeader = R"re({"Referer":"https://roliascan.com/","Content-Type":"application/json"})re";

static int genre_id_for(const std::string &name) {
	for (const auto &g : kGenres) {
		if (name == g.name) return g.id;
	}
	return 0;
}

EXPORT void plugin_free(char *p) { free(p); }

// Search maps types.SearchFilter onto the site's browse payload. The genres
// arrive as the slug names GetGenres advertised, and are resolved to the ids
// the API wants.
EXPORT char *Search(const char *json) {
	rapidjson::Document in;
	in.Parse(json ? json : "");
	if (in.HasParseError()) return error_json("bad_input");

	std::string query = jstr(in, "query");
	int page = jint(in, "page", 1);
	if (page < 1) page = 1;

	std::string sort = jstr(in, "sort_by");
	std::string site_sort = "post_desc";
	if (sort == "popular") site_sort = "popular_desc";
	else if (sort == "title") site_sort = "title_asc";
	else if (sort == "release") site_sort = "release_desc";

	// Genre ids travel as a stringified JSON array, e.g. "[\"5\",\"17\"]".
	std::string genre_ids = "[";
	if (auto it = in.FindMember("genres"); it != in.MemberEnd() && it->value.IsArray()) {
		bool first = true;
		for (const auto &g : it->value.GetArray()) {
			if (!g.IsString()) continue;
			int id = genre_id_for(g.GetString());
			if (id <= 0) continue;
			if (!first) genre_ids += ",";
			genre_ids += "\"" + std::to_string(id) + "\"";
			first = false;
		}
	}
	genre_ids += "]";

	rapidjson::StringBuffer sb;
	rapidjson::Writer<rapidjson::StringBuffer> w(sb);
	w.StartObject();
	w.Key("page");
	w.Int(page);
	w.Key("search");
	w.String(query.c_str());
	w.Key("years");
	w.String("[]");
	w.Key("genres");
	w.String(genre_ids.c_str());
	w.Key("types");
	w.String("[]");
	w.Key("statuses");
	w.String("[]");
	w.Key("sort");
	w.String(site_sort.c_str());
	w.Key("genreMatchMode");
	w.String("any");
	w.EndObject();

	HttpResult r = http_post(std::string(kBase) + "/wp-json/manga/v1/load", sb.GetString(), kRefJSONHeader);
	if (!r.ok()) return error_json(r.status == 404 ? "not_found" : "upstream_unavailable");

	rapidjson::Document doc;
	doc.Parse(r.body.c_str());
	if (doc.HasParseError() || !doc.IsArray()) return error_json("bad_upstream_response");

	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	for (const auto &m : doc.GetArray()) {
		// Novels share the catalogue but have no readable pages.
		if (jstr(m, "type") == "Novel") continue;
		if (jstr(m, "url").empty()) continue;
		rapidjson::Value o(rapidjson::kObjectType);
		add_str(o, "id", jstr(m, "id"), alloc);
		add_str(o, "title", jstr(m, "title"), alloc);
		add_str(o, "cover_url", jstr(m, "cover"), alloc);
		add_str(o, "description", jstr(m, "description"), alloc);
		add_str(o, "status", jstr(m, "status"), alloc);
		out.PushBack(o, alloc);
	}
	return document_json(out);
}

// GetMangaDetail reads the WordPress manga post with _embed, so tags and the
// author taxonomy arrive in one response. Status is the exception: the REST
// payload has no such field, so it comes from the page's JSON-LD.
EXPORT char *GetMangaDetail(const char *json) {
	std::string id = parse_string_arg(json);
	if (id.empty()) return error_json("bad_input");

	HttpResult r = http_get(std::string(kBase) + "/wp-json/wp/v2/manga/" + id + "?_embed", kRefHeader);
	if (!r.ok()) return error_json("not_found");

	rapidjson::Document d;
	d.Parse(r.body.c_str());
	if (d.HasParseError() || !d.IsObject()) return error_json("bad_upstream_response");

	rapidjson::Document out;
	out.SetObject();
	auto &alloc = out.GetAllocator();
	add_str(out, "id", id, alloc);

	std::string title;
	if (auto it = d.FindMember("title"); it != d.MemberEnd() && it->value.IsObject())
		title = jstr(it->value, "rendered");
	std::string plain = clean(title);
	add_str(out, "title", plain.empty() ? title : plain, alloc);

	std::string content;
	if (auto it = d.FindMember("content"); it != d.MemberEnd() && it->value.IsObject())
		content = jstr(it->value, "rendered");
	add_str(out, "description", clean(content), alloc);

	std::string cover;
	std::vector<std::string> genres, authors;
	if (auto emb = d.FindMember("_embedded"); emb != d.MemberEnd() && emb->value.IsObject()) {
		auto &e = emb->value;
		if (auto fm = e.FindMember("wp:featuredmedia");
		    fm != e.MemberEnd() && fm->value.IsArray() && !fm->value.Empty() && fm->value[0].IsObject()) {
			cover = jstr(fm->value[0], "source_url");
			if (cover.empty()) cover = jstr(fm->value[0], "url");
		}
		if (auto terms = e.FindMember("wp:term"); terms != e.MemberEnd() && terms->value.IsArray()) {
			for (const auto &group : terms->value.GetArray()) {
				if (!group.IsArray()) continue;
				for (const auto &t : group.GetArray()) {
					if (!t.IsObject()) continue;
					std::string tax = jstr(t, "taxonomy");
					std::string name = jstr(t, "name");
					if (name.empty()) continue;
					if (tax == "post_tag") genres.push_back(name);
					else if (tax == "manga_author") authors.push_back(name);
				}
			}
		}
	}
	add_str(out, "cover_url", cover, alloc);

	std::string author;
	for (size_t i = 0; i < authors.size(); i++) {
		if (i) author += ", ";
		author += authors[i];
	}
	add_str(out, "author", author, alloc);

	// The type doubles as a genre when no format tag was set (mirrors the
	// reference extension, which injects it when Manga/Manhwa/Manhua is absent).
	std::string type;
	if (auto it = d.FindMember("type"); it != d.MemberEnd() && it->value.IsString()) type = it->value.GetString();
	bool has_fmt = false;
	for (const auto &g : genres)
		if (g == "Manga" || g == "Manhwa" || g == "Manhua") has_fmt = true;
	if (!has_fmt && (type == "Manga" || type == "Manhwa" || type == "Manhua")) genres.push_back(type);

	rapidjson::Value garr(rapidjson::kArrayType);
	for (const auto &g : genres) garr.PushBack(rapidjson::Value(g.c_str(), (rapidjson::SizeType)g.size(), alloc), alloc);
	out.AddMember("genres", garr, alloc);

	std::string slug;
	if (auto it = d.FindMember("slug"); it != d.MemberEnd() && it->value.IsString()) slug = it->value.GetString();
	std::string page_url = slug.empty() ? "" : std::string(kBase) + "/manga/" + slug + "/";
	add_str(out, "url", page_url, alloc);

	// A detail without a status is still useful, so a failed page fetch degrades
	// to an empty status instead of failing the whole call.
	std::string status;
	if (!page_url.empty()) {
		HttpResult page = http_get(page_url, kRefHeader);
		if (page.ok()) {
			std::string blob = host_str2("text.json_blob", page.body, "application/ld+json");
			rapidjson::Document ld;
			ld.Parse(blob.c_str());
			if (!ld.HasParseError() && ld.IsObject()) status = jstr(ld, "status");
		}
	}
	add_str(out, "status", status, alloc);

	return document_json(out);
}

// GetChapterList needs the site's signed token: md5("<ts>mng_ch_<utc yyyyMMddHH>")
// truncated to 16 hex chars, sent alongside the same ts.
EXPORT char *GetChapterList(const char *json) {
	std::string id = parse_string_arg(json);
	if (id.empty()) return error_json("bad_input");

	time_t now = time(nullptr);
	long ts = (long)now;
	struct tm tm;
	gmtime_r(&now, &tm);
	char hour[16];
	strftime(hour, sizeof hour, "%Y%m%d%H", &tm);

	std::string token = host_str("crypto.md5_hex", std::to_string(ts) + "mng_ch_" + hour).substr(0, 16);
	std::string url = std::string(kBase) + "/auth/manga-chapters?manga_id=" + id +
	                  "&offset=0&limit=9999&order=DESC&_t=" + token + "&_ts=" + std::to_string(ts);

	HttpResult r = http_get(url, kRefHeader);
	if (!r.ok()) return error_json("upstream_unavailable");

	rapidjson::Document d;
	d.Parse(r.body.c_str());
	if (d.HasParseError() || !d.IsObject()) return error_json("bad_upstream_response");
	auto it = d.FindMember("chapters");
	if (it == d.MemberEnd() || !it->value.IsArray()) return error_json("bad_upstream_response");

	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	for (const auto &c : it->value.GetArray()) {
		// Only English releases; the site mixes scanlation groups and languages.
		std::string lang = jstr(c, "language");
		if (!lang.empty() && lang != "en") continue;

		std::string num = jstr(c, "chapter");
		std::string label = jstr(c, "title");
		std::string title = "Chapter " + num;
		if (!label.empty() && label != "N/A" && label != "-") title += ": " + clean(label);

		rapidjson::Value o(rapidjson::kObjectType);
		add_str(o, "id", jstr(c, "id"), alloc);
		add_str(o, "manga_id", id, alloc);
		add_str(o, "title", title, alloc);
		// chapter_num is a JSON number in types.Chapter, not a string.
		o.AddMember("chapter_num", rapidjson::Value(strtod(num.c_str(), nullptr)), alloc);
		std::string iso = parse_time(jstr(c, "date"));
		if (!iso.empty()) add_str(o, "released_at", iso, alloc);
		add_str(o, "url", jstr(c, "url"), alloc);
		out.PushBack(o, alloc);
	}
	return document_json(out);
}

// GetPageList takes the chapter id returned above and asks the content
// endpoint for its image list.
EXPORT char *GetPageList(const char *json) {
	std::string id = parse_string_arg(json);
	if (id.empty()) return error_json("bad_input");

	HttpResult r = http_get(std::string(kBase) + "/auth/chapter-content?chapter_id=" + id, kRefHeader);
	if (!r.ok()) return error_json("upstream_unavailable");

	rapidjson::Document d;
	d.Parse(r.body.c_str());
	if (d.HasParseError() || !d.IsObject()) return error_json("bad_upstream_response");
	auto it = d.FindMember("images");
	if (it == d.MemberEnd() || !it->value.IsArray()) return error_json("no_pages");

	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	int idx = 0;
	for (const auto &img : it->value.GetArray()) {
		if (!img.IsString()) continue;
		rapidjson::Value o(rapidjson::kObjectType);
		o.AddMember("index", idx++, alloc);
		add_str(o, "url", img.GetString(), alloc);
		out.PushBack(o, alloc);
	}
	if (idx == 0) return error_json("no_pages");
	return document_json(out);
}

// GetGenres advertises the tag catalogue. slug doubles as the tag name, which
// Search maps back to the numeric id the site expects.
EXPORT char *GetGenres(const char *json) {
	(void)json;
	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	for (const auto &g : kGenres) {
		rapidjson::Value o(rapidjson::kObjectType);
		add_str(o, "name", g.name, alloc);
		add_str(o, "slug", g.name, alloc);
		out.PushBack(o, alloc);
	}
	return document_json(out);
}
