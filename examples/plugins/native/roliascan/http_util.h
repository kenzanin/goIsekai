// http_util.h — libcurl wrapper for native goIsekai plugins (C++17).
//
// Native plugins get no host_call import the way wasm ones do, so HTTP is the
// plugin's own job. This keeps the one curl handle setup in a single place.
#pragma once

#include <curl/curl.h>

#include <cstring>
#include <string>

static const char *kUA =
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 "
    "Safari/537.36";

struct HttpResult {
	long status = 0;
	std::string body;
	bool ok() const { return status >= 200 && status < 300; }
};

static size_t write_cb(char *ptr, size_t size, size_t nmemb, void *ud) {
	((std::string *)ud)->append(ptr, size * nmemb);
	return size * nmemb;
}

// http_request performs one GET, or a POST when body is non-nil. ctype is the
// full Content-Type header line ("Content-Type: application/json").
static HttpResult http_request(const std::string &url, const char *referer, const char *body,
                               const char *ctype) {
	HttpResult res;
	static bool inited = false;
	if (!inited) {
		curl_global_init(CURL_GLOBAL_DEFAULT);
		inited = true;
	}
	CURL *h = curl_easy_init();
	if (!h) return res;

	struct curl_slist *hdr = nullptr;
	hdr = curl_slist_append(hdr, "Accept: application/json, text/plain, */*");
	std::string ref = std::string("Referer: ") + (referer ? referer : "");
	hdr = curl_slist_append(hdr, ref.c_str());
	if (body) {
		std::string origin = std::string("Origin: ") + (referer ? referer : "");
		hdr = curl_slist_append(hdr, origin.c_str());
	}

	curl_easy_setopt(h, CURLOPT_URL, url.c_str());
	curl_easy_setopt(h, CURLOPT_USERAGENT, kUA);
	curl_easy_setopt(h, CURLOPT_WRITEFUNCTION, write_cb);
	curl_easy_setopt(h, CURLOPT_WRITEDATA, &res.body);
	curl_easy_setopt(h, CURLOPT_FOLLOWLOCATION, 1L);
	curl_easy_setopt(h, CURLOPT_TIMEOUT, 30L);
	curl_easy_setopt(h, CURLOPT_ACCEPT_ENCODING, "");
	if (body) {
		struct curl_slist *with_type = curl_slist_append(hdr, ctype);
		curl_easy_setopt(h, CURLOPT_HTTPHEADER, with_type);
		curl_easy_setopt(h, CURLOPT_POST, 1L);
		curl_easy_setopt(h, CURLOPT_POSTFIELDS, body);
		curl_easy_setopt(h, CURLOPT_POSTFIELDSIZE, (long)strlen(body));
		curl_easy_perform(h);
		curl_easy_getinfo(h, CURLINFO_RESPONSE_CODE, &res.status);
		curl_easy_cleanup(h);
		curl_slist_free_all(with_type);
		return res;
	}

	curl_easy_setopt(h, CURLOPT_HTTPHEADER, hdr);
	curl_easy_perform(h);
	curl_easy_getinfo(h, CURLINFO_RESPONSE_CODE, &res.status);
	curl_easy_cleanup(h);
	curl_slist_free_all(hdr);
	return res;
}
