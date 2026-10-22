-- main.lua — BatCave (https://batcave.biz) plugin for goIsekai
-- ABI: contract_version 1. Entry points take a JSON string and return a JSON
-- string. HTTP failures return an empty result with nil error (host
-- convention) and never surface as Lua errors.
--
-- Site profile: DLE (DataLifeEngine) custom "guard" JS, NOT Cloudflare.
--   GET renders an inline 404-challenge page carrying a pow token; the
--   plugin solves it MURNI over HTTP via dle.lua (sha256(token..":"..nonce)
--   hex prefix "00") and POSTs /_v with rtip set — an empty rtip escalates
--   to Turnstile, so a stable fake is sent. The resulting __guard_* trust
--   cookie lands in the host's per-plugin cookie jar and is reused by every
--   later request in the call (images included).
--
-- Data shapes (verified against batcave.har):
--   search text:  GET /search/<query>/page/<N>  -> cards div.readed__*  (regex only)
--   search filter: GET|POST /ComicList/<filters>/page/<N>  -> __XFILTER__ drives the filter form
--   detail:      GET /<news_id>-<slug>.html      -> window.__DATA__ + JSON-LD ComicSeries
--   chapters:    __DATA__ .chapters[] {id,posi,pages,title,date,broken}  (posi = number)
--   pages:       reader GET /reader/<news_id>/<chapter_id> -> __DATA__ .images[] (| .pages count)
--
-- The reader TRUNCATES images to `pages` (if(pages < images.length) images.length = pages),
-- so a chapter reporting images=21 but pages=20 must yield exactly 20 page records.

local dle = require("dle")
local BASE = host.plugin.param("site_url")

PLUGIN = {
    contract_version = 1,
    name = "BatCave",
    site_url = BASE,
    logo = "logo.png",
    needs_js = false,
    thumb_ratio = 0.75,
}

-- ─── helpers ────────────────────────────────────────────────────────────────

-- site_get fetches path and solves the DLE guard if present (per dle.lua).
-- dle.get concatenates site .. path, so an absolute URL must be reduced to its
-- path first or the origin doubles (https://sitehttps://site/...).
local function site_get(path)
    if path:match("^https?://") then
        path = path:match("^https?://[^/]+(/.*)$") or "/"
    end
    return dle.get(BASE, path)
end

-- json_data extracts window.__DATA__ = <json> (the reader/detail envelope).
local function json_data(html)
    return dle.json_blob(html, "window.__DATA__ = ")
end

-- abs_url turns "/path" into a full URL anchored at site, leaving absolutes alone.
local function abs_url(site, path)
    if not path or path == "" then return "" end
    if path:match("^https?://") then return path end
    return site .. path
end

-- path_of turns a manga id into a site path. The id is the DLE slug
-- ("29706-thor-1966-2011.html"); a full permalink is accepted too.
local function path_of(id)
    if id:match("^https?://") then
        return id:match("^https?://[^/]+(/.*)$") or "/"
    end
    if id:sub(1, 1) ~= "/" then return "/" .. id end
    return id
end

-- manga_id_of reduces a permalink to the slash-free DLE id. The host routes
-- manga pages as /view/manga/{pluginID}/{mangaID} - one path segment - so an
-- id carrying a slash would split the route and 404.
local function manga_id_of(href)
    return path_of(href):sub(2)
end

-- parse_readed_cards pulls div.readed cards into {id,title,cover_url}.
-- Card shape:
--   <div class="readed d-flex ..."><a class="readed__img ..." href="/NNN-slug.html">
--     <img data-src="/uploads/mini/100x150/xx/hash.webp" ... alt="Title">
--   </a><div class="readed__desc ..."><h2 class="readed__title"><a href="...">Title</a></h2>
local function parse_readed_cards(body)
    local cards = {}
    -- The card anchor carries href BEFORE class:
    --   <a href="https://site/NNN-slug.html" class="readed__img img-fit-cover anim">
    --     <img data-src="/uploads/mini/.../h.webp" ... alt="Title">
    -- So the pattern matches href, then class, then the img's data-src/alt.
    for href, data_src, title in host.regex.gmatch(body,
        [[(?s)<a\s+href="([^"]+)"\s+class="readed__img[^"]*"[^>]*>\s*<img[^>]*data-src="([^"]+)"[^>]*alt="([^"]*)"]]) do
        if dle.slug_id(href) then
            cards[#cards + 1] = {
                id = manga_id_of(href),
                title = host.text.html_decode(title),
                cover_url = abs_url(BASE, data_src),
            }
        end
    end
    return cards
end

-- normalize_status maps the site's "Release type" wording to the host vocabulary.
local function normalize_status(raw)
    local s = host.text.trim(raw or ""):lower()
    if s == "completed" or s == "finished" or s == "finish" then return "Completed" end
    if s == "ongoing" or s == "publish" or s == "releasing" or s == "published" then return "Ongoing" end
    if s == "hiatus" or s == "onhold" or s == "on hold" then return "Hiatus" end
    if s == "dropped" or s == "cancelled" or s == "canceled" then return "Dropped" end
    if s == "upcoming" or s == "not yet published" then return "Upcoming" end
    return host.text.trim(raw or "")
end

-- The id this plugin emits is the full permalink (e.g.
-- https://batcave.biz/29706-thor-1966-2011.html); it round-trips through
-- search -> detail -> chapter list -> page list without re-encoding.
-- dle.slug_id(permalink) still yields (news_id, slug) for building reader URLs.

-- ─── ABI: search_manga(arg) ────────────────────────────────────────────────
-- arg: {"query":"...","page":1,"genres":[...]}
-- Text search: GET /search/<query>/page/<N>. Category/genre browse:
-- GET|POST /ComicList/<filters>/page/<N>. The page is server-rendered with
-- cards; the embedded __COMICS__ blob is malformed, so cards are regex-parsed.

function search_manga(arg)
    local a = host.json.decode(arg)
    local query = (a and a.query) or ""
    local page = tonumber(a and a.page or 1)
    local genres = (a and a.genres) or {}
    log.debug("batcave search q=" .. tostring(query) .. " page=" .. tostring(page) .. " genres=" .. tostring(#genres))

    local body
    if #genres > 0 then
        -- Genre/category browse uses ComicList with g=<id> path. First genre wins.
        local id = tonumber(genres[1]:match("(%d+)"))
        if not id then
            return host.json.encode({})
        end
        local url = "/ComicList/g=" .. id .. "/"
        if page > 1 then url = url .. "page/" .. tostring(page) .. "/" end
        body = site_get(url)
    elseif query ~= "" then
        local url = "/search/" .. host.text.url_encode(query)
        if page > 1 then url = url .. "/page/" .. tostring(page) end
        body = site_get(url)
    else
        return host.json.encode({})
    end

    if not body or body == "" then
        return host.json.encode({})
    end
    local out = parse_readed_cards(body)
    log.debug("batcave search: found " .. #out .. " results")
    return host.json.encode(out)
end

-- ─── ABI: get_manga_detail(arg) ────────────────────────────────────────────
-- arg: '"<news_id-slug>"' (the id emitted by search = full href)
-- Detail: GET /<news_id-slug>.html -> window.__DATA__ (no chapters there;
-- the detail body embeds the chapter Vue component with __DATA__" chapters).
-- JSON-LD ComicSeries is the fallback source for description/cover/genres.

function get_manga_detail(arg)
    local manga_id = host.json.decode(arg)
    if not manga_id then
        return host.json.encode({})
    end
    -- manga_id from search is a full href; normalize to a path.
    local path = path_of(manga_id)
    local body = site_get(path)
    if not body or body == "" then
        log.error("batcave: detail fetch failed: " .. manga_id)
        return host.json.encode({})
    end

    local title = host.regex.find(body, [[<header class="page__header">\s*<div class="page__title-row">\s*<h1>([^<]+)</h1>]]) or ""
        or host.regex.find(body, [[<title>([^<]+)</title>]]) or ""
    title = host.text.html_decode(title):gsub("%s*BatCave.*$", "")
    title = host.text.trim(title)

    -- The poster <img> is sometimes wrapped in an <a> ("Open full cover"),
    -- so the direct child is not always the <img>.
    local cover = host.regex.find(body, [[<div class="page__poster[^"]*"[^>]*>\s*<img[^>]*src="([^"]+)"]]) or ""
    if cover == "" then
        cover = host.regex.find(body, [[<div class="page__poster[^"]*"[^>]*>\s*<a[^>]*>\s*<img[^>]*src="([^"]+)"]]) or ""
    end
    cover = abs_url(BASE, cover)

    local desc = host.regex.find(body, [[<div class="page__text full-text clearfix">(.*?)</div><div class="page__tags]]) or ""
    desc = host.text.strip_html(desc)
    if desc == "" then desc = host.regex.find(body, [[<meta name="description" content="([^"]*)"]]) or "" end
    desc = host.text.html_decode(desc)

    -- Writer row in ul.page__list: <li><div>Writer:</div> <a href=... rel=nofollow>Stan Lee</a></li>
    local a = host.regex.find(body, [[<div>Writer:</div>\s*<a[^>]*>([^<]+)</a>]]) or ""

    local status_raw = host.regex.find(body, [[Release type:</div>\s*([^<]+)</li>]]) or ""
    local status = normalize_status(status_raw)

    -- Genres: prefer the page__tags <a> labels, fall back to JSON-LD genre.
    local genres = {}
    for g in host.regex.gmatch(body, [[<div class="page__tags[^"]*">\s*(.*?)<div class="char-block"]]) do
        for label in host.regex.gmatch(g, [[<a[^>]*>([^<]+)</a>]]) do
            label = host.text.trim(label)
            if label ~= "" then genres[#genres + 1] = label end
        end
    end
    if #genres == 0 then
        local ld = dle.comic_series(body)
        if ld and type(ld.genre) == "string" then
            for g in host.regex.gmatch(ld.genre, "([^,]+)") do
                g = host.text.trim(g)
                if g ~= "" then genres[#genres + 1] = g end
            end
        end
    end

    local detail = {
        id = manga_id,
        title = title,
        cover_url = cover,
        author = a,
        description = desc,
        genres = genres,
        status = status,
    }
    log.debug("batcave detail: " .. title .. " | " .. status .. " | " .. #genres .. " genres")
    return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list(arg) ────────────────────────────────────────────
-- arg: '"<path>"' (the manga path emitted by detail / search)
-- Chapters live in window.__DATA__ on the detail page: chapters[] with
-- {id, posi, pages, title, date}. posi is the chapter number (1-based asc),
-- so the array is already newest-first — keep document order.

function get_chapter_list(arg)
    local manga_id = host.json.decode(arg)
    local path = path_of(manga_id)
    local body = site_get(path)
    if not body or body == "" then
        return host.json.encode({})
    end
    local data = json_data(body)
    if not data then
        log.error("batcave: no __DATA__ in chapter list: " .. path)
        return host.json.encode({})
    end
    local ch = data.chapters
    if type(ch) ~= "table" then
        return host.json.encode({})
    end
    local out = {}
    for _, c in ipairs(ch) do
        if not c.broken then
            out[#out + 1] = {
                -- The id IS the reader path: get_page_list gets it back as a
                -- plain JSON string, and the reader URL needs news_id + chapter.
                id = "/reader/" .. tostring(data.news_id) .. "/" .. tostring(c.id) .. (data.xhash or ""),
                manga_id = manga_id,
                chapter_num = tonumber(c.posi) or host.text.chapter_num(c.title or ""),
                title = c.title or ("Issue #" .. tostring(c.id)),
                released_at = dle.dle_date(c.date),
                url = "/reader/" .. tostring(data.news_id) .. "/" .. tostring(c.id) .. (data.xhash or ""),
            }
        end
    end
    log.debug("batcave chapters: " .. #out .. " for " .. path)
    return host.json.encode(out)
end

-- ─── ABI: get_page_list(arg) ───────────────────────────────────────────────
-- arg: '"/reader/<news_id>/<chapter_id>"' (the chapter id, as a plain string)
-- GET /reader/<news_id>/<chapter_id> -> window.__DATA__ .images[] (| .pages).
-- The reader truncates images to `pages`; honor that so we don't emit a
-- phantom 21st page. Images are absolute (img.batcave.biz / komacdn).
function get_page_list(arg)
    local reader = host.json.decode(arg)
    if type(reader) ~= "string" or reader == "" then
        return host.json.encode({})
    end
    local body = site_get(reader)
    if not body or body == "" then
        return host.json.encode({})
    end
    local data = json_data(body)
    if not data then
        log.error("batcave: no __DATA__ in reader: " .. reader)
        return host.json.encode({})
    end
    local images = data.images or {}
    local pages = tonumber(data.pages) or #images
    if pages > #images then pages = #images end
    local out = {}
    for i = 1, pages do
        local u = images[i]
        if u then
            out[#out + 1] = { index = i - 1, url = u }
        end
    end
    log.debug("batcave pages: " .. #out .. " for " .. reader)
    return host.json.encode(out)
end

-- ─── get_genres(arg) ───────────────────────────────────────────────────────
-- GET /comix/ once to grab __XFILTER__'s "g" list (genre id -> name).
function get_genres(_)
    local body = site_get("/comix/")
    if not body or body == "" then
        return host.json.encode({})
    end
    local f = dle.json_blob(body, "__XFILTER__ = ")
    if not f or not f.filter_items then
        return host.json.encode({})
    end
    local items = f.filter_items.g
    if not items or not items.values then
        return host.json.encode({})
    end
    local out = {}
    for _, v in ipairs(items.values) do
        -- ABI: {name, slug}. The slug is the DLE genre id, which is what
        -- search_manga feeds back into /ComicList/g=<id>/.
        out[#out + 1] = { name = v.value, slug = tostring(v.id) }
    end
    log.debug("batcave genres: " .. #out)
    return host.json.encode(out)
end
