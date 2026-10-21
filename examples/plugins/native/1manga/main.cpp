// 1manga native plugin for goIsekai (C++17).
//
// Site: https://1manga.co — MangaHub GraphQL API at api.mghcdn.com
// Source id mn03. The API answers 404 without Origin/Referer on the POST;
// mhub_access is only needed by the chapter (pages) resolver.
//
// This is the native port of examples/plugins/lua/1manga/main.lua, which stays
// in the archive as the reference implementation. Behaviour matches it, so the
// two can be A/B tested against the live site.
//
// ABI (see internal/pluginmanager/goplug.go): each entry point takes a
// NUL-terminated JSON string and returns one. The returned buffer belongs to
// this library: the host copies it, then calls plugin_free on that pointer.
//
// Everything external goes through the host bridge (native_abi.h): HTTP, JSON,
// text helpers and the AES-256-GCM page decryption. So the plugin links only
// header-only RapidJSON — no libcurl, libxml2 or OpenSSL.
#include "util.h"

#include <string>
#include <vector>

static const char *kGraphQL = "https://api.mghcdn.com/graphql";
static const char *kSite = "https://1manga.co";
static const char *kSourceID = "mn03";
static const char *kImgCDN = "https://imgx.mghcdn.com/";
static const char *kThumbCDN = "https://thumb.mghcdn.com/";

// Cached access key, dropped whenever the API refuses.
static std::string g_cached_key;

// Last refusal message, so callers surface it instead of a silent empty list.
static std::string g_gql_error;

// ─── helpers ───────────────────────────────────────────────────────────────

// Escape a string for embedding in a GraphQL string literal.
static std::string escape_gql(const std::string &s) {
	std::string out;
	for (char c : s) {
		if (c == '\\' || c == '"') out += '\\';
		out += c;
	}
	return out;
}

// json_escape escapes a string for embedding in a JSON string literal (the
// header values below are hand-built JSON, so they need this).
static std::string json_escape(const std::string &s) {
	std::string out;
	for (char c : s) {
		if (c == '"' || c == '\\') out += '\\';
		out += c;
	}
	return out;
}

// A bare GET only hands back the key this session/IP already exhausted, so
// every chapter comes back rate limited. Presenting a cookie value the server
// has never issued makes it mint a genuinely new key, and ?reloadKey=1 is what
// triggers the re-issue. Each key so obtained carries four chapters of quota,
// and an explicit Cookie header overrides whatever the host cookie jar holds.
static std::string fetch_access_key() {
	// The Lua original randomises the chapter number; the number only has to
	// name a chapter page that exists, so a counter serves the same purpose
	// without pulling in a RNG.
	static int counter = 1000;
	counter = 1000 + (counter % 2000);

	std::string url = std::string(kSite) + "/chapter/martial-peak/chapter-" + std::to_string(counter++) +
					  "?reloadKey=1";
	std::string headers = std::string("{\"Referer\":\"") + kSite +
						  "/manga/martial-peak\""
						  ",\"Accept\":\"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\""
						  ",\"Sec-Fetch-Dest\":\"document\",\"Sec-Fetch-Mode\":\"navigate\""
						  ",\"Sec-Fetch-Site\":\"same-origin\",\"Upgrade-Insecure-Requests\":\"1\""
						  ",\"Cookie\":\"mhub_access=0000000000000000000000000000cafe\"}";

	HttpResult r = http_get(url, headers);
	if (r.status == 0) return "";

	// host.regex.find returns the first capture group as a plain string.
	std::string cookie = r.header("Set-Cookie");
	if (cookie.empty()) return "";
	rapidjson::Document d;
	if (!host_json("regex.find", {cookie, "mhub_access=([^;]+)"}, d)) return "";
	if (!d.IsString()) return "";
	std::string found = d.GetString();
	return found;
}

// Run a GraphQL query, dropping the cached key and retrying once when the API
// refuses. MangaHub reports both expired keys and its "API rate limit
// excessed" refusal as HTTP 200 with an errors array and a null payload, so
// the status code alone cannot tell a good answer from a refused one. The
// message is kept in g_gql_error so callers can surface it.
//
// Returns true and fills out with the parsed response document on success.
static bool graphql_query(const std::string &query, rapidjson::Document &out) {
	g_gql_error.clear();

	for (int attempt = 0; attempt < 2; attempt++) {
		std::string key = g_cached_key;
		if (key.empty()) {
			key = fetch_access_key();
			if (!key.empty()) g_cached_key = key;
		}
		if (key.empty()) {
			host_str("log.error", "1manga: no mhub_access key available");
			return false;
		}

		// Body: {"query":"..."} — the same shape host.json.encode produces.
		rapidjson::Document body;
		body.SetObject();
		auto &alloc = body.GetAllocator();
		body.AddMember("query", rapidjson::Value(query.c_str(), (rapidjson::SizeType)query.size(), alloc), alloc);
		rapidjson::StringBuffer sb;
		rapidjson::Writer<rapidjson::StringBuffer> w(sb);
		body.Accept(w);

		std::string headers = std::string("{\"Content-Type\":\"application/json\",\"Origin\":\"") + kSite +
							  "\",\"Referer\":\"" + kSite + "/\",\"x-mhub-access\":\"" + json_escape(key) + "\"}";

		HttpResult r = http_post(kGraphQL, sb.GetString(), headers);
		if (!r.ok()) {
			host_str("log.info", "1manga: GraphQL status " + std::to_string(r.status) + ", refreshing key");
			g_cached_key.clear();
			continue;
		}

		rapidjson::Document parsed;
		parsed.Parse(r.body.c_str(), r.body.size());
		if (parsed.HasParseError()) {
			host_str("log.error", "1manga: unparseable GraphQL response");
			return false;
		}

		const rapidjson::Value *errors = member(parsed, "errors");
		const rapidjson::Value *data = member(parsed, "data");
		if (errors && errors->IsArray() && errors->Size() > 0) {
			g_gql_error = jstr(errors->GetArray()[0], "message");
			if (g_gql_error.empty()) g_gql_error = "GraphQL error";
		} else if (!data || data->IsNull()) {
			g_gql_error = "GraphQL response carried no data";
		} else {
			out.Swap(parsed);
			return true;
		}

		host_str("log.warn", "1manga: GraphQL refused (" + g_gql_error + "), retrying with a fresh key");
		g_cached_key.clear();
	}

	return false;
}

// ─── ABI: Search ────────────────────────────────────────────────────────────
// arg: {"query":"...","page":1,"genres":[...]}
EXPORT char *Search(const char *json) {
	rapidjson::Document in;
	in.Parse(json ? json : "");
	if (in.HasParseError()) return error_json("bad_input");

	std::string query = jstr(in, "query");
	int page = jint(in, "page", 1);
	if (page < 1) page = 1;
	int offset = (page - 1) * 30;

	host_str("log.info", "1manga search: q=" + query + " page=" + std::to_string(page));

	// GraphQL takes one genre name per call; multi-genre means intersecting
	// result sets, which one query cannot express. First genre wins.
	std::string genre = "all";
	if (auto it = in.FindMember("genres"); it != in.MemberEnd() && it->value.IsArray() && it->value.Size() > 0) {
		const rapidjson::Value &g = it->value.GetArray()[0];
		if (g.IsString() && g.GetString()[0] != '\0') genre = g.GetString();
	}

	std::string gql = std::string("{search(x: ") + kSourceID + ", q: \"" + escape_gql(query) + "\", genre: \"" +
					  escape_gql(genre) + "\", mod: POPULAR, offset: " + std::to_string(offset) +
					  ") {rows {title, slug, image}}}";

	rapidjson::Document doc;
	bool got = graphql_query(gql, doc);

	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	if (got) {
		const rapidjson::Value *data = member(doc, "data");
		const rapidjson::Value *search = data ? member(*data, "search") : nullptr;
		const rapidjson::Value *rows = search ? member(*search, "rows") : nullptr;
		if (rows && rows->IsArray()) {
			for (const auto &row : rows->GetArray()) {
				std::string cover = jstr(row, "image");
				if (!cover.empty() && cover.compare(0, 4, "http") != 0) cover = std::string(kThumbCDN) + cover;
				rapidjson::Value o(rapidjson::kObjectType);
				add_str(o, "id", jstr(row, "slug"), alloc);
				add_str(o, "title", jstr(row, "title"), alloc);
				add_str(o, "cover_url", cover, alloc);
				out.PushBack(o, alloc);
			}
		}
	}

	host_str("log.info", "1manga search: found " + std::to_string(out.Size()) + " results for q=" + query);
	return document_json(out);
}

// ─── ABI: GetMangaDetail ────────────────────────────────────────────────────
// arg: '"slug"'. A failed lookup still returns an object: an empty array
// cannot decode into a detail record.
EXPORT char *GetMangaDetail(const char *json) {
	std::string slug = parse_string_arg(json);
	if (slug.empty()) return error_json("bad_input");

	host_str("log.info", "1manga detail: slug=" + slug);

	std::string gql = std::string("{manga(x: ") + kSourceID + ", slug: \"" + escape_gql(slug) +
					  "\") {title, slug, status, image, author, artist, genres, description, alternativeTitle}}";

	rapidjson::Document doc;
	bool got = graphql_query(gql, doc);
	const rapidjson::Value *m = nullptr;
	if (got) {
		const rapidjson::Value *data = member(doc, "data");
		const rapidjson::Value *node = data ? member(*data, "manga") : nullptr;
		if (node && !node->IsNull()) m = node;
	}

	rapidjson::Document out;
	out.SetObject();
	auto &alloc = out.GetAllocator();
	if (!m) {
		add_str(out, "id", slug, alloc);
		return document_json(out);
	}

	std::string cover = jstr(*m, "image");
	if (!cover.empty() && cover.compare(0, 4, "http") != 0) cover = std::string(kThumbCDN) + cover;

	// status may arrive as an enum string or as a numeric code; the host wants
	// a string either way.
	std::string status = jnum_str(*m, "status");

	std::string id = jstr(*m, "slug");
	if (id.empty()) id = slug;

	add_str(out, "id", id, alloc);
	add_str(out, "title", jstr(*m, "title"), alloc);
	add_str(out, "cover_url", cover, alloc);
	add_str(out, "author", jstr(*m, "author"), alloc);
	add_str(out, "description", jstr(*m, "description"), alloc);
	add_str(out, "status", host_str("text.normalize_status", status), alloc);

	// genres arrives as one comma-separated string; the ABI wants a list.
	const rapidjson::Value *genres = member(*m, "genres");
	rapidjson::Value glist(rapidjson::kArrayType);
	if (genres && genres->IsString()) {
		// Split on commas and trim each part, like host.regex.gmatch over
		// "[^,]+" plus host.text.trim.
		const std::string s = genres->GetString();
		size_t pos = 0;
		while (pos <= s.size()) {
			size_t comma = s.find(',', pos);
			std::string part = s.substr(pos, comma == std::string::npos ? std::string::npos : comma - pos);
			std::string name = trim(part);
			if (!name.empty()) {
				glist.PushBack(rapidjson::Value(name.c_str(), (rapidjson::SizeType)name.size(), alloc), alloc);
			}
			if (comma == std::string::npos) break;
			pos = comma + 1;
		}
	} else if (genres && genres->IsArray()) {
		for (const auto &g : genres->GetArray()) {
			if (!g.IsString()) continue;
			glist.PushBack(rapidjson::Value(g.GetString(), (rapidjson::SizeType)strlen(g.GetString()), alloc), alloc);
		}
	}
	out.AddMember("genres", glist, alloc);

	return document_json(out);
}

// ─── ABI: GetChapterList ────────────────────────────────────────────────────
// arg: '"slug"'
EXPORT char *GetChapterList(const char *json) {
	std::string slug = parse_string_arg(json);
	if (slug.empty()) return error_json("bad_input");

	host_str("log.info", "1manga chapters: slug=" + slug);

	std::string gql = std::string("{manga(x: ") + kSourceID + ", slug: \"" + escape_gql(slug) +
					  "\") {chapters {number, title, date}}}";

	rapidjson::Document doc;
	bool got = graphql_query(gql, doc);

	struct Chapter {
		std::string id;
		std::string number;
		std::string title;
		double num = 0;
		std::string released;
	};
	std::vector<Chapter> chapters;

	if (got) {
		const rapidjson::Value *data = member(doc, "data");
		const rapidjson::Value *m = data ? member(*data, "manga") : nullptr;
		const rapidjson::Value *raw = m ? member(*m, "chapters") : nullptr;
		if (raw && raw->IsArray()) {
			for (const auto &ch : raw->GetArray()) {
				// An absent title arrives as an empty string, not nil.
				std::string title = jstr(ch, "title");
				std::string number = jnum_str(ch, "number");
				if (title.empty()) title = "Chapter " + number;

				Chapter c;
				c.number = number;
				c.id = slug + ":chapter-" + number;
				c.title = title;
				c.num = host_num("text.chapter_num", number);
				// MangaHub sends epoch seconds, epoch millis or a date string;
				// the host normalizes all three. An unparseable date leaves the
				// key out so Go keeps its zero value.
				c.released = parse_time(jnum_str(ch, "date"));
				chapters.push_back(c);
			}
		}
	}

	// GraphQL returns ascending; the ABI wants newest-first.
	std::stable_sort(chapters.begin(), chapters.end(),
					 [](const Chapter &a, const Chapter &b) { return a.num > b.num; });

	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	for (const auto &c : chapters) {
		rapidjson::Value o(rapidjson::kObjectType);
		add_str(o, "id", c.id, alloc);
		add_str(o, "manga_id", slug, alloc);
		add_str(o, "title", c.title, alloc);
		add_num(o, "chapter_num", c.num, alloc);
		add_str(o, "url", std::string(kSite) + "/chapter/" + slug + "/chapter-" + c.number, alloc);
		// An empty released_at would fail the time.Time decode, so the field
		// is left out entirely when the date did not parse.
		if (!c.released.empty()) add_str(o, "released_at", c.released, alloc);
		out.PushBack(o, alloc);
	}

	host_str("log.info", "1manga chapters: found " + std::to_string(chapters.size()) + " chapters for " + slug);
	return document_json(out);
}

// ─── ABI: GetPageList ───────────────────────────────────────────────────────
// arg: '"slug:chapter-NUMBER"'
EXPORT char *GetPageList(const char *json) {
	std::string chapter_id = parse_string_arg(json);
	if (chapter_id.empty()) return error_json("bad_input");

	// Split on the first ":chapter-" — the slug cannot contain one, the tail can.
	size_t sep = chapter_id.find(":chapter-");
	if (sep == std::string::npos) {
		rapidjson::Document out;
		out.SetArray();
		return document_json(out);
	}
	std::string slug = chapter_id.substr(0, sep);
	std::string number = chapter_id.substr(sep + 9);
	host_str("log.info", "1manga pages: slug=" + slug + " ch=" + number);

	std::string gql = std::string("{chapter(x: ") + kSourceID + ", slug: \"" + escape_gql(slug) +
					  "\", number: " + number + ") {pages, mangaID, number}}";

	rapidjson::Document doc;
	bool got = graphql_query(gql, doc);

	const rapidjson::Value *chapter = nullptr;
	if (got) {
		const rapidjson::Value *data = member(doc, "data");
		const rapidjson::Value *c = data ? member(*data, "chapter") : nullptr;
		if (c && !c->IsNull()) chapter = c;
	}
	if (!chapter) {
		// A refusal (rate limit, expired key) must fail loudly: returning an
		// empty list renders a blank chapter with no explanation.
		if (!g_gql_error.empty()) {
			host_str("log.error", "1manga: graphql refused: " + g_gql_error);
			return error_json("upstream_auth_failed");
		}
		rapidjson::Document out;
		out.SetArray();
		return document_json(out);
	}

	const rapidjson::Value *pages_field = member(*chapter, "pages");
	std::string blob = pages_field && pages_field->IsString() ? pages_field->GetString() : "";

	// Authoritative path: the encrypted page list the API hands back. It names
	// every file exactly, which the CDN pattern below cannot do (chapter 59 of
	// this manga is "1a.jpg 2a.jpg 3a.jpg 4.jpg ..."), so both the extension
	// probe and the binary search fail on it.
	//
	// MangaHub encrypts the page list and hands the key out separately:
	//   pages = "enc:v1:<keyId>:<iv>:<tag>:<ciphertext>"  (all base64url)
	//   key   = GET /api/chapter-crypto -> {keyId, key, expiresAt}
	//   plain = AES-256-GCM(key, iv, ciphertext+tag) -> {"p":"<prefix>","i":[...]}
	// Decryption runs in the host (crypto.aes_gcm_decrypt).
	if (blob.compare(0, 7, "enc:v1:") == 0) {
		std::vector<std::string> parts;
		size_t pos = 0;
		while (true) {
			size_t colon = blob.find(':', pos);
			if (colon == std::string::npos) {
				parts.push_back(blob.substr(pos));
				break;
			}
			parts.push_back(blob.substr(pos, colon - pos));
			pos = colon + 1;
		}
		if (parts.size() != 6 || parts[0] != "enc" || parts[1] != "v1") {
			host_str("log.error", "1manga: unrecognised pages envelope");
			return error_json("envelope_unrecognised");
		}
		const std::string key_id = parts[2];
		const std::string &iv = parts[3];
		const std::string &tag = parts[4];
		const std::string &ciphertext = parts[5];

		rapidjson::Document pages_doc;

		// One attempt: mint a key, fetch the crypto material and decrypt.
		// The key and the message rotate independently, so a keyId mismatch is
		// retried once with a fresh key rather than treated as a failure.
		auto attempt = [&]() -> bool {
			std::string key = g_cached_key;
			if (key.empty()) {
				key = fetch_access_key();
				if (!key.empty()) g_cached_key = key;
			}
			if (key.empty()) return false;

			// This endpoint authenticates on the cookie, not the x-mhub-access
			// header the GraphQL call uses: the header alone answers 403.
			std::string headers = std::string("{\"Cookie\":\"mhub_access=") + json_escape(key) +
								  "\",\"Accept\":\"application/json\",\"Referer\":\"" + kSite + "/chapter/" +
								  json_escape(slug) + "/chapter-" + json_escape(number) + "\"}";
			HttpResult r = http_get(std::string(kSite) + "/api/chapter-crypto", headers);
			if (r.status != 200) {
				host_str("log.error", "1manga: chapter-crypto status " + std::to_string(r.status));
				return false;
			}
			rapidjson::Document d;
			d.Parse(r.body.c_str(), r.body.size());
			if (d.HasParseError() || !d.IsObject()) return false;
			std::string got_key = jstr(d, "key");
			std::string got_id = jnum_str(d, "keyId");
			if (got_key.empty() || got_id.empty()) return false;
			if (got_id != key_id) return false;

			rapidjson::Document plain_doc;
			if (!host_json("crypto.aes_gcm_decrypt", {got_key, iv, tag, ciphertext}, plain_doc)) {
				host_str("log.error", "1manga: pages decrypt failed");
				return false;
			}
			if (!plain_doc.IsString()) return false;
			std::string plain = plain_doc.GetString();

			rapidjson::Document decoded;
			decoded.Parse(plain.c_str(), plain.size());
			if (decoded.HasParseError() || !decoded.IsObject()) {
				host_str("log.error", "1manga: unexpected pages plaintext");
				return false;
			}
			const rapidjson::Value *prefix = member(decoded, "p");
			const rapidjson::Value *names = member(decoded, "i");
			if (!prefix || !prefix->IsString() || !names || !names->IsArray()) {
				host_str("log.error", "1manga: unexpected pages plaintext");
				return false;
			}

			pages_doc.SetArray();
			auto &alloc = pages_doc.GetAllocator();
			int idx = 0;
			for (const auto &name : names->GetArray()) {
				if (!name.IsString()) continue;
				rapidjson::Value o(rapidjson::kObjectType);
				add_num(o, "index", idx, alloc);
				add_str(o, "url", std::string(kImgCDN) + prefix->GetString() + name.GetString(), alloc);
				pages_doc.PushBack(o, alloc);
				idx++;
			}
			return true;
		};

		bool decrypted = attempt();
		if (!decrypted) {
			// The session's key may simply have rotated since the message was
			// encrypted, so ask for a new one and try once more.
			g_cached_key.clear();
			decrypted = attempt();
		}
		if (!decrypted) return error_json("decrypt_key_mismatch");

		host_str("log.info", "1manga pages: found " + std::to_string(pages_doc.Size()) + " pages for " +
								  chapter_id + " (decrypted)");
		return document_json(pages_doc);
	}

	// Fallback for a plaintext or unreadable pages field: find the page count
	// by probing (page N exists with 200, N+1 404s). Binary search keeps it to
	// ~9 cheap requests; the list lands in the chapters_pages cache afterward,
	// so the cost is paid once per chapter. This assumes contiguous page
	// numbers, which is exactly what the encrypted path above exists to avoid.
	// The extension varies per chapter (older ones .jpg, newer ones .jpeg), so
	// probe candidates once on page 1 instead of hardcoding one.
	std::string referer = std::string("{\"Referer\":\"") + kSite + "/\"}";
	std::string ext;
	for (const char *c : {"jpg", "jpeg", "png", "webp"}) {
		HttpResult r = http_get(std::string(kImgCDN) + slug + "/" + number + "/1." + c, referer);
		if (r.status == 200) {
			ext = c;
			break;
		}
	}
	if (ext.empty()) {
		host_str("log.error", "1manga pages: page 1 missing for " + chapter_id);
		rapidjson::Document out;
		out.SetArray();
		return document_json(out);
	}

	auto page_exists = [&](int i) {
		HttpResult r =
			http_get(std::string(kImgCDN) + slug + "/" + number + "/" + std::to_string(i) + "." + ext, referer);
		return r.status == 200;
	};

	int lo = 1, hi = 200;
	while (lo < hi) {
		int mid = (lo + hi + 1) / 2;
		if (page_exists(mid)) {
			lo = mid;
		} else {
			hi = mid - 1;
		}
	}

	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	for (int i = 1; i <= lo; i++) {
		rapidjson::Value o(rapidjson::kObjectType);
		add_num(o, "index", i - 1, alloc);
		add_str(o, "url", std::string(kImgCDN) + slug + "/" + number + "/" + std::to_string(i) + "." + ext, alloc);
		out.PushBack(o, alloc);
	}

	host_str("log.info", "1manga pages: found " + std::to_string(out.Size()) + " pages for " + chapter_id);
	return document_json(out);
}

// ─── GetGenres (optional export) ────────────────────────────────────────────
// MangaHub GraphQL takes genre names directly (genre: "Action"); the list
// below mirrors the site's genre facet.
struct Genre {
	const char *name;
	const char *slug;
};

static const Genre kGenres[] = {
	{"Action", "Action"},
	{"Adventure", "Adventure"},
	{"Comedy", "Comedy"},
	{"Cooking", "Cooking"},
	{"Doujinshi", "Doujinshi"},
	{"Drama", "Drama"},
	{"Fantasy", "Fantasy"},
	{"Gender bender", "Gender Bender"},
	{"Harem", "Harem"},
	{"Historical", "Historical"},
	{"Horror", "Horror"},
	{"Isekai", "Isekai"},
	{"Josei", "Josei"},
	{"Martial arts", "Martial Arts"},
	{"Mecha", "Mecha"},
	{"Mystery", "Mystery"},
	{"One shot", "One Shot"},
	{"Psychological", "Psychological"},
	{"Romance", "Romance"},
	{"School life", "School Life"},
	{"Sci-fi", "Sci Fi"},
	{"Seinen", "Seinen"},
	{"Shoujo", "Shoujo"},
	{"Shounen", "Shounen"},
	{"Slice of life", "Slice of Life"},
	{"Sports", "Sports"},
	{"Supernatural", "Supernatural"},
	{"Tragedy", "Tragedy"},
	{"Webtoons", "Webtoons"},
};

EXPORT char *GetGenres(const char *json) {
	(void)json;
	rapidjson::Document out;
	out.SetArray();
	auto &alloc = out.GetAllocator();
	for (const auto &g : kGenres) {
		rapidjson::Value o(rapidjson::kObjectType);
		add_str(o, "name", g.name, alloc);
		add_str(o, "slug", g.slug, alloc);
		out.PushBack(o, alloc);
	}
	return document_json(out);
}

// plugin_free releases a buffer the plugin handed to the host.
EXPORT void plugin_free(char *p) { free(p); }
