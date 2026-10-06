-- AnisaScan plugin for goIsekai (Lua / Lunar VM)
-- Site: https://anisascans.in (WordPress Madara theme 1.7.4)
-- ABI contract version: 1

PLUGIN = {
    contract_version = 1,
    name = "AnisaScan",
    site_url = "https://anisascans.in",
    logo = "logo.png",
    verify_url = "https://anisascans.in",
    needs_human_verify = false,
    thumb_ratio = 0.703,
}

BASE = "https://anisascans.in"

-- ─── helpers ────────────────────────────────────────────────────────────────

local trim = host.text.trim
local unescape = host.text.unescape

-- ─── ABI: search_manga ──────────────────────────────────────────────────────

function search_manga(arg)
    local args = host.json.decode(arg)
    local query = args.query or ""
    local page = args.page or 1
    local genres = args.genres or {}

    local body
    -- Genre browsing: GET /genres/<slug>/ (page N: /genres/<slug>/?page=N is
    -- not supported by Madara; /genres/<slug>/page/N/ is).
    if #genres > 0 then
        -- This theme's genre archives live under /manga-genre/<slug>/, not the
        -- /genre/<slug>/ that /genre/ redirects from.
        local url = BASE .. "/manga-genre/" .. host.text.url_encode(genres[1]) .. "/"
        if page > 1 then
            url = url .. "page/" .. tostring(page) .. "/"
        end
        body = host.http.get_body(url)
    else
        -- Madara GET search: admin-ajax.php returns 0 bytes, use server-rendered GET instead
        body = host.http.get_body(BASE .. "/?s=" .. host.text.url_encode(query) .. "&post_type=wp-manga")
    end
    if not body or body == "" then
        log.error("anisascans search empty response")
        return host.json.encode({})
    end

    local results = {}
    local seen = {}

    -- Madara search results are laid out one card per
    -- `c-tabs-item__content` block: cover <img> first, then post-title link.
    -- Parse per-card instead of scanning a ±3000-char window around the slug:
    -- the first occurrence of a slug is the og:url in <head>, which makes the
    -- window miss the card entirely (item #1 lost its cover), and far-apart
    -- markup made the fallback regex grab a neighbouring card's cover
    -- (covers "shifting" by one row). Capture up to the post-title close —
    -- the cover always precedes it inside the same card.
    for card in host.regex.gmatch(body, [[(?s)<div[^>]*class="[^"]*c-tabs-item__content.*?post-title.*?</h3>]]) do
        local href = host.regex.find(card, [[href="([^"]*/manga/[^"/]+/)"]])
        local title = host.regex.find(card, [[<h3[^>]*><a[^>]*>([^<]+)</a>]])
        if href then
            local slug = host.regex.find(href, [[/manga/([^/]+)/]]) or host.regex.find(href, [[/manga/([^/]+)$]])
            if slug and not seen[slug] then
                seen[slug] = true
                -- Cover lives in the same card, before the title: prefer
                -- data-src (lazy) then src; skip srcset width descriptors.
                local cover = host.regex.find(card, [[data-src="[\t\n\s]*(https?://[^"]+)"]])
                    or host.regex.find(card, [[\ssrc="(https?://[^"]+)"]])
                if cover then cover = trim(cover) end
                results[#results + 1] = {
                    id = slug,
                    title = unescape(title or ""),
                    cover_url = cover or ""
                }
            end
        end
    end

    log.debug("anisascans search q=" .. query .. " page=" .. tostring(page) .. " found=" .. tostring(#results))
    return host.json.encode(results)
end

-- ─── ABI: get_manga_detail ──────────────────────────────────────────────────

function get_manga_detail(arg)
    local slug = host.json.decode(arg)
    local body = host.http.get_body(BASE .. "/manga/" .. slug .. "/")
    if not body then return host.json.encode({id = slug}) end

    local detail = { id = slug, title = "", author = "", description = "",
        cover_url = "", genres = {}, status = "" }

    -- cover
    local cover_block = host.regex.find(body, [[(?s)class="summary_image".*?</a>]]) or ""
    detail.cover_url = trim(host.regex.find(cover_block, [[data-src="([^"]+)"]]) or host.regex.find(cover_block, [[src="([^"]+)"]]) or "")

    -- title
    local tblock = host.regex.find(body, [[(?s)class="post-title[^"]*">(.*?)</div>]]) or ""
    detail.title = unescape(trim(host.regex.find(tblock, [[(?s)<h[1-6][^>]*>(.*?)</h[1-6]>]]) or ""))

    -- author
    local ablock = host.regex.find(body, [[(?s)class="author-content">(.*?)</div>]]) or ""
    detail.author = host.text.strip_html(ablock):gsub(",", ", ")

    -- genres
    local gblock = host.regex.find(body, [[(?s)class="genres-content">(.*?)</div>]]) or ""
    for g in host.regex.gmatch(gblock, [[<a[^>]*>([^<]+)</a>]]) do
        local name = unescape(trim(g))
        if name ~= "" then detail.genres[#detail.genres + 1] = name end
    end

    -- status
    local sblock = host.regex.find(body, [[(?s)class="post-status">(.*?)$]]) or ""
    if sblock ~= "" then
        local after = host.regex.find(sblock, [[(?s)Status\s*</h5>.*?class="summary-content">\s*([^<]+)]])
        if after then
            detail.status = host.text.normalize_status(trim(after))
        end
    end

    -- description
    local dblock = host.regex.find(body, [[(?s)class="summary__content[^"]*">(.*?)<span\s+class="[^"]*content-readmore"]]) or ""
    if dblock == "" then dblock = host.regex.find(body, [[(?s)class="summary__content[^"]*">(.*?)</div>]]) or "" end
    detail.description = host.regex.replace(host.text.strip_html(dblock), [[\s+]], " ")

    return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list ──────────────────────────────────────────────────
-- Madara chapters via POST {base}/manga/SLUG/ajax/chapters/

function get_chapter_list(arg)
    local slug = host.json.decode(arg)
    local body = host.http.post_body(BASE .. "/manga/" .. slug .. "/ajax/chapters/", "", {
        ["X-Requested-With"] = "xmlhttprequest",
        ["Content-Type"] = "application/x-www-form-urlencoded",
    })
    if not body then return host.json.encode({}) end

    local chapters = {}
    local seen = {}

    -- Madara chapter list: <li class="wp-manga-chapter"><a href="URL">LABEL</a>...<i>DATE</i></li>
    for li in host.regex.gmatch(body, [[(?s)<li[^>]*class="wp-manga-chapter[^"]*"[^>]*>(.*?)</li>]]) do
        local href = host.regex.find(li, [[href="([^"]+)"]])
        local label = trim(host.regex.find(li, [[>([^<]*[Cc]hapter[^<]*)<]]) or "")
        local date = trim(host.regex.find(li, [[(?s)chapter-release-date[^>]*>.*?<i>([^<]+)</i>]]) or "")
        if href and label ~= "" then
            local cid = host.regex.find(href, [[/manga/[^/]+/([^/]+)/?$]]) or host.regex.find(href, [[/([^/]+)/?$]])
            if cid and not seen[cid] then
                seen[cid] = true
                -- Released dates normalize through host.text; "" means the site
                -- gave nothing usable, which leaves released_at out entirely.
                local iso = host.text.date_to_iso(date)
                local ch = {
                    id = slug .. ":" .. cid,
                    chapter_num = host.text.chapter_num(label),
                    title = label,
                    url = href
                }
                if iso ~= "" then ch.released_at = iso end
                chapters[#chapters + 1] = ch
            end
        end
    end

    -- Fallback: extract chapter links from <a href> containing /chapter-
    if #chapters == 0 then
        for href, label in host.regex.gmatch(body, [[<a[^>]*href="([^"]*chapter-[^"]+)"[^>]*>\s*([^<]+)\s*</a>]]) do
            local cid = host.regex.find(href, [[/([^/]+)/?$]])
            if cid and not seen[cid] then
                seen[cid] = true
                chapters[#chapters + 1] = {
                    id = slug .. ":" .. cid,
                    chapter_num = host.text.chapter_num(label),
                    title = trim(label),
                    url = href
                }
            end
        end
    end

    log.debug("anisascans chapters slug=" .. slug .. " count=" .. tostring(#chapters))
    return host.json.encode(chapters)
end

-- ─── ABI: get_page_list ─────────────────────────────────────────────────────

function get_page_list(arg)
    local path = host.json.decode(arg)
    path = path:gsub(":", "/")
    local body = host.http.get_body(BASE .. "/manga/" .. path .. "/")
    if not body then return host.json.encode({}) end

    local pages = {}
    local pos = 1
    while true do
        local s = host.regex.find_index(body, "<img", pos)
        if not s then break end
        local e = host.regex.find_index(body, ">", s)
        if not e then break end
        local tag = body:sub(s, e)
        if host.regex.match(tag, "wp-manga-chapter-img") then
            -- data-src may have tabs/newlines between the attribute and URL value
            local src = host.regex.find(tag, [[data-src="[\t\n\s]*(https?://[^"]+)"]])
                or host.regex.find(tag, [[data-src="(https?://[^"]+)"]])
                or host.regex.find(tag, [[src="[\t\n\s]*(https?://[^"]+)"]])
                or host.regex.find(tag, [[src="(https?://[^"]+)"]])
            if src then pages[#pages + 1] = { index = #pages, url = trim(src) } end
        end
        pos = e + 1
    end

    log.debug("anisascans pages " .. path .. " count=" .. tostring(#pages))
    return host.json.encode(pages)
end


-- ─── get_genres (optional export) ──────────────────────────────────────────
-- Slugs from /genres/<slug> (site navigation).
-- Slugs read from the site's own /manga-genre/ navigation. Some of these
-- names are truncated on the site itself (it labels a genre "Moder" and another
-- "Crossdressin"); they are kept verbatim rather than second-guessed.
local GENRES = {
    { name = "Romance", slug = "romance" },
    { name = "Historical", slug = "historical" },
    { name = "Comedy", slug = "comedy" },
    { name = "Shoujo", slug = "shoujo" },
    { name = "School Life", slug = "school-life" },
    { name = "Action", slug = "action" },
    { name = "Adaptation", slug = "adaptation" },
    { name = "Adventure", slug = "adventure" },
    { name = "Anime", slug = "anime" },
    { name = "Drama", slug = "drama" },
    { name = "Cooking", slug = "cooking" },
    { name = "Crime", slug = "crime" },
    { name = "Crossdressin", slug = "crossdressin" },
    { name = "Delinquents", slug = "delinquents" },
    { name = "Demons", slug = "demons" },
    { name = "Detective", slug = "detective" },
    { name = "Ecchi", slug = "ecchi" },
    { name = "Fantasy", slug = "fantasy" },
    { name = "Game", slug = "game" },
    { name = "Ghosts", slug = "ghosts" },
    { name = "Harem", slug = "harem" },
    { name = "Horror", slug = "horror" },
    { name = "Isekai", slug = "isekai" },
    { name = "Josei", slug = "josei" },
    { name = "Magic", slug = "magic" },
    { name = "Magical", slug = "magical" },
    { name = "Manhua", slug = "manhua" },
    { name = "Manhwa", slug = "manhwa" },
    { name = "Martial Arts", slug = "martial-arts" },
    { name = "Mature", slug = "mature" },
    { name = "Mecha", slug = "mecha" },
    { name = "Medical", slug = "medical" },
    { name = "Military", slug = "military" },
    { name = "Moder", slug = "moder" },
    { name = "Monsters", slug = "monsters" },
    { name = "Music", slug = "music" },
    { name = "Mystery", slug = "mystery" },
    { name = "Office Workers", slug = "office-workers" },
    { name = "One shot", slug = "one-shot" },
    { name = "Philosophical", slug = "philosophical" },
    { name = "Police", slug = "police" },
    { name = "Reincarnation", slug = "reincarnation" },
    { name = "Reverse", slug = "reverse" },
    { name = "Reverse harem", slug = "reverse-harem" },
    { name = "Royal family", slug = "royal-family" },
    { name = "Sci-fi", slug = "sci-fi" },
    { name = "Seinen", slug = "seinen" },
    { name = "Shounen", slug = "shounen" },
    { name = "Shounen Ai", slug = "shounen-ai" },
    { name = "Slice of Life", slug = "slice-of-life" },
    { name = "Sports", slug = "sports" },
    { name = "Super power", slug = "super-power" },
    { name = "Superhero", slug = "superhero" },
    { name = "Supernatural", slug = "supernatural" },
    { name = "Survival", slug = "survival" },
    { name = "Thriller", slug = "thriller" },
    { name = "Time Travel", slug = "time-travel" },
    { name = "Tragedy", slug = "tragedy" },
    { name = "Vampire", slug = "vampire" },
    { name = "Villainess", slug = "villainess" },
    { name = "Webtoons", slug = "webtoons" },
    { name = "Yaoi", slug = "yaoi" },
    { name = "Yuri", slug = "yuri" },
    { name = "Zombies", slug = "zombies" },
}

function get_genres()
    return host.json.encode(GENRES)
end
