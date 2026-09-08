-- Mangafreak plugin for goIsekai (Lua / Lunar VM)
-- Site: https://www.mangafreak.me (standalone HTML scraping, NOT Madara)
-- ABI contract version: 1

PLUGIN = {
    contract_version = 1,
    name = "Mangafreak",
    site_url = "https://www.mangafreak.me",
    logo = "logo.png",
    verify_url = "https://www.mangafreak.me",
    needs_human_verify = false,
    thumb_ratio = 0.703,
    search_page_size = 20,
    alt_title_servers = {
        {id = "mangadex",     name = "MangaDex",     kind = "titles"},
        {id = "mangaupdates", name = "MangaUpdates", kind = "both"}
    },
}

local BASE = "https://www.mangafreak.me"
local util = require("util")

-- ─── ABI: search_manga ─────────────────────────────────────────────────────
-- arg: {"query":"...","page":1}
-- Search URL: GET /Find/QUERY (path-based, NOT query-param)
-- Results: div.manga_search_item or div.mangaka_search_item
-- No pagination support (hasNextPage always false in Kotlin source)

function search_manga(arg)
    local args = json.decode(arg)
    local query = args.query or ""
    log.debug("mangafreak search q=" .. query)

    if query == "" then
        return json.encode({})
    end

    local url = BASE .. "/Find/" .. util.url_encode(query)
    local resp = util.http_get(url)
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end

    local results = util.parse_search(resp.body)
    log.debug("mangafreak search: found " .. #results .. " results for q=" .. query)
    return json.encode(results)
end

-- ─── ABI: get_manga_detail ─────────────────────────────────────────────────
-- arg: '"slug"' (JSON-encoded plain string)
-- Detail page: GET /manga/SLUG
-- Returns: {id, title, author, artist, description, cover_url, genres, status}

function get_manga_detail(arg)
    local manga_id = json.decode(arg)
    local resp = util.http_get(BASE .. "/Manga/" .. manga_id)
    if not resp or resp.status ~= 200 then
        return json.encode({ id = manga_id })
    end
    return json.encode(util.parse_manga_detail(resp.body, manga_id))
end

-- ─── ABI: get_chapter_list ─────────────────────────────────────────────────
-- arg: '"slug"' (JSON-encoded plain string)
-- Chapters are on the same page as detail: GET /manga/SLUG
-- Table rows in div.manga_series_list: td:eq(0)=name, td:eq(1)=date
-- Reversed to newest-first in util.parse_chapter_list

function get_chapter_list(arg)
    local manga_id = json.decode(arg)
    local resp = util.http_get(BASE .. "/Manga/" .. manga_id)
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    local chapters = util.parse_chapter_list(resp.body, manga_id)
    log.debug("mangafreak chapters slug=" .. manga_id .. " count=" .. tostring(#chapters))
    return json.encode(chapters)
end

-- ─── ABI: get_page_list ────────────────────────────────────────────────────
-- arg: '"SLUG:chapter-PATH"' (JSON-encoded, colon = slash)
-- Chapter page: GET /manga/SLUG/chapter-PATH
-- Images: img#gohere[src] — all img with id="gohere" and src attribute

function get_page_list(arg)
    local chapter_id = json.decode(arg) -- "SLUG:chapter-PATH"
    -- Extract Read1_ path from "SLUG:Read1_SLUG_CHNUM"
    local _, _, ch_path = chapter_id:find(":(.+)$")
    local url = BASE .. "/Read1_" .. ch_path
    local resp = util.http_get(url)
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    local pages = util.parse_page_list(resp.body)
    log.debug("mangafreak pages " .. chapter_id .. " count=" .. tostring(#pages))
    return json.encode(pages)
end

