-- Weebcentral plugin for goIsekai
--
-- Server-rendered HTML + HTMX (no JSON API). Every interactive load is a GET
-- returning an HTML fragment targeted by HX-Target. Anonymous traffic: 0 cookies.
--
-- Identity: manga = ULID at /series/{ULID}/{slug}, chapter = ULID at /chapters/{ULID}.
-- Covers on temp.compsci88.com (no Referer needed).
-- Page images on official.lowee.us REQUIRE Referer: https://weebcentral.com/
--   (host referer ladder is origin-first, so [] as site_url would break images).
--
-- Spec: okf-mcp/concept/weebcentral-plugin-build-spec.md
--   Search    GET /search/data?text=&limit=24&offset=0  -> fragment of <article> cards
--   Detail    GET /series/{ULID}/{slug}                -> h1 + picture cover + info <li>
--   Chapters  GET /series/{ULID}/full-chapter-list     -> fragment, 121 rows, newest-first
--   Pages     GET /chapters/{ULID}                     -> doc with max_page embedded
--             GET /chapters/{ULID}/images?...current_page=N -> <img> fragment (paginated!)

PLUGIN = {
    contract_version = 1,
    name = "Weebcentral",
    site_url = "https://weebcentral.com",
    verify_url = "https://weebcentral.com",
    needs_human_verify = false,
    thumb_ratio = 0.667,
    logo = "logo.png",
}

BASE = "https://weebcentral.com"
UA = "Mozilla/5.0 (compatible; goIsekai-plugin/1.0)"

local function fetch(url)
    local body = host.http.get_body(url, {
        ["User-Agent"] = UA,
        ["Accept"] = "text/html,application/xhtml+xml",
    })
    if not body then
        return nil
    end
    return host.html.parse(body)
end

-- ─── ABI: search_manga ───────────────────────────────────────────────────
-- arg: {query=string, page=n}. Returns bare ULID as id; get_manga_detail
-- navigates /series/{ULID}/auto.
function search_manga(arg)
    local args = host.json.decode(arg) or {}
    local query = host.text.url_encode(args.query or "")
    local page = args.page or 1
    local limit = 24
    local offset = (page - 1) * limit

    local url = BASE .. "/search/data?text=" .. query
        .. "&limit=" .. limit .. "&offset=" .. offset
    local doc = fetch(url)
    if not doc then
        return host.json.encode({})
    end

    -- Cards: article.bg-base-300. Clean title lives in the desktop
    -- a.line-clamp-1 anchor (1 per card); the picture anchor's text is
    -- badge noise ("Official..."), so never take titles from it.
    local hrefs = host.html.find_list_attr(doc, "article.bg-base-300 a[href*='/series/']", "href")
    local imgs = host.html.find_list_attr(doc, "article.bg-base-300 img", "src")
    local ttl_hrefs = host.html.find_list_attr(doc, "article.bg-base-300 a.line-clamp-1", "href")
    local ttl_texts = host.html.find_list_text(doc, "article.bg-base-300 a.line-clamp-1")
    local clean = {}
    for j, th in ipairs(ttl_hrefs) do
        local u = th:match("/series/([^/?#]+)")
        if u and ttl_texts[j] then
            local t = ttl_texts[j]:gsub("^%s+", ""):gsub("%s+$", "")
            if t ~= "" then clean[u] = t end
        end
    end

    -- hrefs carries 2 anchors per card (picture + title) while imgs has
    -- 1 per card, so never index imgs by the href position. Collect
    -- unique ULIDs in card order, align covers by card position.
    local out = {}
    local idx = {}
    for _, href in ipairs(hrefs) do
        local ulid = href:match("/series/([^/?#]+)")
        if ulid and not idx[ulid] then
            idx[ulid] = #out + 1
            out[#out + 1] = {
                id = ulid,
                title = clean[ulid] or ulid,
                cover_url = imgs[#out + 1] or "",
            }
        end
    end
    log.info("weebcentral: search found " .. tostring(#out))
    return host.json.encode(out)
end

-- ─── ABI: get_manga_detail ───────────────────────────────────────────────
-- arg: '"<ULID>"'. Site resolves /series/{ULID}/auto (302 to canonical slug).
function get_manga_detail(arg)
    local ulid = host.json.decode(arg)
    if not ulid then
        return host.json.encode({})
    end
    local doc = fetch(BASE .. "/series/" .. ulid .. "/auto")
    if not doc then
        return host.json.encode({id = ulid, title = "", cover_url = ""})
    end

    -- title is a plain h1 (no span.lead on the page)
    local title = host.html.find_text(doc, "h1") or ""
    -- cover: <picture> > <img>, src is the 400x600 /cover/normal webp
    local cover = host.html.find_attr(doc, "picture img", "src") or ""

    -- info <li>: <strong>Status|Type|Author(s)|Tags(s)</strong> ...
    -- Genre links carry included_tag= in href (no #main-info/.tag hooks).
    local genres = host.html.find_list_text(doc, "a[href*='included_tag']")

    -- author: <li> whose <strong> is Author(s), value in the <div>
    local author = ""
    local lis = host.html.find_list_text(doc, "li")
    for _, li in ipairs(lis) do
        if li:match("^Author") then
            author = li:gsub("^Author%b()**:%s*", ""):gsub("^Author[^:]*:%s*", "")
            break
        end
    end

    local desc = host.html.find_text(doc, "p.whitespace-pre-wrap") or ""
    if desc == "" then
        desc = host.html.find_attr(doc, "meta[name='description']", "content") or ""
    end

    -- status lives in a <strong> anchor: normalize via text.normalize_status
    local status_raw = ""
    for _, li in ipairs(lis) do
        if li:match("^Status") then
            status_raw = li:gsub("^Status[^:]*:%s*", "")
            break
        end
    end

    return host.json.encode({
        id = ulid,
        title = title,
        cover_url = cover,
        author = author,
        description = desc,
        genres = genres,
        status = host.text.normalize_status(status_raw),
    })
end

-- ─── ABI: get_chapter_list ───────────────────────────────────────────────
-- arg: '"<ULID>"'. HTMX fragment, newest-first (ascending = reverse).
function get_chapter_list(arg)
    local ulid = host.json.decode(arg)
    if not ulid then
        return host.json.encode({})
    end
    local doc = fetch(BASE .. "/series/" .. ulid .. "/full-chapter-list")
    if not doc then
        return host.json.encode({})
    end

    local hrefs = host.html.find_list_attr(doc, "a[href*='/chapters/']", "href")
    local row_texts = host.html.find_list_text(doc, "a[href*='/chapters/']")
    local times = host.html.find_list_attr(doc, "a[href*='/chapters/'] time", "datetime")
    if #times == 0 then
        times = host.html.find_list_attr(doc, "time", "datetime")
    end
    local out = {}
    local idx = {}
    for i, href in ipairs(hrefs) do
        local chid = href:match("/chapters/([^/?#]+)")
        if chid and not idx[chid] then
            idx[chid] = #out + 1
            local t = (row_texts[i] or ""):gsub("^%s+", ""):gsub("%s+$", "")
            t = t:gsub("%s*Last Read.*$", "")
            t = t:gsub("%s*%d%d%d%d%-%d%d%-%d%d.*$", "")
            t = t:gsub("^%s+", ""):gsub("%s+$", "")
            out[#out + 1] = {
                id = chid,
                title = t ~= "" and t or chid,
                released_at = times[i] or "",
            }
        end
    end
    -- fragment is newest-first; reader wants ascending
    local asc = {}
    for i = #out, 1, -1 do
        asc[#asc + 1] = out[i]
    end
    log.info("weebcentral: found " .. tostring(#asc) .. " chapters")
    return host.json.encode(asc)
end

-- ─── ABI: get_page_list ──────────────────────────────────────────────────
-- arg: '"<ULID>"'. Two-step: doc has EMPTY #chapter-images + max_page embedded;
-- /images?...current_page=N returns a fragment per batch. Pagination
-- means we must walk ALL batches, not just the first.
function get_page_list(arg)
    local chapter_id = host.json.decode(arg)
    if not chapter_id then
        return host.json.encode({})
    end

    local body = host.http.get_body(BASE .. "/chapters/" .. chapter_id, {
        ["User-Agent"] = UA,
        ["Accept"] = "text/html,application/xhtml+xml",
    })
    if not body then
        return host.json.encode({})
    end

    -- max_page is embedded as Alpine state: max_page: parseInt('20')
    local max_page = 1
    local mp = body:match("max_page:%s*parseInt%(%s*['\"](%d+)")
        or body:match("max_page[%s=]+(%d+)")
    if mp then max_page = tonumber(mp) or 1 end

    local out = {}
    local seen = {}
    for page = 1, max_page do
        local url = BASE .. "/chapters/" .. chapter_id
            .. "/images?is_prev=False&reading_style=long_strip&current_page=" .. page
        local fbody = host.http.get_body(url, {
            ["User-Agent"] = UA,
            ["Accept"] = "text/html,application/xhtml+xml",
        })
        if fbody then
            local frag = host.html.parse(fbody)
            if frag then
                local imgs = host.html.find_list_attr(frag, "#chapter-images img", "src")
                for _, u in ipairs(imgs) do
                    if not seen[u] then
                        seen[u] = true
                        out[#out + 1] = { index = #out, url = u }
                    end
                end
            end
        end
    end
    if #out == 0 then
        return nil, "no_pages"
    end
    log.info("weebcentral: found " .. tostring(#out) .. " pages")
    return host.json.encode(out)
end