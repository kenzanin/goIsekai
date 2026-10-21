-- Comizy plugin for goIsekai (Lua / Lunar VM)
-- Site: https://comizy.io (Next.js, API-first)
-- ABI contract version: 1
--
-- Contract (verified live 2026-10-12):
--   search:   GET https://api.comizy.io/titles/search?q=<q>&page=N&limit=24
--             -> {success,data:{items:[{id,slug,name,cover,status,cv}],pagination}}
--   version:  GET https://comizy.io/api/version -> {buildId}
--   detail:   GET https://comizy.io/_next/data/{buildId}/{slug}.json?slug={slug}
--             -> {pageProps:{initialManga:{name,summary,cover,status,
--                genres:[{name}],authors:[{name}],cv}}}
--   chapters: GET https://api.comizy.io/titles/{mangaId}/chapters
--             -> {success,data:{chapters:[{id,name,slug,updated_at}]}}
--             (cv param optional — works without; embedded list in detail
--             JSON is capped, never use it)
--   pages:    GET https://comizy.io/_next/data/{buildId}/{mSlug}/{cSlug}.json
--             -> {pageProps:{initialChapter:{images:["url",...]}}}
-- Images need Referer: https://comizy.io/ — covered by host image-proxy
-- fallback to site_url (origin-first policy).

PLUGIN = {
    contract_version = 1,
    name = "Comizy",
    site_url = "https://comizy.io",
    logo = "logo.png",
    verify_url = "https://comizy.io",
    needs_human_verify = false,
    thumb_ratio = 0.703,
}

local API = host.plugin.param("api")
local WEB = host.plugin.param("site_url")

local function get_json(url)
    local body = host.http.get_body(url, { ["accept"] = "application/json" })
    if not body or body == "" then return nil end
    local ok, decoded = pcall(host.json.decode, body)
    if not ok or decoded == nil then return nil end
    return decoded
end

-- buildId rotates per deploy; cache per VM lifetime.
local cached_build_id = nil
local function build_id()
    if cached_build_id then return cached_build_id end
    local v = get_json(WEB .. "/api/version")
    if v and v.buildId then cached_build_id = v.buildId end
    return cached_build_id
end

local function split_id(id)
    local sep = id:find("|", 1, true)
    if not sep then return nil, nil end
    return id:sub(1, sep - 1), id:sub(sep + 1)
end

-- comizy labels a split chapter "Chapter 12-2" and a decimal one "Chapter
-- 12.1". text.chapter_num reads the leading number only, so the hyphen form
-- collapsed to 12 and collided with its sibling. Fold the hyphen into the
-- decimal the dot form already uses.
local function comizy_chapter_num(title)
    local t = (title or ""):gsub("^(%s*[Cc]hapter%s+%d+)%s*%-%s*(%d+)%s*$", "%1.%2")
    return host.text.chapter_num(t)
end

-- ─── ABI: search_manga ─────────────────────────────────────────────────
-- arg: {query=string, page=n, genres?={slug,...}}. id packs "{mangaId}|{slug}"
-- (chapters need the opaque mangaId, pages need the slug — neither derives
-- the other). Genre mode browses /genres/<slug>/ SSR JSON (the search REST
-- endpoint silently ignores genre params — verified 2026-10-12); query is
-- ignored in genre mode, same as anisascans.

function search_manga(arg)
    local args = host.json.decode(arg)
    local query = (args and args.query) or ""
    local page = (args and args.page) or 1
    local genres = (args and args.genres) or {}

    -- Genre browse: SSR JSON at /_next/data/<buildId>/genres/<slug>.json.
    if #genres > 0 then
        local bid = build_id()
        if not bid then
            log.error("comizy: version backend unreachable")
            return nil, "upstream_unavailable"
        end
        local gslug = host.text.url_encode(genres[1])
        local url = WEB .. "/_next/data/" .. bid .. "/genres/" .. gslug
            .. ".json?slug=" .. gslug
        if page > 1 then
            url = url .. "&page=" .. tostring(page)
        end
        local res = get_json(url)
        if not res then
            log.error("comizy: genre browse backend unreachable")
            return nil, "upstream_unavailable"
        end
        local items = ((res.pageProps or {}).items) or {}
        local out = {}
        for _, it in ipairs(items) do
            if it.id and it.slug then
                out[#out + 1] = {
                    id = it.id .. "|" .. it.slug,
                    title = it.name or it.slug,
                    cover_url = it.cover or "",
                }
            end
        end
        log.info("comizy: genre " .. genres[1] .. " page "
            .. tostring(page) .. " found " .. tostring(#out))
        return host.json.encode(out)
    end

    if query == "" then
        return host.json.encode({})
    end

    local url = API .. "/titles/search?q=" .. host.text.url_encode(query)
        .. "&page=" .. tostring(page) .. "&limit=24"
    local res = get_json(url)
    if not res then
        log.error("comizy: search backend unreachable")
        return nil, "upstream_unavailable"
    end
    local items = ((res.data or {}).items) or {}
    local out = {}
    for _, it in ipairs(items) do
        if it.id and it.slug then
            out[#out + 1] = {
                id = it.id .. "|" .. it.slug,
                title = it.name or it.slug,
                cover_url = it.cover or "",
            }
        end
    end
    log.info("comizy: search found " .. tostring(#out) .. " for " .. query)
    return host.json.encode(out)
end

-- ─── ABI: get_genres ───────────────────────────────────────────────────
-- Static genre list from GET /genres (71 entries). Slug doubles as the
-- browse key (/genres/<slug>/). Cached per VM lifetime.

local cached_genres = nil
function get_genres()
    if cached_genres then return cached_genres end
    local res = get_json(API .. "/genres")
    local genres = {}
    if res then
        for _, g in ipairs(((res.data or {}).items) or {}) do
            if g.slug and g.name then
                genres[#genres + 1] = { name = g.name, slug = g.slug }
            end
        end
    end
    log.debug("comizy genres found=" .. tostring(#genres))
    cached_genres = host.json.encode(genres)
    return cached_genres
end

-- ─── ABI: get_manga_detail ─────────────────────────────────────────────
-- arg: JSON string '"{mangaId}|{slug}"'.

function get_manga_detail(arg)
    local manga_id = host.json.decode(arg)
    -- empty table encodes as []; detail must stay an OBJECT — emit {id} only
    if manga_id == nil then
        return host.json.encode({})
    end
    local _, slug = split_id(manga_id)
    if not slug then
        return host.json.encode({ id = manga_id })
    end

    local bid = build_id()
    if not bid then
        log.error("comizy: version backend unreachable")
        return nil, "upstream_unavailable"
    end
    local res = get_json(WEB .. "/_next/data/" .. bid .. "/" .. slug
        .. ".json?slug=" .. host.text.url_encode(slug))
    local m = res and (res.pageProps or {}).initialManga
    if not m then
        return host.json.encode({ id = manga_id })
    end

    local genres = {}
    for _, g in ipairs(m.genres or {}) do
        genres[#genres + 1] = g.name or g.slug
    end
    local author = ""
    if m.authors and m.authors[1] then
        author = m.authors[1].name or ""
    end
    return host.json.encode({
        id = manga_id,
        title = m.name or "",
        cover_url = m.cover or "",
        author = author,
        description = m.summary or "",
        genres = genres,
        status = host.text.normalize_status(m.status or ""),
    })
end

-- ─── ABI: get_chapter_list ─────────────────────────────────────────────
-- arg: JSON string '"{mangaId}|{slug}"'. Dedicated endpoint (no cv);
-- returns newest-first, which is the ABI order. Chapter id packs
-- "{mangaSlug}|{chapterSlug}" for get_page_list.

function get_chapter_list(arg)
    local manga_id = host.json.decode(arg)
    if manga_id == nil then
        return host.json.encode({})
    end
    local mid, slug = split_id(manga_id)
    if not mid then
        return host.json.encode({})
    end

    local res = get_json(API .. "/titles/" .. mid .. "/chapters")
    if not res then
        log.error("comizy: chapters backend unreachable")
        return nil, "upstream_unavailable"
    end
    local chs = ((res.data or {}).chapters) or {}
    local out = {}
    for _, c in ipairs(chs) do
        if c.slug then
            local title = c.name or c.slug
            out[#out + 1] = {
                id = slug .. "|" .. c.slug,
                title = title,
                chapter_num = comizy_chapter_num(title),
                released_at = c.updated_at or "",
            }
        end
    end
    -- The endpoint is already newest-first, which is the ABI order: the host
    -- sorts its cached list by chapter_num descending and its reader resolves
    -- prev/next assuming newest-first. Reversing here flipped the list.
    log.info("comizy: found " .. tostring(#out) .. " chapters")
    return host.json.encode(out)
end

-- ─── ABI: get_page_list ────────────────────────────────────────────────
-- arg: JSON string '"{mangaSlug}|{chapterSlug}"'. images[] are plain URL
-- strings; Referer comes from the host proxy (site_url fallback).

function get_page_list(arg)
    local chapter_id = host.json.decode(arg)
    if chapter_id == nil then
        return host.json.encode({})
    end
    local mslug, cslug = split_id(chapter_id)
    if not mslug then
        return host.json.encode({})
    end

    local bid = build_id()
    if not bid then
        log.error("comizy: version backend unreachable")
        return nil, "upstream_unavailable"
    end
    local res = get_json(WEB .. "/_next/data/" .. bid .. "/" .. mslug .. "/"
        .. cslug .. ".json?slug=" .. host.text.url_encode(mslug)
        .. "&chapter-slug=" .. host.text.url_encode(cslug))
    local ch = res and (res.pageProps or {}).initialChapter
    local imgs = ch and ch.images or {}
    if #imgs == 0 then
        return nil, "no_pages"
    end
    local out = {}
    for _, u in ipairs(imgs) do
        out[#out + 1] = { index = #out, url = u }
    end
    log.info("comizy: found " .. tostring(#out) .. " pages")
    return host.json.encode(out)
end
