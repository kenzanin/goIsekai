-- main.lua — MangaBay (https://manga-bay.biz) plugin for goIsekai
-- ABI: contract_version 1. Entry points take a JSON string and return a JSON
-- string. HTTP failures return an empty result with nil error (host
-- convention) and never surface as Lua errors.
--
-- Site profile: mirrors BatCave — same DataLifeEngine fork, the SAME custom
-- "guard" JS (GET -> 404 challenge with a pow token -> POST /_v + POW ->
-- __guard_* trust cookie) and the same window.__DATA__ envelope on detail and
-- reader pages. The shared dle.lua module (copied from batcave) solves the
-- guard purely over HTTP, so no browser/CDP path is needed.
--
-- Verified against manga-bay.har:
--   search:  GET /search/<query>/page/<N>  -> div.readed cards (same as batcave)
--   browse:  GET /ComicList/[g=<id>/|or=1]/page/<N>  -> readed cards
--   detail:  GET /<news_id>-<slug>.html   -> window.__DATA__ + JSON-LD ComicSeries
--   chapters: __DATA__.chapters[] {id,posi,pages,title,date,broken}
--            posi is the chapter number; array is newest-first already.
--   pages:   GET /reader/<news_id>/<chapter_id> -> __DATA__.images[] (| .pages)
--            reader truncates images to `pages`; honor it (images=9, pages=8).
--   genres:  GET /comix/ -> window.__XFILTER__ .filter_items.g.values [{id,value}]
--
-- The id this plugin emits is the full permalink (https://manga-bay.biz/<id>-<slug>.html);
-- it round-trips through search -> detail -> chapter list -> page list.

local dle = require("dle")
local BASE = host.plugin.param("site_url")

PLUGIN = {
    contract_version = 1,
    name = "MangaBay",
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

-- abs_url turns "/path" into a full URL anchored at BASE.
local function abs_url(path)
    if not path then return "" end
    if path:match("^https?://") then return path end
    return BASE .. path
end

-- path_of turns a manga id into a site path. The id is the DLE slug
-- ("11222-dungeon-meshi.html"); a full permalink is accepted too.
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

-- parse_cd_cards pulls a.cd cards into {id,title,cover_url}.
-- Manga-bay's search and ComicList pages do NOT use batcave's readed__ markup;
-- they render (verified in manga-bay.har):
--   <a href="/NNN-slug.html" class="cd" title="Title">
--     <span class="cd__ph "><img data-src="/uploads/mini/142x213/xx/h.webp" alt="Title">
local function parse_cd_cards(body)
    local cards = {}
    for href, title, data_src in host.regex.gmatch(body,
        [[(?s)<a\s+href="([^"]+)"\s+class="cd"\s+title="([^"]*)">\s*<span class="cd__ph[^"]*">\s*<img[^>]*data-src="([^"]+)"]] ) do
        if dle.slug_id(href) then
            cards[#cards + 1] = {
                id = manga_id_of(href),
                title = host.text.html_decode(title),
                cover_url = abs_url(data_src),
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

-- ─── ABI: search_manga(arg) ────────────────────────────────────────────────
-- arg: {"query":"...","page":1,"genres":[...]}
-- Text search: GET /search/<query>/page/<N>. Genre/category browse:
-- GET /ComicList/[g=<id>/|or=1]/page/<N>. Both render div.readed cards.

function search_manga(arg)
    local a = host.json.decode(arg)
    local query = (a and a.query) or ""
    local page = tonumber(a and a.page or 1)
    local genres = (a and a.genres) or {}
    log.debug("mangabay search q=" .. tostring(query) .. " page=" .. tostring(page) .. " genres=" .. tostring(#genres))

    local body
    if #genres > 0 then
        -- Genre browse: /ComicList/g=<id>/page/<N>/. First genre id wins.
        local id = tonumber(genres[1]:match("(%d+)"))
        if not id then
            return host.json.encode({})
        end
        local url = "/ComicList/g=" .. tostring(id)
        if page > 1 then url = url .. "/page/" .. tostring(page) .. "/" end
        url = url .. "/"
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
    local out = parse_cd_cards(body)
    log.debug("mangabay search: found " .. #out .. " results")
    return host.json.encode(out)
end

-- ─── ABI: get_manga_detail(arg) ────────────────────────────────────────────
-- arg: '"<permalink>"'
-- Detail: GET /<news_id>-<slug>.html -> window.__DATA__ (holds chapters +
-- news_id) and JSON-LD ComicSeries (description/cover/genre). Meta rows in
-- div.page__list give author/artist/status.

function get_manga_detail(arg)
    local manga_id = host.json.decode(arg)
    if not manga_id then
        return host.json.encode({})
    end
    local body = site_get(path_of(manga_id))
    if not body or body == "" then
        log.error("mangabay: detail fetch failed: " .. manga_id)
        return host.json.encode({})
    end

    -- Manga-bay's detail markup (verified live, differs from batcave's):
    --   <header class="page__header"><h1>Dungeon Meshi</h1><p class="page__alt">alt titles</p>
    --   <dl class="page__list page__list--main"><div><dt>Status</dt><dd>Completed</dd></div>...
    --   <div class="page__desc"><div class="page__desc-text full-text clearfix">...</div>
    local title = host.regex.find(body, [[<header class="page__header">\s*<h1>([^<]+)</h1>]]) or ""
        or host.regex.find(body, [[<title>([^<]+)</title>]]) or ""
    title = host.text.html_decode(title):gsub("%s*Manga%-?Bay.*$", "")
    title = host.text.trim(title)

    -- Alt titles: <p class="page__alt">A; B; C</p>
    local alt_titles = host.regex.find(body, [[<p class="page__alt">(.*?)</p>]]) or ""

    -- The poster <img> is sometimes wrapped in an <a> ("Open full cover"),
    -- so the direct child is not always the <img>.
    local cover = host.regex.find(body, [[<div class="page__poster[^"]*"[^>]*>\s*<img[^>]*src="([^"]+)"]]) or ""
    if cover == "" then
        cover = host.regex.find(body, [[<div class="page__poster[^"]*"[^>]*>\s*<a[^>]*>\s*<img[^>]*src="([^"]+)"]]) or ""
    end
    cover = abs_url(cover)

    local desc = host.regex.find(body, [[<div class="page__desc-text full-text clearfix">(.*?)</div>]]) or ""
    desc = host.text.strip_html(desc)
    desc = host.text.html_decode(desc)

    -- page__list rows: <div><dt>Author</dt><dd>Kui Ryouko</dd></div>
    local author = host.regex.find(body, [[<dt>Author</dt><dd>([^<]+)</dd>]]) or ""
    local artist = host.regex.find(body, [[<dt>Artist</dt><dd>([^<]+)</dd>]]) or ""

    local status_raw = host.regex.find(body, [[<dt>Status</dt><dd>([^<]+)</dd>]]) or ""
    local status = normalize_status(status_raw)

    -- Genres: the <dt>Genres</dt><dd> row carries one <a> per genre.
    local genres = {}
    local genre_row = host.regex.find(body, [[<dt>Genres</dt><dd>(.*?)</dd>]]) or ""
    for label in host.regex.gmatch(genre_row, [[<a[^>]*>([^<]+)</a>]]) do
        label = host.text.trim(label)
        if label ~= "" then genres[#genres + 1] = label end
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

    -- The alt-title line holds the series' other names; keep it as a tail of
    -- the description so it is not lost (the ABI has one description field).
    if alt_titles ~= "" then
        alt_titles = host.text.trim(host.text.html_decode(alt_titles))
        if alt_titles ~= "" then
            desc = desc .. "\n\nAlternative titles: " .. alt_titles
        end
    end

    local detail = {
        id = manga_id,
        title = title,
        cover_url = cover,
        author = author,
        description = desc,
        genres = genres,
        status = status,
    }
    -- artist is an extra; emit under author only if writer absent (both are Kui Ryouko here).
    if author == "" and artist ~= "" then detail.author = artist end
    log.debug("mangabay detail: " .. title .. " | " .. status .. " | " .. #genres .. " genres")
    return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list(arg) ────────────────────────────────────────────
-- arg: '"<permalink>"'
-- Chapters: GET /<news_id>-<slug>.html -> window.__DATA__ .chapters[] {id,posi,pages,title,date}.
-- posi = chapter number (ascending); the array is newest-first (posi desc).

function get_chapter_list(arg)
    local manga_id = host.json.decode(arg)
    local body = site_get(path_of(manga_id))
    if not body or body == "" then
        return host.json.encode({})
    end
    local data_holder = dle.json_blob(body, "window.__DATA__ = ")
    if not data_holder then
        log.error("mangabay: no __DATA__ in chapter list: " .. manga_id)
        return host.json.encode({})
    end
    local ch = data_holder.chapters
    if type(ch) ~= "table" then
        return host.json.encode({})
    end
    local out = {}
    for _, c in ipairs(ch) do
        if not c.broken then
            out[#out + 1] = {
                -- The id IS the reader path: get_page_list receives it back as a
                -- plain JSON string, and the reader URL needs both the news_id
                -- and the chapter id.
                id = "/reader/" .. tostring(data_holder.news_id) .. "/" .. tostring(c.id),
                manga_id = manga_id,
                chapter_num = tonumber(c.posi) or host.text.chapter_num(c.title or ""),
                title = c.title or ("Issue #" .. tostring(c.id)),
                released_at = dle.dle_date(c.date),
                url = "/reader/" .. tostring(data_holder.news_id) .. "/" .. tostring(c.id),
            }
        end
    end
    log.debug("mangabay chapters: " .. #out .. " for " .. manga_id)
    return host.json.encode(out)
end

-- ─── ABI: get_page_list(arg) ───────────────────────────────────────────────
-- arg: '"/reader/<news_id>/<chapter_id>"' (the chapter id, as a plain string)
-- GET -> window.__DATA__ .images[] (| .pages). The reader truncates images
-- to `pages` (images=9, pages=8 in the probe), so cap to avoid a phantom page.

function get_page_list(arg)
    -- arg is the reader path emitted as a chapter id ("/reader/11222/659063"),
    -- handed back as a plain JSON string.
    local reader = host.json.decode(arg)
    if type(reader) ~= "string" or reader == "" then
        return host.json.encode({})
    end
    local body = site_get(path_of(reader))
    if not body or body == "" then
        return host.json.encode({})
    end
    local data = dle.json_blob(body, "window.__DATA__ = ")
    if not data then
        log.error("mangabay: no __DATA__ in reader: " .. reader)
        return host.json.encode({})
    end
    local images = data.images or {}
    local pages = tonumber(data.pages) or #images
    if pages > #images then pages = #images end
    local out = {}
    for i = 1, pages do
        local u = images[i]
        if u then
            -- komacdn.com hotlink-guards its pages: it answers 403 for a
            -- missing Referer, for the CDN's own origin, and for the site
            -- URL without its trailing slash. Only "<site>/" is accepted.
            out[#out + 1] = { index = i - 1, url = u, headers = { Referer = BASE .. "/" } }
        end
    end
    log.debug("mangabay pages: " .. #out .. " for " .. reader)
    return host.json.encode(out)
end

-- ─── get_genres(arg) ───────────────────────────────────────────────────────
-- GET /comix/ once -> window.__XFILTER__ .filter_items.g.values [{id,value}].

function get_genres(_)
    local body = site_get("/comix/")
    if not body or body == "" then
        return host.json.encode({})
    end
    local f = dle.json_blob(body, "__XFILTER__ = ")
    if not f or not f.filter_items or not f.filter_items.g or not f.filter_items.g.values then
        return host.json.encode({})
    end
    local out = {}
    for _, v in ipairs(f.filter_items.g.values) do
        -- ABI: {name, slug}. The slug is the DLE genre id, which is what
        -- search_manga feeds back into /ComicList/g=<id>/.
        out[#out + 1] = { name = v.value, slug = tostring(v.id) }
    end
    log.debug("mangabay genres: " .. #out)
    return host.json.encode(out)
end
