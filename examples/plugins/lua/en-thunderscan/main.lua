-- En-Thunderscans plugin for goIsekai (Lua / Lunar VM)
-- Site: https://en-thunderscans.com (WordPress mangareader theme, bsx cards)
-- ABI contract version: 1

-- The origin lives in plugin.json only; PLUGIN and BASE both read it, so a
-- site move is one edit there instead of three strings kept in sync by hand.
BASE = host.plugin.param("site_url")

PLUGIN = {
    contract_version = 1,
    name = "En-Thunderscans",
    site_url = BASE,
    logo = "logo.png",
    verify_url = BASE,
    needs_human_verify = false,
    thumb_ratio = 0.703,
}

local trim = host.text.trim
local unescape = host.text.unescape

-- ─── ABI: search_manga ──────────────────────────────────────────────────────

function search_manga(arg)
    local args = host.json.decode(arg)
    local query = args.query or ""
    local page = args.page or 1
    local genres = args.genres or {}

    local url
    if #genres > 0 then
        url = BASE .. "/genres/" .. host.text.url_encode(genres[1]) .. "/"
        if page > 1 then
            url = url .. "page/" .. tostring(page) .. "/"
        end
    else
        if page > 1 then
            url = BASE .. "/page/" .. tostring(page) .. "/?s=" .. host.text.url_encode(query)
        else
            url = BASE .. "/?s=" .. host.text.url_encode(query)
        end
    end
    local body = host.http.get_body(url)
    if not body or body == "" then
        log.error("en-thunderscans search empty response")
        return host.json.encode({})
    end

    local results = {}
    local seen = {}

    -- bsx cards: <div class="bsx"><a href=".../comics/<slug>/">...<img src>...<div class="tt">Title</div>
    for card in host.regex.gmatch(body, [[(?s)<div class="bsx".*?<div class="tt">.*?</div>]]) do
        local href = host.regex.find(card, [[href="([^"]+/comics/[^"/]+/)"]])
            or host.regex.find(card, [[href="([^"]+)"]])
        local title = host.regex.find(card, [[<div class="tt">\s*([^<]+)\s*</div>]])
            or host.regex.find(card, [[title="([^"]+)"]])
        if href then
            local slug = host.regex.find(href, [[/comics/([^/]+)/]])
                or host.regex.find(href, [[/([^/]+)/?$]])
            if slug and not seen[slug] then
                seen[slug] = true
                local cover = host.regex.find(card, [[data-src="[\t\n\s]*(https?://[^"]+)"]])
                    or host.regex.find(card, [[\ssrc="(https?://[^"]+)"]])
                if cover then cover = trim(cover) end
                results[#results + 1] = {
                    id = slug,
                    title = unescape(trim(title or "")),
                    cover_url = cover or ""
                }
            end
        end
    end

    log.debug("en-thunderscans search q=" .. query .. " page=" .. tostring(page) .. " found=" .. tostring(#results))
    return host.json.encode(results)
end

-- ─── ABI: get_manga_detail ──────────────────────────────────────────────────

local function fetch_detail_body(slug)
    local body = host.http.get_body(BASE .. "/comics/" .. slug .. "/")
    if body and body ~= "" and host.regex.find(body, [[entry-title]]) then
        return body
    end
    -- fallback: flat URL without /comics/ prefix
    local flat = host.http.get_body(BASE .. "/" .. slug .. "/")
    if flat and flat ~= "" then
        return flat
    end
    return body
end

function get_manga_detail(arg)
    local slug = host.json.decode(arg)
    local body = fetch_detail_body(slug)
    if not body then return host.json.encode({id = slug}) end

    local detail = { id = slug, title = "", author = "", description = "",
        cover_url = "", genres = {}, status = "" }

    detail.title = unescape(trim(host.regex.find(body, [[(?s)<h1 class="entry-title"[^>]*>(.*?)</h1>]]) or ""))

    local thumb = host.regex.find(body, [[(?s)<div class="thumb"[^>]*>(.*?)</div>]]) or ""
    detail.cover_url = trim(host.regex.find(thumb, [[data-src="(https?://[^"]+)"]])
        or host.regex.find(thumb, [[src="(https?://[^"]+)"]]) or "")

    detail.author = unescape(trim(host.regex.find(body, [[(?s)<div class="imptdt">\s*<h1>\s*Author\s*</h1>\s*<i>([^<]+)</i>]]) or ""))

    local gblock = host.regex.find(body, [[(?s)<span class="mgen">(.*?)</span>]]) or ""
    for g in host.regex.gmatch(gblock, [[<a[^>]*>([^<]+)</a>]]) do
        local name = unescape(trim(g))
        if name ~= "" then detail.genres[#detail.genres + 1] = name end
    end

    local status = host.regex.find(body, [[(?s)<div class="imptdt">\s*<h1>\s*Status\s*</h1>\s*<i>([^<]+)</i>]]) or ""
    detail.status = host.text.normalize_status(trim(status))

    local dblock = host.regex.find(body, [[(?s)entry-content entry-content-single"[^>]*>(.*?)</div>]]) or ""
    detail.description = host.regex.replace(host.text.strip_html(dblock), [[\s+]], " ")

    return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list ──────────────────────────────────────────────────
-- Free chapters: <li data-num="40"><a href=".../<manga>-chapter-40/">...chapternum...chapterdate...</a></li>
-- Coin-locked chapters have no <a href> (lockedChapterModal trigger) — skipped.

function get_chapter_list(arg)
    local slug = host.json.decode(arg)
    local body = fetch_detail_body(slug)
    if not body then return host.json.encode({}) end

    local chapters = {}
    local seen = {}

    for num, li in host.regex.gmatch(body, [[(?s)<li data-num="([^"]+)"[^>]*>(.*?)</li>]]) do
        local href = host.regex.find(li, [[href="([^"]+)"]])
        if href and not host.regex.match(li, "lockedChapterModal") then
            local label = host.regex.find(li, [[(?s)chapternum[^>]*>\s*([^<]+)]]) or ""
            label = trim(host.regex.replace(label, [[\s+]], " "))
            if label == "" then label = "Chapter " .. num end
            local date = trim(host.regex.find(li, [[(?s)chapterdate[^>]*>\s*([^<]+)]]) or "")
            local cid = host.regex.find(href, [[/([^/]+)/?$]])
            if cid and not seen[cid] then
                seen[cid] = true
                local iso = host.text.date_to_iso(date)
                local ch = {
                    id = cid,
                    chapter_num = host.text.chapter_num(label),
                    title = unescape(label),
                    url = href
                }
                if iso ~= "" then ch.released_at = iso end
                chapters[#chapters + 1] = ch
            end
        end
    end

    log.debug("en-thunderscans chapters slug=" .. slug .. " count=" .. tostring(#chapters))
    return host.json.encode(chapters)
end

-- ─── ABI: get_page_list ─────────────────────────────────────────────────────
-- Reader payload: ts_reader.run({"sources":[{"source":"Server 1","images":[...]}]})

function get_page_list(arg)
    local path = host.json.decode(arg)
    local body = host.http.get_body(BASE .. "/" .. path .. "/")
    if not body then return host.json.encode({}) end

    local pages = {}
    local run = host.regex.find(body, [[(?s)ts_reader\.run\((\{.*?\})\)]]) or ""
    local imgblock = host.regex.find(run, [==[(?s)"images"\s*:\s*\[(.*?)\]]==]) or ""
    if imgblock ~= "" then
        for src in host.regex.gmatch(imgblock, [[(https?:[^"]+?\.webp)]]) do
            src = host.regex.replace(src, [[\\]], "")
            src = trim(src)
            if src ~= "" then
                pages[#pages + 1] = { index = #pages, url = src }
            end
        end
    end

    log.debug("en-thunderscans pages " .. path .. " count=" .. tostring(#pages))
    return host.json.encode(pages)
end


-- ─── get_genres (optional export) ──────────────────────────────────────────
-- Slugs from /genres/<slug>/ (site navigation, plural).

local GENRES = {
    { name = "Action",        slug = "action" },
    { name = "Adult",         slug = "adult" },
    { name = "Adventure",     slug = "adventure" },
    { name = "Comedy",        slug = "comedy" },
    { name = "Cooking",       slug = "cooking" },
    { name = "Drama",         slug = "drama" },
    { name = "Ecchi",         slug = "ecchi" },
    { name = "Fantasy",       slug = "fantasy" },
    { name = "Gender Bender", slug = "gender-bender" },
    { name = "Harem",         slug = "harem" },
    { name = "Historical",    slug = "historical" },
    { name = "Horror",        slug = "horror" },
    { name = "Isekai",        slug = "isekai" },
    { name = "Josei",         slug = "josei" },
    { name = "Manhua",        slug = "manhua" },
    { name = "Manhwa",        slug = "manhwa" },
    { name = "Manga",         slug = "manga" },
    { name = "Martial Arts",  slug = "martial-arts" },
    { name = "Mature",        slug = "mature" },
    { name = "Mecha",         slug = "mecha" },
    { name = "Mystery",       slug = "mystery" },
    { name = "One Shot",      slug = "one-shot" },
    { name = "Psychological", slug = "psychological" },
    { name = "Romance",       slug = "romance" },
    { name = "School Life",   slug = "school-life" },
    { name = "Sci-Fi",        slug = "sci-fi" },
    { name = "Seinen",        slug = "seinen" },
    { name = "Shoujo",        slug = "shoujo" },
    { name = "Shoujo Ai",     slug = "shoujo-ai" },
    { name = "Shounen",       slug = "shounen" },
    { name = "Shounen Ai",    slug = "shounen-ai" },
    { name = "Slice of Life", slug = "slice-of-life" },
    { name = "Smut",          slug = "smut" },
    { name = "Sports",        slug = "sports" },
    { name = "Supernatural",  slug = "supernatural" },
    { name = "Thriller",      slug = "thriller" },
    { name = "Tragedy",       slug = "tragedy" },
    { name = "Webtoon",       slug = "webtoon" },
    { name = "Yaoi",          slug = "yaoi" },
    { name = "Yuri",          slug = "yuri" },
}

function get_genres()
    return host.json.encode(GENRES)
end
