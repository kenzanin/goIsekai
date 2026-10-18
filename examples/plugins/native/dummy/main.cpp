// Dummy native plugin for testing the .so ABI. Not a real source: it answers
// every entry point from a canned fixture so the host's loader, ownership
// (plugin_free) and error-envelope handling can be exercised without a live
// site. Build with ./build.sh; the test skips when libdummy.so is absent.
//
// ABI (see internal/pluginmanager/goplug.go):
//   char *Search(const char *json);   // JSON in, JSON out, NUL-terminated
//   void  plugin_free(char *p);       // releases a buffer a plugin returned
//
// Return shapes match the host's decoders exactly: Search and GetChapterList
// decode a bare JSON array, GetMangaDetail a bare object.

#include <cstdlib>
#include <cstring>
#include <string>

#include "native_abi.h"

static char *dup_json(const std::string &s) {
	char *p = static_cast<char *>(std::malloc(s.size() + 1));
	if (!p) return nullptr;
	std::memcpy(p, s.c_str(), s.size() + 1);
	return p;
}

extern "C" {

void plugin_free(char *p) { std::free(p); }

// Echoes the query back so the test can prove the argument crossed the ABI.
char *Search(const char *json) {
	std::string q = json ? json : "";
	std::string title = "dummy result";
	std::string key = "\"query\":\"";
	auto at = q.find(key);
	if (at != std::string::npos) {
		at += key.size();
		auto end = q.find('"', at);
		if (end != std::string::npos) title += ":" + q.substr(at, end - at);
	}
	return dup_json(R"([{"id":"dummy-1","title":")" + title +
	               R"(","cover_url":"https://example.invalid/1.jpg"}])");
}

char *GetMangaDetail(const char *) {
	// description carries the host's answer to "text.titlecase" so the test can
	// assert the plugin_init bridge actually reached a helper. An empty value
	// means the bridge never got installed.
	std::string probe = host_str("text.titlecase", "host bridge");
	return dup_json(R"({"id":"dummy-1","title":"dummy result","description":")" + probe + R"("})");
}

char *GetChapterList(const char *) {
	// chapter_num is a JSON number in types.Chapter (float64), not a string.
	return dup_json(R"([{"id":"c1","chapter_num":1,"title":"Chapter 1"}])");
}

// Reports failure through the shared error envelope instead of a payload, so
// the test can assert the host turns it into an error.
char *GetPageList(const char *) { return dup_json(R"({"error":"not_found"})"); }

char *GetGenres(const char *) { return dup_json(R"([{"name":"Action","slug":"action"}])"); }

} // extern "C"
