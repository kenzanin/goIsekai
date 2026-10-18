// native_abi.h — host-helper bridge for native (.so) goIsekai plugins (C++17).
//
// Copy this file into a native plugin directory as-is. It is the .so
// counterpart of the wasm plugins' wasm_abi.h: the same helper names and the
// same JSON envelope, so plugin code ports between the two runtimes unchanged.
//
// Why a bridge at all: a wasm guest reaches host helpers through the
// env.host_call import, but a .so has no import table. So the HOST calls this
// file's plugin_init export at load time with a function pointer, and every
// helper below goes through it. One bridge reaches every helper, because the
// host resolves them all from a single dispatch table (http.*, text.*, html.*,
// json.*, regex.*, codecs.*, crypto.*).
//
// Ownership: the host writes its answer into a buffer THIS plugin owns, so no
// allocation ever crosses the two allocators.
#pragma once

#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <string>
#include <vector>

#include <rapidjson/document.h>
#include <rapidjson/stringbuffer.h>
#include <rapidjson/writer.h>

typedef int32_t (*goisekai_host_call_fn)(const char *req_json, char *out, int32_t cap);

static goisekai_host_call_fn goisekai_host_call = nullptr;

// plugin_init is the export the host looks for. It is optional: a plugin
// without it still loads, and the helpers below then return "" / false.
extern "C" __attribute__((visibility("default"))) void plugin_init(goisekai_host_call_fn fn) {
	goisekai_host_call = fn;
}

// host_call_raw sends the wasm-shaped {"fn":..,"args":[..]} envelope and returns
// the JSON answer, or "" when the bridge is absent or no buffer was big enough.
// The buffer doubles on a -1 reply, which is how a large body (an HTML page)
// gets through.
static std::string host_call_raw(const std::string &fn, const std::vector<std::string> &args) {
	if (!goisekai_host_call) return "";

	rapidjson::Document req;
	req.SetObject();
	auto &alloc = req.GetAllocator();
	req.AddMember("fn", rapidjson::Value(fn.c_str(), (rapidjson::SizeType)fn.size(), alloc), alloc);
	rapidjson::Value arr(rapidjson::kArrayType);
	for (const auto &a : args) {
		arr.PushBack(rapidjson::Value(a.c_str(), (rapidjson::SizeType)a.size(), alloc), alloc);
	}
	req.AddMember("args", arr, alloc);
	rapidjson::StringBuffer sb;
	rapidjson::Writer<rapidjson::StringBuffer> w(sb);
	req.Accept(w);
	const std::string req_str = sb.GetString();

	for (size_t cap = 64 * 1024; cap <= 8 * 1024 * 1024; cap *= 2) {
		std::vector<char> buf(cap);
		int32_t n = goisekai_host_call(req_str.c_str(), buf.data(), (int32_t)cap);
		if (n < 0) continue; // did not fit; retry with a bigger buffer
		return std::string(buf.data(), (size_t)n);
	}
	return "";
}

// host_json calls a helper and parses its answer into out, reporting whether it
// parsed. A helper failure comes back as {"error":"..."}, which parses fine and
// is left for the caller to notice.
static bool host_json(const std::string &fn, const std::vector<std::string> &args, rapidjson::Document &out) {
	const std::string raw = host_call_raw(fn, args);
	if (raw.empty()) return false;
	out.Parse(raw.c_str(), raw.size());
	return !out.HasParseError();
}

// host_str calls a one-string helper and returns its JSON string result, or ""
// when the bridge is missing or the helper replied with an error envelope.
static std::string host_str(const char *fn, const std::string &arg) {
	rapidjson::Document d;
	if (!host_json(fn, {std::string(arg)}, d)) return "";
	return d.IsString() ? d.GetString() : "";
}

static bool host_body(const char *fn, const std::vector<std::string> &args, std::string &out) {
	rapidjson::Document d;
	if (!host_json(fn, args, d)) return false;
	if (!d.IsString()) return false;
	out = d.GetString();
	return !out.empty();
}

// get_body fetches a URL through the host proxy (which applies the plugin's
// headers, rate limiting and any CDP solve) and returns the body.
static bool get_body(const std::string &url, std::string &out) {
	return host_body("http.get_body", {url, ""}, out);
}

// http_body is get_body with extra request headers given as a JSON object.
static bool http_body(const std::string &url, const std::string &headers_json, std::string &out) {
	return host_body("http.get_body", {url, headers_json}, out);
}

// http_post_body POSTs url with body and headers; args are (url, body, headers).
static bool http_post_body(const std::string &url, const std::string &body, const std::string &headers_json,
                           std::string &out) {
	return host_body("http.post_body", {url, body, headers_json}, out);
}

// to_json serializes a RapidJSON value (object/array) to a compact string.
static std::string to_json(const rapidjson::Value &v) {
	rapidjson::StringBuffer sb;
	rapidjson::Writer<rapidjson::StringBuffer> w(sb);
	v.Accept(w);
	return sb.GetString();
}
