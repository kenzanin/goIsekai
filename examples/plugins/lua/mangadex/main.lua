-- MangaDex plugin for goIsekai (Lua / Lunar VM)
-- Site: https://mangadex.org — public JSON API at api.mangadex.org
-- ABI contract version: 1 (matches pkg/types ContractVersion)

PLUGIN = {
    contract_version = 1,
    name = "MangaDex",
    site_url = "https://mangadex.org",
    logo = "logo.png",
    verify_url = "https://mangadex.org",
    needs_human_verify = false,
    thumb_ratio = 0.703,
}

local util = require("util")

-- ─── ABI: search_manga ─────────────────────────────────────────────────────
-- arg: {"query":"...","page":1}. MangaDex relevance ordering is not stable
-- across offset windows, so every result is fetched in one go and the host
-- slices it into pages.
function search_manga(arg)
    local args = host.json.decode(arg) or {}
    local title = host.text.trim(args.query or "")
    local genres = args.genres or {}
    log.info("mangadex search: q=" .. title .. " genres=" .. tostring(#genres))

    local results = {}
    local offset = 0
    local total = 999999
    -- Host invoke budget is 15s: stop sweeping in time and return partial
    -- results rather than failing the whole search.
    local deadline = os.clock() + 8

    while offset < total do
        if os.clock() > deadline then
            log.warn("mangadex search: budget reached offset=" .. tostring(offset))
            break
        end

        local qs = "limit=100&offset=" .. tostring(offset)
        qs = qs .. "&includes[]=cover_art&includes[]=author"
        qs = qs .. "&availableTranslatedLanguage[]=" .. util.LANG
        if title == "" then
            qs = qs .. "&order[followedCount]=desc"
        else
            qs = qs .. "&order[relevance]=desc&title=" .. host.text.url_encode(title)
        end
        for _, g in ipairs(genres) do
            qs = qs .. "&includedTags[]=" .. host.text.url_encode(g)
        end
        qs = qs .. "&" .. util.content_rating_params()

        local resp = util.http_get(util.API_URL .. "/manga?" .. qs)
        if not resp or resp.status < 200 or resp.status >= 300 then
            local status = resp and resp.status or 0
            log.error("mangadex search: HTTP " .. tostring(status) .. " offset=" .. tostring(offset))
            break
        end

        local body = host.json.decode(resp.body)
        if not body then
            log.error("mangadex search: unparseable response at offset=" .. tostring(offset))
            break
        end

        if body.total then total = body.total end
        local data = body.data or {}
        if #data == 0 then break end

        for _, md in ipairs(data) do
            results[#results + 1] = util.to_manga(md)
        end

        offset = offset + #data
        if #data < 100 then break end
        if #results >= 2000 then break end
    end

    log.info("mangadex search: found " .. tostring(#results) .. " results for q=" .. title)
    return host.json.encode(results)
end

-- ─── get_genres (optional export) ──────────────────────────────────────────
-- Tag ids are read from the API instead of being hardcoded, because MangaDex
-- reissues tag UUIDs and a stale list silently matches nothing. Slugs are the
-- tag UUIDs. The "genre" and "theme" groups are the ones users browse by.
local genres_cache

function get_genres()
    if not genres_cache then
        local resp = util.http_get(util.API_URL .. "/manga/tag")
        if not resp or resp.status < 200 or resp.status >= 300 then
            log.error("mangadex genres: HTTP " .. tostring(resp and resp.status or 0))
            return host.json.encode({})
        end

        local body = host.json.decode(resp.body)
        local list = {}
        for _, tag in ipairs(body and body.data or {}) do
            local a = tag.attributes or {}
            local group = a.group
            local name = (a.name or {}).en
            if tag.id and name and (group == "genre" or group == "theme") then
                list[#list + 1] = { name = name, slug = tag.id }
            end
        end

        table.sort(list, function(x, y) return x.name < y.name end)
        genres_cache = list
        log.info("mangadex genres: loaded " .. tostring(#list) .. " tags")
    end

    return host.json.encode(genres_cache)
end

-- ─── ABI: get_manga_detail ─────────────────────────────────────────────────
-- arg: '"manga-uuid"'. A failed lookup still returns an object: an empty Lua
-- table encodes as [], which cannot decode into a detail record.
function get_manga_detail(arg)
    local mangaID = host.json.decode(arg)
    if not mangaID then return host.json.encode({ id = "" }) end

    log.info("mangadex detail: id=" .. mangaID)

    local qs = "includes[]=cover_art&includes[]=author&includes[]=artist"
    local resp = util.http_get(util.API_URL .. "/manga/" .. mangaID .. "?" .. qs)
    if not resp or resp.status < 200 or resp.status >= 300 then
        return host.json.encode({ id = mangaID })
    end

    local body = host.json.decode(resp.body)
    if not body or not body.data then
        return host.json.encode({ id = mangaID })
    end

    return host.json.encode(util.to_manga(body.data))
end

-- ─── ABI: get_chapter_list ─────────────────────────────────────────────────
-- arg: '"manga-uuid"'. /feed pages at 500; external chapters are skipped
-- because they have no reader pages of their own.
function get_chapter_list(arg)
    local mangaID = host.json.decode(arg)
    if not mangaID then return host.json.encode({}) end

    log.info("mangadex chapters: id=" .. mangaID)

    local chapters = {}
    local offset = 0
    local total = 999999

    while offset < total do
        local qs = "limit=500&offset=" .. tostring(offset)
        qs = qs .. "&translatedLanguage[]=" .. util.LANG
        qs = qs .. "&order[volume]=asc&order[chapter]=asc"
        qs = qs .. "&includes[]=scanlation_group"
        qs = qs .. "&includeEmptyPages=0"
        qs = qs .. "&" .. util.content_rating_params()

        local resp = util.http_get(util.API_URL .. "/manga/" .. mangaID .. "/feed?" .. qs)
        if not resp or resp.status < 200 or resp.status >= 300 then
            log.error("mangadex chapters: HTTP " .. tostring(resp and resp.status or 0) .. " offset=" .. tostring(offset))
            break
        end

        local body = host.json.decode(resp.body)
        if not body then
            log.error("mangadex chapters: unparseable response at offset=" .. tostring(offset))
            break
        end

        if body.total then total = body.total end
        local data = body.data or {}
        if #data == 0 then break end

        for _, cd in ipairs(data) do
            local a = cd.attributes or {}
            -- Missing fields arrive as null and unset ones as "": both must be
            -- treated as absent, because "" is truthy in Lua. External chapters
            -- link off-site and have no reader pages, so they are skipped.
            local external = a.externalURL
            if not external or external == "" then
                local hasNum = a.chapter ~= nil and a.chapter ~= ""
                local hasTitle = a.title ~= nil and a.title ~= ""

                local title = "Chapter " .. tostring(a.chapter)
                if hasTitle then title = a.title end
                if not hasNum and not hasTitle then title = "Oneshot" end

                local ch = {
                    id = cd.id,
                    manga_id = mangaID,
                    title = title,
                    chapter_num = util.number(a.chapter),
                    volume_num = util.number(a.volume),
                    url = "https://mangadex.org/chapter/" .. cd.id,
                }
                -- An unparseable date is left out so the host keeps its zero
                -- value instead of failing the whole record.
                local iso = host.text.date_to_iso(a.publishAt or "")
                if iso ~= "" then ch.released_at = iso end

                chapters[#chapters + 1] = ch
            end
        end

        offset = offset + #data
        if #data < 500 then break end
    end

    -- ABI contract: chapters newest-first. The API sorts ascending.
    local newest_first = {}
    for i = #chapters, 1, -1 do
        newest_first[#newest_first + 1] = chapters[i]
    end

    log.info("mangadex chapters: found " .. tostring(#newest_first) .. " chapters for " .. mangaID)
    return host.json.encode(newest_first)
end

-- ─── ABI: get_page_list ────────────────────────────────────────────────────
-- arg: '"chapter-uuid"'. /at-home/server returns a CDN node plus a page list;
-- the CDN rejects requests without a Referer.
function get_page_list(arg)
    local chapterID = host.json.decode(arg)
    if not chapterID then return host.json.encode({}) end

    log.info("mangadex pages: chapter=" .. chapterID)

    local resp = util.http_get(util.API_URL .. "/at-home/server/" .. chapterID)
    if not resp or resp.status < 200 or resp.status >= 300 then
        return host.json.encode({})
    end

    local body = host.json.decode(resp.body)
    if not body or not body.chapter or not body.chapter.data then
        return host.json.encode({})
    end

    local pages = {}
    for i, file in ipairs(body.chapter.data) do
        pages[#pages + 1] = {
            index = i - 1,
            url = body.baseUrl .. "/data/" .. body.chapter.hash .. "/" .. file,
            headers = { ["Referer"] = util.CDN_URL .. "/" },
        }
    end

    log.info("mangadex pages: found " .. tostring(#pages) .. " pages for " .. chapterID)
    return host.json.encode(pages)
end
