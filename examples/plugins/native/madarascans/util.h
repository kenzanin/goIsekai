// util.h — shared text/JSON/HTML helpers for the madarascans native plugin.
//
// Every helper here is either a thin wrapper over a host bridge helper (so the
// scraping vocabulary cannot drift from the Lua/JS/WASM runtimes) or a small
// regex utility. Depends on native_abi.h (RapidJSON + the host_call bridge).
#pragma once

// EXPORT marks an entry point the host looks up by name. The build hides every
// other symbol (-fvisibility=hidden), so this has to stay exported.
#define EXPORT extern "C" __attribute__((visibility("default")))

#include <algorithm>
#include <cctype>
#include <cstdlib>
#include <cstring>
#include <regex>
#include <string>
#include <vector>

#include "native_abi.h"

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

static void add_str(rapidjson::Value &obj, const char *k, const std::string &v,
                    rapidjson::Document::AllocatorType &a) {
	obj.AddMember(rapidjson::Value(k, (rapidjson::SizeType)strlen(k), a),
	              rapidjson::Value(v.c_str(), (rapidjson::SizeType)v.size(), a), a);
}

static void add_num(rapidjson::Value &obj, const char *k, double v, rapidjson::Document::AllocatorType &a) {
	obj.AddMember(rapidjson::Value(k, (rapidjson::SizeType)strlen(k), a), rapidjson::Value(v), a);
}

static std::string document_json(const rapidjson::Document &d) {
	rapidjson::StringBuffer sb;
	rapidjson::Writer<rapidjson::StringBuffer> w(sb);
	d.Accept(w);
	return sb.GetString();
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

// ─── text ───────────────────────────────────────────────────────────────────

static std::string trim(const std::string &s) {
	size_t a = s.find_first_not_of(" \t\r\n");
	if (a == std::string::npos) return "";
	size_t b = s.find_last_not_of(" \t\r\n");
	return s.substr(a, b - a + 1);
}

// collapse_ws squeezes runs of whitespace (including the tabs/newlines that
// the site's markup indentation leaves inside a text node) into single spaces.
// The host's strip_html does not do this, and a chapter title otherwise arrives
// as "Chapter\t\t\t\t\t\t\t20".
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

// Both delegate to the host, so the scraping vocabulary matches every runtime.
// The host's strip_html already unescapes entities, so clean() is idempotent.
static std::string clean(const std::string &s) {
	return collapse_ws(host_str("text.unescape", host_str("text.strip_html", s)));
}

static std::string url_encode(const std::string &s) { return host_str("text.url_encode", s); }
static std::string parse_time(const std::string &s) { return s.empty() ? "" : host_str("text.date_to_iso", s); }

static std::string lower(const std::string &s) {
	std::string l = s;
	std::transform(l.begin(), l.end(), l.begin(), [](unsigned char c) { return (char)std::tolower(c); });
	return l;
}

static std::string normalize_status(const std::string &raw) {
	std::string l = lower(raw);
	if (l.find("ongoing") != std::string::npos || l.find("updating") != std::string::npos) return "Ongoing";
	if (l.find("complete") != std::string::npos || l.find("finished") != std::string::npos) return "Completed";
	if (l.find("hiatus") != std::string::npos || l.find("hold") != std::string::npos) return "Hiatus";
	if (l.find("cancel") != std::string::npos || l.find("drop") != std::string::npos) return "Cancelled";
	return "";
}

// Last path segment of a URL (query/fragment and trailing slash removed).
static std::string slug_of(const std::string &raw) {
	std::string p = raw;
	size_t q = p.find_first_of("?#");
	if (q != std::string::npos) p = p.substr(0, q);
	while (!p.empty() && p.back() == '/') p.pop_back();
	size_t i = p.find_last_of('/');
	return i == std::string::npos ? p : p.substr(i + 1);
}

// ─── regex ──────────────────────────────────────────────────────────────────

// Trimmed first capture group of `re` inside `body`, or "".
static std::string first_sub(const std::regex &re, const std::string &body) {
	std::smatch m;
	if (!std::regex_search(body, m, re) || m.size() < 2) return "";
	return trim(m[1]);
}

// src (or data-src) attribute of an <img ...> tag fragment.
static std::string img_url(const std::string &tag) {
	std::smatch m;
	if (std::regex_search(tag, m, std::regex("data-src\\s*=\\s*\"([^\"]+)\""))) return trim(m[1]);
	if (std::regex_search(tag, m, std::regex("src\\s*=\\s*\"([^\"]+)\""))) return trim(m[1]);
	return "";
}
