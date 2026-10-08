-- MadaraDex plugin for goIsekai (Lua / Lunar VM)
-- Site: https://madaradex.org (WordPress Madara theme, manga archives under /title/)
-- ABI contract version: 1
--
-- Ported from examples/plugins/wasm/madaradex so the selectors stay editable
-- without a Go toolchain. The port fixed one real bug: genre archives on this
-- site are /genre/<slug>/, but the WASM regex only matched /manga-genre/<slug>/
-- as used by sibling Madara sites, so get_genres returned nothing and the empty
-- result was then cached permanently by the host.
--
-- Chapter mode is MangaPage, so the chapter list is read straight off the detail
-- page - no admin-ajax chapter round-trip like anisascans uses.

PLUGIN = {
    contract_version = 1,
    name = "Madaradex",
    site_url = "https://madaradex.org",
    logo = "logo.png",
    verify_url = "https://madaradex.org",
    needs_human_verify = false,
    thumb_ratio = 0.703,
}

BASE = "https://madaradex.org"
AJAX = BASE .. "/wp-admin/admin-ajax.php"
MANGA_PATH = "/title/"

local trim = host.text.trim
local unescape = host.text.unescape
local find = host.regex.find
local gmatch = host.regex.gmatch
local match = host.regex.match

-- MadaraDex intercepts requests and refreshes auth whenever the mdx_auth cookie
-- is missing. The host's per-plugin cookie jar keeps the cookie afterwards, so
-- one handshake per plugin load is enough.
local auth_done = false
local function warm_auth()
    if auth_done then return end
    auth_done = true
    host.http.post_body(AJAX, "action=mdx_auth_refresh", {
        ["X-Mdx-Auth-Refresh"] = "1",
        ["X-Requested-With"] = "xmlhttprequest",
        ["Content-Type"] = "application/x-www-form-urlencoded",
    })
end

local function manga_url(slug)
    return BASE .. MANGA_PATH .. host.text.url_encode(slug) .. "/"
end

-- href can be absolute or site-relative; the reader needs absolute.
local function abs_url(raw)
    if not raw or raw == "" then return "" end
    if match(raw, "^https?://") then return raw end
    if match(raw, "^//") then return "https:" .. raw end
    return BASE .. "/" .. find(raw, "^/+", "")
end

-- Lazy attributes win over src, in the order Madara emits them.
local function img_src(fragment)
    if not fragment or fragment == "" then return "" end
    local v =
        find(fragment, [[data-src="[\t\n\s]*(https?://[^"]+)"]])
        or find(fragment, [[data-src="[\t\n\s]*(/[^"]+)"]])
        or find(fragment, [[data-lazy-src="[\t\n\s]*(https?://[^"]+)"]])
        or find(fragment, [[data-cfsrc="[\t\n\s]*(https?://[^"]+)"]])
        or find(fragment, [[data-manga-src="[\t\n\s]*(https?://[^"]+)"]])
        or find(fragment, [[src="[\t\n\s]*(https?://[^"]+)"]])
        or find(fragment, [[src="[\t\n\s]*(/[^"]+)"]])
    if v then return abs_url(trim(v)) end
    return ""
end

-- ─── ABI: search_manga ───────────────────────────────────────────────────────
-- Plain WordPress GET search, not admin-ajax: this theme's madara_load_more
-- archive endpoint answers "no-posts" for a query string. Genre browsing uses the
-- theme's own /genre/<slug>/ archive, which is server-rendered here.

function search_manga(arg)
    local args = host.json.decode(arg or "{}")
    local query = trim(args.query or "")
    local page = tonumber(args.page) or 1
    if page < 1 then page = 1 end
    local genres = args.genres or {}

    warm_auth()

    local url
    if #genres > 0 then
        url = BASE .. "/genre/" .. host.text.url_encode(genres[1]) .. "/"
        if page > 1 then url = url .. "page/" .. tostring(page) .. "/" end
    else
        url = BASE .. "/?s=" .. host.text.url_encode(query) .. "&post_type=wp-manga"
    end

    local body = host.http.get_body(url)
    if not body or body == "" then
        log.error("madaradex search empty response")
        return host.json.encode({})
    end

    -- Two card layouts on this theme: the search results grid uses
    -- c-tabs-item__content, the /genre/<slug>/ archive uses page-listing-item.
    -- They share the post-title block, so one tolerant wrapper reads both.
    -- Whitespace between <h3 ...> and its <a> also differs between the two.
    local results = {}
    local seen = {}
    for card in gmatch(body, [[(?s)<div[^>]*class="[^"]*(?:c-tabs-item__content|page-listing-item)[^"]*".*?post-title.*?</h3>]]) do
        local href = find(card, [[href="([^"]*/title/[^"/]+/)"]])
        local title = find(card, [[(?s)<h3[^>]*>.*?<a[^>]*>(.*?)</a>]])
        if href then
            local slug = find(href, [[/title/([^/]+)/?]])
            if slug and slug ~= "" and not seen[slug] then
                seen[slug] = true
                local cover = find(card, [[data-src="[\t\n\s]*(https?://[^"]+)"]])
                    or find(card, [[\ssrc="(https?://[^"]+)"]])
                results[#results + 1] = {
                    id = slug,
                    title = unescape(host.text.strip_html(title or "")),
                    cover_url = cover and trim(cover) or "",
                    url = abs_url(href),
                }
            end
        end
    end

    log.debug("madaradex search q=" .. query .. " page=" .. page .. " found=" .. tostring(#results))
    return host.json.encode(results)
end

-- ─── ABI: get_manga_detail ──────────────────────────────────────────────────

function get_manga_detail(arg)
    local slug = trim(host.json.decode(arg or '""'))
    if slug == "" then return host.json.encode({ id = "" }) end

    warm_auth()
    local body = host.http.get_body(manga_url(slug))
    if not body or body == "" then return host.json.encode({ id = slug }) end

    local tblock = find(body, [[(?s)class="post-title[^"]*">(.*?)</div>]]) or ""
    local detail = {
        id = slug,
        title = unescape(host.text.strip_html(find(tblock, [[(?s)<h[1-6][^>]*>(.*?)</h[1-6]>]]) or "")),
        author = unescape(host.text.strip_html(find(body, [[(?s)class="author-content">(.*?)</div>]]) or "")),
        cover_url = img_src(find(body, [[(?s)class="summary_image".*?</a>]]) or ""),
        description = "",
        genres = {},
        status = "",
    }

    local dblock = find(body, [[(?s)class="summary__content[^"]*">(.*?)<span\s+class="[^"]*content-readmore"]]) or ""
    if dblock == "" then
        dblock = find(body, [[(?s)class="summary__content[^"]*">(.*?)</div>]]) or ""
    end
    detail.description = unescape(trim(host.regex.replace(host.text.strip_html(dblock), [[\s+]], " ")))

    local gblock = find(body, [[(?s)class="genres-content">(.*?)</div>]]) or ""
    for g in gmatch(gblock, [[<a[^>]*>([^<]+)</a>]]) do
        local name = unescape(trim(g))
        if name ~= "" then detail.genres[#detail.genres + 1] = name end
    end

    local sblock = find(body, [[(?s)Status\s*</h5>.*?class="summary-content">\s*([^<]+)]])
    if sblock then detail.status = host.text.normalize_status(trim(sblock)) end

    return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list ──────────────────────────────────────────────────
-- MangaPage chapter mode: the list is already on the detail page.

function get_chapter_list(arg)
    local slug = trim(host.json.decode(arg or '""'))
    if slug == "" then return host.json.encode({}) end

    warm_auth()
    local body = host.http.get_body(manga_url(slug))
    if not body or body == "" then return host.json.encode({}) end

    local chapters = {}
    local seen = {}
    for li in gmatch(body, [[(?s)<li[^>]*class="wp-manga-chapter[^"]*"[^>]*>(.*?)</li>]]) do
        local href = find(li, [[href="([^"]+)"]])
        local cid = href and find(href, [[/([^/]+)/?$]])
        local label = trim(host.text.strip_html(find(li, [[(?s)<a[^>]*>(.*?)</a>]]) or ""))
        if href and cid and cid ~= "" and not seen[cid] then
            seen[cid] = true
            chapters[#chapters + 1] = {
                id = slug .. ":" .. cid,
                manga_id = slug,
                chapter_num = host.text.chapter_num(label),
                title = label,
                url = abs_url(href),
            }
        end
    end

    log.debug("madaradex chapters slug=" .. slug .. " count=" .. tostring(#chapters))
    return host.json.encode(chapters)
end

-- ─── ABI: get_page_list ─────────────────────────────────────────────────────

function get_page_list(arg)
    local path = trim(host.json.decode(arg or '""'))
    if path == "" then return host.json.encode({}) end
    local slug, cslug = path:match("^([^:]+):(.+)$")
    if not slug or slug == "" or not cslug or cslug == "" then
        return host.json.encode({})
    end

    warm_auth()
    local url = manga_url(slug) .. host.text.url_encode(cslug) .. "/"
    local body = host.http.get_body(url)
    if not body or body == "" then return host.json.encode({}) end

    -- Madara defaults to the paged reader, which only carries the first image.
    -- The list layout exposes every <img> in one document.
    if find(body, [[id="single-pager"]]) then
        local listed = host.http.get_body(url .. "?style=list")
        if listed and listed ~= "" then body = listed end
    end

    local pages = {}
    local pos = 1
    local scanned = 0
    while true do
        local s = host.regex.find_index(body, "<img", pos)
        if not s then break end
        local e = host.regex.find_index(body, ">", s)
        if not e then break end
        local tag = body:sub(s, e)
        scanned = scanned + 1
        if match(tag, "wp-manga-chapter-img") then
            local src = img_src(tag)
            if src ~= "" then
                -- cdn.madaradex.org answers 403 without a same-site Referer
                -- (curl-verified: no header 403/4546 bytes, with Referer
                -- 200/973784). The headers field must stay populated — an empty
                -- Lua table encodes as [] and the host decodes it as a map.
                pages[#pages + 1] = {
                    index = #pages,
                    url = src,
                    headers = { Referer = BASE .. "/" },
                }
            end
        end
        pos = e + 1
    end

    log.debug("madaradex pages " .. path .. " count=" .. tostring(#pages))
    return host.json.encode(pages)
end

-- ─── get_genres (optional export) ────────────────────────────────────────────
-- Scraped from the site's own /title/ navigation rather than hardcoded, so the
-- list cannot drift from what the theme actually serves. Note the archive path is
-- /genre/<slug>/ on this site - not /manga-genre/<slug>/ like its Madara siblings.

function get_genres()
    warm_auth()
    local body = host.http.get_body(BASE .. MANGA_PATH)
    local genres = {}
    if not body or body == "" then return host.json.encode(genres) end

    local seen = {}
    -- The label is not always bare text: some anchors wrap it in a <span> before
    -- </a>, so capture up to the closing tag and strip markup rather than
    -- requiring the text to sit directly against it.
    for slug, name in gmatch(body, [[(?s)href="[^"]*/genre/([a-z0-9_-]+)/?"[^>]*>(.*?)</a>]]) do
        local label = unescape(trim(host.text.strip_html(name)))
        if slug ~= "" and label ~= "" and not seen[slug] then
            seen[slug] = true
            genres[#genres + 1] = { name = label, slug = slug }
        end
    end

    log.debug("madaradex genres found=" .. tostring(#genres))
    return host.json.encode(genres)
end
