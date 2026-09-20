-- KaliScan plugin for goIsekai
-- Site: https://kaliscan.io
-- ABI contract version: 1 (matches pkg/types ContractVersion)

PLUGIN = {
    contract_version = 1,
    name = "Kaliscan",
    site_url = "https://kaliscan.com",
    logo = "logo.png",
    verify_url = "https://kaliscan.io",
    needs_human_verify = false,
    thumb_ratio = 0.703,
}

BASE = "https://kaliscan.io"

local util = require("util")

-- MAX_RESULT_PAGES bounds how much of the result set one search pulls down.
-- Every page is a separate request costing roughly a second, and the host gives
-- a plugin invocation 15s in total: a broad query like "one" spans 27 pages, so
-- walking to the end never finishes and the search fails outright. The host
-- paginates whatever comes back, so a fixed window still fills several pages.
local MAX_RESULT_PAGES = 5

-- search_manga(arg) — arg is a JSON object: {"query":"...","page":1}
-- Returns: array of {id, title, cover_url} + "total" field
function search_manga(arg)
    local args = host.json.decode(arg)
    local query = args.query or ""
    local page = tonumber(args.page) or 1
    local genres = args.genres or {}
    log.debug("search q=" .. query .. " genres=" .. tostring(#genres))


    -- Genre browsing: GET /genres/<slug> (page N: /genres/<slug>?page=N);
    -- archive reuses the search card markup, so util.parse_search handles it.
    local body
    if #genres > 0 then
        local gurl = BASE .. "/genres/" .. host.text.url_encode(genres[1])
        if page > 1 then
            gurl = gurl .. "?page=" .. tostring(page)
        end
        body = host.http.get_body(gurl)
        if not body then
            return host.json.encode({})
        end
        return host.json.encode(util.parse_search(body).results)
    end

    -- Fetch page 1 to discover the page count, then walk up to the cap.
    body = host.http.get_body(BASE .. "/search?q=" .. host.text.url_encode(query) .. "&page=1")
    if not body then
        return host.json.encode({})
    end
    local first = util.parse_search(body)
    local all = first.results
    local max_page = first.total or 1
    local last_page = math.min(max_page, MAX_RESULT_PAGES)

    for p = 2, last_page do
        local page_body = host.http.get_body(BASE .. "/search?q=" .. host.text.url_encode(query) .. "&page=" .. tostring(p))
        if not page_body then
            break
        end
        local page_results = util.parse_search(page_body).results
        if #page_results == 0 then
            break
        end
        for _, r in ipairs(page_results) do
            all[#all + 1] = r
        end
    end

    log.debug("search: found " .. #all .. " results for q=" .. query .. " (pages=fetched " .. last_page .. "/" .. max_page .. ")")
    return host.json.encode(all)
end

-- get_manga_detail(arg) — arg is a JSON-encoded plain string (e.g. '"104-love-shuttle"')
-- Returns: {id, title, author, description, cover_url, genres, status}
function get_manga_detail(arg)
    local manga_id = host.json.decode(arg) -- yields a plain string
    local body = host.http.get_body(BASE .. "/manga/" .. manga_id)
    if not body then
        -- empty table encodes as []; detail must stay an OBJECT — emit {id} only
        return host.json.encode({id = manga_id})
    end
    return host.json.encode(util.parse_manga_detail(body, manga_id))
end

-- get_chapter_list(arg) — arg is a JSON-encoded plain string (e.g. '"104-love-shuttle"')
-- Returns: array of {id, number, title, uploaded_at}
function get_chapter_list(arg)
    local manga_id = host.json.decode(arg) -- yields a plain string
    local body = host.http.get_body(BASE .. "/manga/" .. manga_id)
    if not body then
        return host.json.encode({})
    end
    return host.json.encode(util.parse_chapter_list(body, manga_id))
end

-- get_page_list(arg) — arg is a JSON-encoded plain string (e.g. '"104-love-shuttle/chapter-98"')
-- Returns: array of {url}
function get_page_list(arg)
    local chapter_path = host.json.decode(arg) -- e.g. "SLUG:chapter-98"
    chapter_path = chapter_path:gsub(":", "/") -- restore real path
    -- Step 1: fetch the chapter page to extract the numeric chapterId
    local body = host.http.get_body(BASE .. "/manga/" .. chapter_path)
    if not body then
        return host.json.encode({})
    end
    local chapter_id = host.regex.find(body, [[chapterId\s*=\s*(\d+)]])
    if not chapter_id then
        return host.json.encode({})
    end
    -- Step 2: fetch page images from the chapter server (requires Referer)
    local api_url = BASE .. "/service/backend/chapterServer/?server_id=1&chapter_id=" .. chapter_id
    local img_body = host.http.get_body(api_url, {
        ["Referer"] = BASE .. "/manga/" .. chapter_path,
        ["X-Requested-With"] = "XMLHttpRequest"
    })
    if not img_body then
        return host.json.encode({})
    end
    return host.json.encode(util.parse_page_list(img_body, BASE .. "/manga/" .. chapter_path))
end


-- ─── get_genres (optional export) ──────────────────────────────────────────
-- Slugs from the site's GENRES nav (/genres/<slug> archive pages).
local GENRES = {
    { name = "Action", slug = "action" },
    { name = "Adaptation", slug = "adaptation" },
    { name = "Adventure", slug = "adventure" },
    { name = "Anthology", slug = "anthology" },
    { name = "Comedy", slug = "comedy" },
    { name = "Cooking", slug = "cooking" },
    { name = "Demons", slug = "demons" },
    { name = "Drama", slug = "drama" },
    { name = "Ecchi", slug = "ecchi" },
    { name = "Fantasy", slug = "fantasy" },
    { name = "Full Color", slug = "full-color" },
    { name = "Game", slug = "game" },
    { name = "Gender bender", slug = "gender-bender" },
    { name = "Ghosts", slug = "ghosts" },
    { name = "Harem", slug = "harem" },
    { name = "Historical", slug = "historical" },
    { name = "Horror", slug = "horror" },
    { name = "Isekai", slug = "isekai" },
    { name = "Josei", slug = "josei" },
    { name = "Magic", slug = "magic" },
    { name = "Manhua", slug = "manhua" },
    { name = "Manhwa", slug = "manhwa" },
    { name = "Martial arts", slug = "martial-arts" },
    { name = "Mature", slug = "mature" },
    { name = "Mecha", slug = "mecha" },
    { name = "Medical", slug = "medical" },
    { name = "Military", slug = "military" },
    { name = "Monster girls", slug = "monster-girls" },
    { name = "Monsters", slug = "monsters" },
    { name = "Music", slug = "music" },
    { name = "Mystery", slug = "mystery" },
    { name = "Office workers", slug = "office-workers" },
    { name = "One shot", slug = "one-shot" },
    { name = "Police", slug = "police" },
    { name = "Psychological", slug = "psychological" },
    { name = "Reincarnation", slug = "reincarnation" },
    { name = "Romance", slug = "romance" },
    { name = "School life", slug = "school-life" },
    { name = "Sci fi", slug = "sci-fi" },
    { name = "Seinen", slug = "seinen" },
    { name = "Shoujo", slug = "shoujo" },
    { name = "Shoujo ai", slug = "shoujo-ai" },
    { name = "Shounen", slug = "shounen" },
    { name = "Shounen ai", slug = "shounen-ai" },
    { name = "Slice of life", slug = "slice-of-life" },
    { name = "Smut", slug = "smut" },
    { name = "Sports", slug = "sports" },
    { name = "Super Power", slug = "super-power" },
    { name = "Supernatural", slug = "supernatural" },
    { name = "Thriller", slug = "thriller" },
    { name = "Time travel", slug = "time-travel" },
    { name = "Tragedy", slug = "tragedy" },
    { name = "Vampire", slug = "vampire" },
    { name = "Video games", slug = "video-games" },
    { name = "Villainess", slug = "villainess" },
    { name = "Webtoons", slug = "webtoons" },
    { name = "Yaoi", slug = "yaoi" },
    { name = "Yuri", slug = "yuri" },
    { name = "Zombies", slug = "zombies" },
}

function get_genres()
    return host.json.encode(GENRES)
end
