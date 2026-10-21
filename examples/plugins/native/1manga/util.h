// util.h — shared helpers for the 1manga native plugin.
//
// Every helper is either a thin wrapper over a host bridge helper (so the
// vocabulary cannot drift from the Lua/JS/WASM runtimes) or a small JSON/HTTP
// convenience. Depends on native_abi.h (RapidJSON + the host_call bridge).
#pragma once

#include <algorithm>
#include <cctype>
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <map>
#include <string>
#include <strings.h>
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

// add_num writes a number member. An integral value goes out as a JSON
// integer, not a double: Go's decoder rejects `{"index":0.0}` when the field
// is an int ("expected comma after object element"), so a double here breaks
// the whole page list.
//
// The cast is int64_t, not long long: RapidJSON's constructors take int64_t
// (which is long here), and long long matches several of them equally, so
// the conversion is ambiguous.
static void add_num(rapidjson::Value &obj, const char *k, double v, rapidjson::Document::AllocatorType &a) {
	rapidjson::Value key(k, (rapidjson::SizeType)strlen(k), a);
	if (v == std::floor(v) && std::fabs(v) < 1e15) {
		obj.AddMember(key, rapidjson::Value((int64_t)v), a);
		return;
	}
	obj.AddMember(key, rapidjson::Value(v), a);
}

// member returns the value at key, or nullptr when it is absent.
static const rapidjson::Value *member(const rapidjson::Value &obj, const char *k) {
	if (!obj.IsObject()) return nullptr;
	auto it = obj.FindMember(k);
	return it == obj.MemberEnd() ? nullptr : &it->value;
}

static std::string jstr(const rapidjson::Value &obj, const char *k) {
	const rapidjson::Value *v = member(obj, k);
	if (!v || !v->IsString()) return "";
	return v->GetString();
}

static int jint(const rapidjson::Value &obj, const char *k, int def) {
	const rapidjson::Value *v = member(obj, k);
	if (!v || !v->IsInt()) return def;
	return v->GetInt();
}

// number_str formats a JSON number the way Lua's tostring does: an integral
// value prints without a decimal point, so a chapter number reads "1196" and
// not "1196.0". The chapter id is built from this string, so the two runtimes
// have to agree on it.
static std::string number_str(const rapidjson::Value &v) {
	if (v.IsInt64()) return std::to_string(v.GetInt64());
	if (v.IsUint64()) return std::to_string(v.GetUint64());
	if (v.IsDouble()) {
		double d = v.GetDouble();
		char buf[40];
		if (d == std::floor(d) && std::fabs(d) < 1e15) {
			snprintf(buf, sizeof buf, "%lld", (long long)d);
		} else {
			snprintf(buf, sizeof buf, "%.14g", d);
		}
		return buf;
	}
	return "";
}

// jnum_str is number_str for a member that may be a number or a string.
static std::string jnum_str(const rapidjson::Value &obj, const char *k) {
	const rapidjson::Value *v = member(obj, k);
	if (!v) return "";
	if (v->IsString()) return v->GetString();
	return number_str(*v);
}

// parse_string_arg unwraps the ABI argument for the calls whose input is a bare
// JSON string (a manga id, a chapter id) rather than an object.
static std::string parse_string_arg(const char *json) {
	rapidjson::Document d;
	d.Parse(json ? json : "");
	return d.IsString() ? d.GetString() : "";
}

// ─── host helpers ───────────────────────────────────────────────────────────

// host_num reads a helper that answers with a number (text.chapter_num). The
// plain host_str would see a JSON number and drop it.
static double host_num(const char *fn, const std::string &arg) {
	rapidjson::Document d;
	if (!host_json(fn, {arg}, d)) return 0;
	return d.IsNumber() ? d.GetDouble() : 0;
}

// ─── text ───────────────────────────────────────────────────────────────────

static std::string trim(const std::string &s) {
	size_t a = s.find_first_not_of(" \t\r\n");
	if (a == std::string::npos) return "";
	size_t b = s.find_last_not_of(" \t\r\n");
	return s.substr(a, b - a + 1);
}

// parse_time is RFC3339 in UTC via host.text.date_to_iso; "" when unparsable.
static std::string parse_time(const std::string &s) { return s.empty() ? "" : host_str("text.date_to_iso", s); }

// ─── HTTP ───────────────────────────────────────────────────────────────────
// The plain http.get/http.post helpers return the whole response object, so
// the HTTP status and the response headers both survive.

struct HttpResult {
	int status = 0;
	std::string body;
	std::map<std::string, std::string> headers;
	bool ok() const { return status >= 200 && status < 300; }

	// header looks a response header up, ignoring case: the Go side canonicalises
	// the names ("Set-Cookie"), so a caller cannot rely on the spelling it asked
	// for.
	std::string header(const std::string &name) const {
		for (const auto &kv : headers) {
			if (kv.first.size() != name.size()) continue;
			if (strcasecmp(kv.first.c_str(), name.c_str()) == 0) return kv.second;
		}
		return "";
	}
};

static HttpResult http_result_from(const std::string &fn, const std::vector<std::string> &args) {
	HttpResult r;
	rapidjson::Document d;
	if (!host_json(fn, args, d) || !d.IsObject()) return r;
	if (auto it = d.FindMember("status"); it != d.MemberEnd() && it->value.IsInt()) r.status = it->value.GetInt();
	if (auto it = d.FindMember("body"); it != d.MemberEnd() && it->value.IsString()) r.body = it->value.GetString();
	if (auto it = d.FindMember("headers"); it != d.MemberEnd() && it->value.IsObject()) {
		for (auto h = it->value.MemberBegin(); h != it->value.MemberEnd(); ++h) {
			if (h->name.IsString() && h->value.IsString()) r.headers[h->name.GetString()] = h->value.GetString();
		}
	}
	return r;
}

static HttpResult http_get(const std::string &url, const std::string &headers_json) {
	return http_result_from("http.get", {url, headers_json});
}

static HttpResult http_post(const std::string &url, const std::string &body, const std::string &headers_json) {
	return http_result_from("http.post", {url, body, headers_json});
}
