// util.h — shared helpers for the roliascan native plugin.
//
// Every helper is either a thin wrapper over a host bridge helper (so the
// vocabulary cannot drift from the Lua/JS/WASM runtimes) or a small JSON/HTTP
// convenience. Depends on native_abi.h (RapidJSON + the host_call bridge).
#pragma once

#include <algorithm>
#include <cctype>
#include <cstdlib>
#include <cstring>
#include <string>
#include <vector>

#include "native_abi.h"

// EXPORT marks an entry point the host looks up by name. The build hides every
// other symbol (-fvisibility=hidden), so this has to stay exported.
#define EXPORT extern "C" __attribute__((visibility("default")))

// ─── JSON output ────────────────────────────────────────────────────────────

// dup_c hands a std::string to the host as a malloc'd NUL-terminated buffer.
// The plugin owns every buffer it returns; the host copies it, then calls
// plugin_free on exactly that pointer.
static char *dup_c(const std::string &s) {
	char *p = (char *)malloc(s.size() + 1);
	if (!p) return nullptr;
	memcpy(p, s.c_str(), s.size() + 1);
	return p;
}

// error_json is the shared failure envelope: {"error":"code"}.
static char *error_json(const char *code) {
	return dup_c(std::string("{\"error\":\"") + code + "\"}");
}

static char *document_json(const rapidjson::Document &d) {
	rapidjson::StringBuffer sb;
	rapidjson::Writer<rapidjson::StringBuffer> w(sb);
	d.Accept(w);
	return dup_c(sb.GetString());
}

static void add_str(rapidjson::Value &obj, const char *k, const std::string &v,
                    rapidjson::Document::AllocatorType &a) {
	obj.AddMember(rapidjson::Value(k, (rapidjson::SizeType)strlen(k), a),
	              rapidjson::Value(v.c_str(), (rapidjson::SizeType)v.size(), a), a);
}

static std::string jstr(const rapidjson::Value &obj, const char *k) {
	if (!obj.IsObject()) return "";
	auto it = obj.FindMember(k);
	if (it == obj.MemberEnd() || !it->value.IsString()) return "";
	return it->value.GetString();
}

static int jint(const rapidjson::Value &obj, const char *k, int def) {
	if (!obj.IsObject()) return def;
	auto it = obj.FindMember(k);
	if (it == obj.MemberEnd() || !it->value.IsInt()) return def;
	return it->value.GetInt();
}

// parse_string_arg unwraps the ABI argument for the calls whose input is a bare
// JSON string (a manga id, a chapter id) rather than an object.
static std::string parse_string_arg(const char *json) {
	rapidjson::Document d;
	d.Parse(json ? json : "");
	return d.IsString() ? d.GetString() : "";
}

// host_str2 is host_str for the two-argument helpers (text.json_blob).
static std::string host_str2(const char *fn, const std::string &a, const std::string &b) {
	rapidjson::Document d;
	if (!host_json(fn, {a, b}, d)) return "";
	return d.IsString() ? d.GetString() : "";
}

// ─── text ───────────────────────────────────────────────────────────────────

static std::string trim(const std::string &s) {
	size_t a = s.find_first_not_of(" \t\r\n");
	if (a == std::string::npos) return "";
	size_t b = s.find_last_not_of(" \t\r\n");
	return s.substr(a, b - a + 1);
}

// collapse_ws squeezes runs of whitespace (the markup indentation the site
// leaves inside a text node) into single spaces.
static std::string collapse_ws(const std::string &s) {
	std::string out;
	bool sp = false;
	for (char c : s) {
		if (c == ' ' || c == '\t' || c == '\r' || c == '\n') {
			sp = true;
			continue;
		}
		if (sp && !out.empty()) out += ' ';
		sp = false;
		out += c;
	}
	return out;
}

// clean turns a rendered HTML fragment into plain text, via the host so the
// vocabulary matches every runtime. strip_html already unescapes entities, so
// the second unescape is idempotent.
static std::string clean(const std::string &s) {
	return collapse_ws(host_str("text.unescape", host_str("text.strip_html", s)));
}

// RFC3339 in UTC via host.text.date_to_iso ("3 weeks ago" included); "" when
// unparsable.
static std::string parse_time(const std::string &s) { return s.empty() ? "" : host_str("text.date_to_iso", s); }

// ─── HTTP ───────────────────────────────────────────────────────────────────
// The plain http.get/http.post helpers return the whole response object, so
// the HTTP status survives and a 404 can still map to "not_found". The
// _body forms drop the status, which this plugin needs.

struct HttpResult {
	int status = 0;
	std::string body;
	bool ok() const { return status >= 200 && status < 300; }
};

static HttpResult http_result_from(const std::string &fn, const std::vector<std::string> &args) {
	HttpResult r;
	rapidjson::Document d;
	if (!host_json(fn, args, d) || !d.IsObject()) return r;
	if (auto it = d.FindMember("status"); it != d.MemberEnd() && it->value.IsInt()) r.status = it->value.GetInt();
	if (auto it = d.FindMember("body"); it != d.MemberEnd() && it->value.IsString()) r.body = it->value.GetString();
	return r;
}

static HttpResult http_get(const std::string &url, const std::string &headers_json) {
	return http_result_from("http.get", {url, headers_json});
}

static HttpResult http_post(const std::string &url, const std::string &body, const std::string &headers_json) {
	return http_result_from("http.post", {url, body, headers_json});
}
