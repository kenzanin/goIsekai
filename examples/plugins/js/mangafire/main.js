// MangaFire JS plugin for goIsekai
// Ported from the WASM plugin (examples/plugins/wasm/mangafire). MangaFire
// signs every /api request with a `vrf` query parameter: a 3-stage XOR-table
// transform over a sign-string, then base64url-Raw (no padding). Tables/keys
// rotate with the frontend — keep the base64 constants in sync with
// vrf.go when the frontend changes.
//
// Image CDN (e.g. img-r1.2xstorage.com) returns 403 without a Referer, so
// page objects carry Headers={"Referer":"https://mangafire.to/"}; the host
// /image endpoint forwards that Referer upstream.

var PLUGIN = {
    contract_version: 1,
    name: "MangaFire",
    site_url: "https://mangafire.to",
    logo: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Ctext y='28' font-size='28'%3E\uD83D\uDD25%3C/text%3E%3C/svg%3E",
    thumb_ratio: 0.677,
    search_page_size: 50,
    alt_title_servers: [{ id: "mangadex", name: "MangaDex" }],
};

var API_URL = "https://mangafire.to/api";
var REFERER = "https://mangafire.to/";

// ---------------------------------------------------------------------------
// HTTP helper (same contract as other JS plugins)
// ---------------------------------------------------------------------------

function httpGet(url) {
    var resp = http_request(JSON.stringify({ method: "GET", url: url }));
    if (typeof resp === "string") {
        try { resp = JSON.parse(resp); } catch (e) { return null; }
    }
    return resp;
}

// ---------------------------------------------------------------------------
// VRF signer (port of vrf.go) — byte arrays are plain JS number arrays
// ---------------------------------------------------------------------------

var B64CHARS = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
var B64URL = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";

function b64decode(s) {
    var out = [], buf = 0, bits = 0;
    for (var i = 0; i < s.length; i++) {
        var c = s.charAt(i);
        if (c === "=") break;
        var v = B64CHARS.indexOf(c);
        if (v < 0) continue;
        buf = (buf << 6) | v;
        bits += 6;
        if (bits >= 8) {
            bits -= 8;
            out.push((buf >> bits) & 0xFF);
        }
    }
    return out;
}

function b64urlEncode(bytes) {
    var out = "";
    for (var i = 0; i < bytes.length; i += 3) {
        var b0 = bytes[i];
        var b1 = i + 1 < bytes.length ? bytes[i + 1] : 0;
        var b2 = i + 2 < bytes.length ? bytes[i + 2] : 0;
        var n = (b0 << 16) | (b1 << 8) | b2;
        out += B64URL.charAt((n >> 18) & 63) + B64URL.charAt((n >> 12) & 63);
        if (i + 1 < bytes.length) out += B64URL.charAt((n >> 6) & 63);
        if (i + 2 < bytes.length) out += B64URL.charAt(n & 63);
    }
    return out;
}

// Encode a JS string to its UTF-8 bytes (Go []byte(signStr) semantics).
function utf8Bytes(str) {
    var out = [];
    for (var i = 0; i < str.length; i++) {
        var c = str.charCodeAt(i);
        if (c < 0x80) {
            out.push(c);
        } else if (c < 0x800) {
            out.push(0xC0 | (c >> 6), 0x80 | (c & 63));
        } else if (c >= 0xD800 && c <= 0xDBFF && i + 1 < str.length) {
            var c2 = str.charCodeAt(i + 1);
            if (c2 >= 0xDC00 && c2 <= 0xDFFF) {
                var cp = 0x10000 + ((c - 0xD800) << 10) + (c2 - 0xDC00);
                out.push(0xF0 | (cp >> 18), 0x80 | ((cp >> 12) & 63),
                    0x80 | ((cp >> 6) & 63), 0x80 | (cp & 63));
                i++;
            } else {
                out.push(0xEF, 0xBF, 0xBD); // lone surrogate -> U+FFFD
            }
        } else if (c >= 0xDC00 && c <= 0xDFFF) {
            out.push(0xEF, 0xBF, 0xBD); // lone surrogate -> U+FFFD
        } else {
            out.push(0xE0 | (c >> 12), 0x80 | ((c >> 6) & 63), 0x80 | (c & 63));
        }
    }
    return out;
}

// VRF table/key constants (base64) — mirror vrf.go exactly.
var VRF_K1 = "0Ec58JOY3uBzJK9m3zqIOpdlF7UFiax9DmA=";
var VRF_T1 = "yINlmUNho8VYJT+ibTIP+9ESiULpVEtMOoD6U6lRE0R/xwXo/Xp9NrUgC4cw/Lmo33vUyjUE40kUoEWIr/fxfNNcq2s79ShQ5NhNrFnJ4hXPwOu/SuXzIbuTQKGFvfm08E9jvCfqAtoDqvQq3dVWPQFmJjgvkISBeXY3BgANR+yVnjGbcxZ47d6kLNfZPIayTq3/YGySb1KuVZodWp/WGNAO5pfMcpaK53Hhs0allBszaMaxuouOwdxbwgxIw6YunSsXjI05Yi0j9j4eHKfSXR8Ifo/Od+8iamRfCXTyvm7NGRGYdcQ0ywcK/u6RXhrbcCm4t2eCtrDgQVecJGkQ+A==";
var VRF_K2 = "AAdjb1iPY8CiDmq9H34tKTBF8a3oDQ==";
var VRF_T2 = "IUFltCxD3Oc2cwCgkJffthaOg9cgPUb0LgW6H/VtfcF0kc5F25t+aWj6JH9VOhOaY0rAFdUxlDnl5BLNvwEJvQtP5qcw7vdb/K+chnbwnspSHT8mz5lqwz41TezG0hkO06FTjJZhsyNuFLDpD2ZZxQj/QIRcF90zpmQ7Byu483WsQqUE0C342HL+JXngRB6fRzxRyVTaKu83h7UYTJ0QMt6ixFh6S3F8gqkKwrGTL3jHNBsD45UnifK8+RGtishQV2K3rujLKEkiZxpr2dYcudFW4oFsDKhad3CLBvuyTqsCo4B7mL5IKQ1vXo/MOOvq1I1d8ar9X6Ttu5KF4fZgiA==";
var VRF_K3 = "DELOJgPsVaCcblDtTGMdHzM=";
var VRF_T3 = "NQHlu1/wVO5EmkwQymF810qqY2xG1k2obcas4Z9mCsPEIFl9pRIjFxbJ7ybMHbBckT5Ton85E0FOeHezbh/mjlEYpmpnlXOS8dgrqeq2KfxImTh1YK9y0PeMNhzA1OQzSY9brYOJq/l2QnE/hwOeZIhPixVSKIUlDb5vLcH6RWKxkIEMuP0bDwIqQ71AJJaEaMJL7A6YtyIwoRT+L5v4aZzodN/0+3nOGsfblFjgxSfPzVDjNFeNl5P26+kEC/8AHgdrpAbt3hHz3HrRN1Y6e+JHgF7ncFWnoF0y3THL1S71WgWGCa6KtSzTCCG58n68nTyj2T3Sshk7utqCtMi/ZQ==";

var VRF_STAGES = [
    { iv: 0x5A, key: b64decode(VRF_K1), tbl: b64decode(VRF_T1) },
    { iv: 0x35, key: b64decode(VRF_K2), tbl: b64decode(VRF_T2) },
    { iv: 0xBA, key: b64decode(VRF_K3), tbl: b64decode(VRF_T3) },
];

// stage: out[i] = table[data[i] XOR key[i%len(key)] XOR prev]
function stage(data, iv, key, tbl) {
    var out = [], prev = iv, kl = key.length;
    for (var i = 0; i < data.length; i++) {
        var x = (data[i] ^ key[i % kl] ^ prev) & 0xFF;
        var v = tbl[x];
        out.push(v);
        prev = v;
    }
    return out;
}

// Sign: signStr = apiPath (no /api) + "?" + sorted "k=v" (values raw).
function vrfSign(apiPath, params) {
    var signStr = apiPath;
    if (signStr.indexOf("/api") === 0) signStr = signStr.substring(4);
    var keys = [];
    for (var k in params) keys.push(k);
    keys.sort();
    if (keys.length > 0) {
        var parts = [];
        for (var i = 0; i < keys.length; i++) {
            parts.push(keys[i] + "=" + params[keys[i]]);
        }
        signStr += "?" + parts.join("&");
    }
    var data = utf8Bytes(signStr);
    for (var j = 0; j < VRF_STAGES.length; j++) {
        var st = VRF_STAGES[j];
        data = stage(data, st.iv, st.key, st.tbl);
    }
    return b64urlEncode(data);
}

// Go net/url QueryEscape semantics (space -> "+", unreserved kept).
function qsEscape(s) {
    var bytes = utf8Bytes(s), out = "";
    for (var i = 0; i < bytes.length; i++) {
        var b = bytes[i];
        if ((b >= 48 && b <= 57) || (b >= 65 && b <= 90) || (b >= 97 && b <= 122) ||
            b === 45 || b === 95 || b === 46 || b === 126) {
            out += String.fromCharCode(b);
        } else if (b === 32) {
            out += "+";
        } else {
            var h = b.toString(16).toUpperCase();
            out += "%" + (h.length < 2 ? "0" + h : h);
        }
    }
    return out;
}

function qs(params) {
    var keys = [];
    for (var k in params) keys.push(k);
    keys.sort();
    var parts = [];
    for (var i = 0; i < keys.length; i++) {
        parts.push(qsEscape(keys[i]) + "=" + qsEscape(params[keys[i]]));
    }
    return parts.join("&");
}

function vrfURL(apiPath, params) {
    var sig = vrfSign(apiPath, params);
    var u = API_URL + apiPath;
    var query = params ? qs(params) : "";
    if (query) return u + "?" + query + "&vrf=" + sig;
    return u + "?vrf=" + sig;
}

// ---------------------------------------------------------------------------
// Text helpers (mirror WASM main.go)
// ---------------------------------------------------------------------------

function stripHTML(s) {
    if (!s) return "";
    s = s.split("<br>").join("\n");
    s = s.split("<br/>").join("\n");
    s = s.split("<br />").join("\n");
    s = s.split("&quot;").join('"');
    s = s.split("&#039;").join("'");
    s = s.split("&amp;").join("&");
    s = s.split("&lt;").join("<");
    s = s.split("&gt;").join(">");
    var out = "", inTag = false;
    for (var i = 0; i < s.length; i++) {
        var c = s.charAt(i);
        if (c === "<") { inTag = true; continue; }
        if (c === ">") { inTag = false; continue; }
        if (!inTag) out += c;
    }
    return out.replace(/^\s+|\s+$/g, "");
}

function normalizeStatus(s) {
    if (s === "releasing") return "Ongoing";
    if (s === "finished") return "Completed";
    if (s === "on_hold" || s === "on_hiatus") return "Hiatus";
    if (s === "discontinued") return "Dropped";
    if (s === "not_published" || s === "upcoming") return "Upcoming";
    if (s) return s;
    return "unknown";
}

function sanitizeTitle(s) {
    if (!s) return "";
    s = s.split("&#039;").join("'");
    s = s.split("&quot;").join('"');
    s = s.split("&amp;").join("&");
    return s.replace(/^\s+|\s+$/g, "");
}

// ---------------------------------------------------------------------------
// ABI functions
// ---------------------------------------------------------------------------

// Search — returns ALL results across upstream pages (host paginates).
function searchManga(arg) {
    var f = JSON.parse(arg);
    var query = f && f.query || "";

    log.info("mangafire search: q=" + query);

    var all = [], page = 1;
    while (true) {
        var resp = httpGet(vrfURL("/titles", { keyword: query, limit: "50", page: "" + page }));
        if (!resp || resp.status < 200 || resp.status >= 300) {
            log.error("mangafire search: HTTP " + (resp ? resp.status : "null") + " page=" + page);
            break;
        }
        var body;
        try { body = JSON.parse(resp.body); } catch (e) { log.error("mangafire search: JSON parse: " + e); break; }
        var items = body && body.items || [];
        if (items.length === 0) break;
        for (var i = 0; i < items.length; i++) {
            var it = items[i];
            all.push({
                id: it.hid,
                title: sanitizeTitle(it.title),
                cover_url: it.poster && it.poster.medium || "",
            });
        }
        var meta = body.meta || {};
        if (!meta.has_next || page >= meta.last_page) break;
        page++;
        if (all.length >= 2000) break; // safety cap
    }

    log.info("mangafire search: found " + all.length + " results for q=" + query);
    return JSON.stringify(all);
}

function getMangaDetail(arg) {
    var hid = JSON.parse(arg);
    if (!hid) return JSON.stringify(null);

    log.info("mangafire detail: id=" + hid);

    var resp = httpGet(vrfURL("/titles/" + hid, null));
    if (!resp || resp.status < 200 || resp.status >= 300) return JSON.stringify(null);
    var body;
    try { body = JSON.parse(resp.body); } catch (e) { return JSON.stringify(null); }
    var d = body && body.data;
    if (!d) return JSON.stringify(null);

    return JSON.stringify({
        id: d.hid || hid,
        title: sanitizeTitle(d.title),
        description: stripHTML(d.synopsisHtml),
        cover_url: d.poster && d.poster.medium || "",
        status: normalizeStatus(d.status),
    });
}

// Fetches up to 3 pages (200/page), newest-first, matching the WASM plugin.
function getChapterList(arg) {
    var hid = JSON.parse(arg);
    if (!hid) return JSON.stringify([]);

    log.info("mangafire chapters: id=" + hid);

    var chapters = [], page = 1;
    while (true) {
        var resp = httpGet(vrfURL("/titles/" + hid + "/chapters", {
            language: "en", limit: "200", order: "desc", page: "" + page, sort: "number",
        }));
        if (!resp || resp.status < 200 || resp.status >= 300) break;
        var body;
        try { body = JSON.parse(resp.body); } catch (e) { break; }
        var items = body && body.items || [];
        if (items.length === 0) break;
        for (var i = 0; i < items.length; i++) {
            var c = items[i];
            chapters.push({
                id: "" + c.id,
                manga_id: hid,
                chapter_num: c.number,
                title: c.name || "",
                released_at: new Date((c.created_at || 0) * 1000).toISOString(),
                url: "",
            });
        }
        var meta = body.meta || {};
        if (page >= meta.last_page || !meta.has_next || page >= 3) break;
        page++;
    }

    log.info("mangafire chapters: found " + chapters.length + " chapters for " + hid);
    return JSON.stringify(chapters);
}

// Each page carries the Referer required by the image CDN.
function getPageList(arg) {
    var chapterID = JSON.parse(arg);
    if (!chapterID) return JSON.stringify([]);

    log.info("mangafire pages: chapter=" + chapterID);

    var resp = httpGet(vrfURL("/chapters/" + chapterID, null));
    if (!resp || resp.status < 200 || resp.status >= 300) return JSON.stringify([]);
    var body;
    try { body = JSON.parse(resp.body); } catch (e) { return JSON.stringify([]); }
    var rawPages = body && body.data && body.data.pages || [];
    if (rawPages.length === 0) return JSON.stringify([]);

    var pages = [];
    for (var i = 0; i < rawPages.length; i++) {
        pages.push({
            index: i,
            url: rawPages[i].url,
            headers: { Referer: REFERER },
        });
    }

    log.info("mangafire pages: found " + pages.length + " pages for " + chapterID);
    return JSON.stringify(pages);
}

// Optional enricher: resolve alternative titles via the MangaDex API.
// Returns {source, titles} — the host uses `source` as the "via X" badge.
function getAltTitles(arg) {
    var input = JSON.parse(arg);
    var title = input.title || "";
    var qs2 = "title=" + encodeURIComponent(title) + "&limit=5&includes[]=manga";
    var resp = httpGet("https://api.mangadex.org/manga?" + qs2);
    if (!resp || resp.status < 200 || resp.status >= 300) {
        return JSON.stringify({ source: "MangaDex", titles: [] });
    }
    var body;
    try { body = JSON.parse(resp.body); } catch (e) {
        return JSON.stringify({ source: "MangaDex", titles: [] });
    }
    var data = body && body.data;
    if (!data || data.length === 0) {
        return JSON.stringify({ source: "MangaDex", titles: [] });
    }
    var attrs = data[0].attributes;
    var out = [], seen = {};
    var keepLangs = { "en": true, "ja": true, "ja-ro": true, "ko": true, "ko-ro": true };
    if (attrs && attrs.altTitles) {
        for (var i = 0; i < attrs.altTitles.length; i++) {
            var alt = attrs.altTitles[i];
            var keys = Object.keys(alt);
            for (var j = 0; j < keys.length; j++) {
                var lang = keys[j];
                if (keepLangs[lang] && alt[lang] && !seen[alt[lang]]) {
                    seen[alt[lang]] = true;
                    out.push(alt[lang]);
                }
            }
        }
    }
    return JSON.stringify({ source: "MangaDex", titles: out });
}
