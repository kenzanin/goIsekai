-- MangaBuddy plugin for goIsekai
-- Site: https://mangabuddy1.co.uk (mangabuddy.com redirects here; .com is a
-- status/mirror tracker page, the reader lives on the .co.uk mirror)
-- ABI contract version: 1 (matches pkg/types ContractVersion)
--
-- Site profile: custom Tailwind/Laravel reader (not Madara). No Cloudflare JS
-- challenge — plain GETs with a browser UA return 200. Data sources:
--   search:  GET /api/search?search=Q   -> JSON {comics:[{slug_hash,title,image,status}]}
--                                       (found in search-modal.js; /home?keyword= does NOT
--                                       filter server-side — it renders the home feed)
--   detail:  GET /series/{slug}.{zid} -> HTML; og:title/og:image, meta description,
--                                       Status/Author label rows, /genre/{g} chips
--   chapter: GET /get-chapter-list?slug={bare-slug}  -> clean JSON, all chapters,
--                                       newest-first, has chapter_num + updated_at
--   pages:   GET /series/{slug}.{zid}/{chapter-slug} -> HTML; data-src CDN webp imgs
--
-- Layout (split to make copying to a new plugin trivial):
--   helpers.lua  generic helpers  (normalizeStatus, http_get) — copy unchanged
--   main.lua     THIS file — the only one you edit: PLUGIN table + BASE/UA
--                + site-specific parsers + the four core ABI functions.
-- Every sibling pre-executes before main.lua, so their globals are ready.

PLUGIN = {
    contract_version = 1,
    name = "MangaBuddy",
    site_url = "https://mangabuddy1.co.uk",
    logo = "logo.png",
    verify_url = "https://mangabuddy1.co.uk",
    needs_human_verify = false,
    thumb_ratio = 0.667,
}

BASE = "https://mangabuddy1.co.uk"
UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

-- ─── helpers ────────────────────────────────────────────────────────────────

local trim = host.text.trim
local unescape = host.text.unescape

-- ─── site-specific helpers ────────────────────────────────────────────────

-- Full manga id is "{slug}.{zid}"; several endpoints want the bare slug.
function bare_slug(manga_id)
    return (host.regex.find(manga_id, [[^(.*?)\.]]))
end

-- Info rows on the detail page carry their value as a link target, not a
-- text node: <a href="/series?status=Ongoing"> and <a href="/author/NAME">.
-- (site-specific shape; keep the anchor scan in this plugin's main)
function label_value(html, label, href_prefix)
    -- no ">" anchor: the label is preceded by a newline+indent in real markup
    local _, finish = host.regex.find_index(html, label .. [[\s*</h1>]])
    if not finish then return "" end
    local seg = string.sub(html, finish + 1, finish + 401)
    return host.regex.find(seg, 'href="' .. host.regex.quote(href_prefix) .. '([^"]+)"') or ""
end

-- ─── ABI: search_manga(arg) ────────────────────────────────────────────────
-- arg: {"query":"...","page":1}  ->  array of {id, title, cover_url}
function search_manga(arg)
    local args = host.json.decode(arg)
    local query = args.query or ""
    local genres = args.genres or {}
    log.debug("search q=" .. query .. " page=" .. tostring(args.page or 1) .. " genres=" .. tostring(#genres))

    local out = {}
    if #genres > 0 then
        -- Genre browse: /series?genres=<slug> renders HTML comic cards.
        -- First genre wins (^A combine pattern). The page embeds both the
        -- series id (href="/series/{id}") and a cover (data-src="...").
        local url = BASE .. "/series?genres=" .. host.text.url_encode(genres[1])
        local page = args.page or 1
        if page > 1 then url = url .. "&page=" .. tostring(page) end
        local body = http_get(url, false)
        if not body then
            return host.json.encode({})
        end
        -- Card: <a href="/series/{id}"> ... <img alt="Cover of {title}" ... data-src="{cover}">
        -- (?s) so .* crosses the multi-line gap between <a> and <img>; alt appears
        -- before data-src in the emitted tag, so capture order is href, title, cover.
        for href, title, cover in host.regex.gmatch(body,
            [[(?s)<a[^>]*href="/series/([^/?"]+)"[^>]*>.*?<img[^>]*alt="Cover of ([^"]+)"[^>]*data-src="([^"]+)"]]) do
            if href ~= "" then
                out[#out + 1] = {
                    id = href,
                    title = unescape(title),
                    cover_url = trim(cover)
                }
            end
        end
    else
        -- Text search: /api/search?search=Q returns JSON {comics:[{slug_hash,title,image,status}]}.
        local raw = http_get(BASE .. "/api/search?search=" .. host.text.url_encode(query), true)
        if not raw then
            return host.json.encode({})
        end
        local ok, data = pcall(host.json.decode, raw)
        if not ok or not data or type(data.comics) ~= "table" then
            return host.json.encode({})
        end
        for _, c in ipairs(data.comics) do
            if c.slug_hash and c.slug_hash ~= "" then
                out[#out + 1] = {
                    id = c.slug_hash,
                    title = c.title or "",
                    cover_url = c.image or ""
                }
            end
        end
    end
    log.debug("search: found " .. #out .. " results")
    return host.json.encode(out)
end

-- ─── ABI: get_manga_detail(arg) ────────────────────────────────────────────
-- arg: '"slug.ZID"'  ->  {id, title, author, description, cover_url, genres, status}
function get_manga_detail(arg)
    local manga_id = host.json.decode(arg)
    local html = http_get(BASE .. "/series/" .. manga_id)
    if not html then
        return host.json.encode({id = manga_id}) -- detail must stay an OBJECT
    end

    local title = host.regex.find(html, [[<meta property="og:title" content="([^"]*)"]]) or ""
    title = host.regex.replace(title, [[^Read\s+]], "")
    title = host.regex.replace(title, [[\s*\|\s*MangaBuddy\s*$]], "")
    title = host.regex.replace(title, [[\s+Online$]], "")
    title = host.text.html_decode(title)

    local cover = host.regex.find(html, [[<meta property="og:image" content="([^"]*)"]]) or ""

    local desc = host.regex.find(html, [[<meta name="description" content="([^"]*)"]]) or ""
    desc = host.regex.replace(desc, [[^Read\s+[^.]+\.\s*]], "")
    desc = host.regex.replace(desc, [[\s*Read the latest chapters online for free at MangaBuddy\.?\s*$]], "")
    desc = host.text.html_decode(desc)

    local status = label_value(html, "Status", "/series?status=")
    local author = label_value(html, "Author", "/author/")
    if author == "Unknown" then author = "" end

    local genres = {}
    local seen_g = {}
    for g in host.regex.gmatch(html, [[href="/genre/([a-z0-9-]+)"]]) do
        if not seen_g[g] then
            seen_g[g] = true
            local label = host.regex.replace(g, "-", " ")
            genres[#genres + 1] = host.regex.replace(label, "([A-Za-z])([\\w']*)", function(a, b)
                return a:upper() .. b
            end)
        end
    end

    local detail = {
        id = manga_id,
        title = title,
        author = author,
        description = desc,
        cover_url = cover,
        genres = genres,
        status = normalizeStatus(status)
    }
    log.debug("detail: " .. detail.title .. " | " .. detail.status)
    return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list(arg) ────────────────────────────────────────────
-- arg: '"slug.ZID"'  ->  array of {id, manga_id, chapter_num, title, url, released_at}
-- Chapters are newest-first (descending number) per ABI convention.
function get_chapter_list(arg)
    local manga_id = host.json.decode(arg)
    local slug = bare_slug(manga_id)
    if not slug then
        return host.json.encode({})
    end
    -- JSON endpoint wants the bare slug (no .zid suffix)
    local raw = http_get(BASE .. "/get-chapter-list?slug=" .. slug)
    if not raw then
        return host.json.encode({})
    end
    local ok, body = pcall(host.json.decode, raw)
    if not ok or not body or not body.success or type(body.data) ~= "table" then
        log.error("get-chapter-list bad payload for " .. slug)
        return host.json.encode({})
    end

    local chapters = {}
    for _, ch in ipairs(body.data) do
        -- chapter_num is missing on some rows; the slug still ends in the
        -- number, which host.text parses. A row with neither carries no usable
        -- id or URL, so it is dropped.
        local slug = ch.chapter_slug or ""
        local num = ch.chapter_num or host.text.chapter_num(slug)
        if num > 0 or slug ~= "" then
            local entry = {
                id = manga_id .. ":" .. (ch.chapter_slug or ("chapter-" .. num)),
                manga_id = manga_id,
                chapter_num = num,
                title = ch.chapter_name or ("Chapter " .. tostring(num)),
                url = BASE .. "/series/" .. manga_id .. "/" .. (ch.chapter_slug or ("chapter-" .. num))
            }
            -- updated_at is an ISO timestamp or a relative phrase; the host
            -- knows both. A row with no date gets no released_at key at all,
            -- because an empty one fails the ABI decode of the whole list.
            local iso = host.text.date_to_iso(ch.updated_at or "")
            if iso ~= "" then entry.released_at = iso end
            chapters[#chapters + 1] = entry
        end
    end
    table.sort(chapters, function(a, b) return a.chapter_num > b.chapter_num end)
    log.debug("chapters: " .. #chapters .. " for " .. manga_id)
    return host.json.encode(chapters)
end

-- ─── ABI: get_page_list(arg) ───────────────────────────────────────────────
-- arg: '"slug.ZID:N"' (chapter id from get_chapter_list)  ->  array of {url}
function get_page_list(arg)
    local chapter_id = host.json.decode(arg) -- "slug.ZID:chapter_slug"
    local manga_id, chslug = host.regex.find(chapter_id, [[(?s)^(.*?):(.+)$]])
    if not manga_id or not chslug then
        return host.json.encode({})
    end
    local html = http_get(BASE .. "/series/" .. manga_id .. "/" .. chslug)
    if not html then
        return host.json.encode({})
    end

    -- Page images: data-src="https://{cdn}/{slug}/{ch}/{i}.webp"
    -- The host is matched loosely on purpose: the site moved its CDN from
    -- cdn1.love4awalk.xyz to cdn4.sags2mobile.com, and pinning the host here
    -- made every chapter come back with zero pages. The page shape (slug /
    -- chapter number / page index) is what identifies a page image - the cover
    -- (/thumb/...) and the ad script URLs do not have that shape.
    local pages = {}
    local seen = {}
    for u in host.regex.gmatch(html, [[data-src="(https://[^"/]+/[^"]*?/[\d.]+/\d+\.webp)"]]) do
        if not seen[u] then
            seen[u] = true
            pages[#pages + 1] = {index = #pages, url = u}
        end
    end
    log.debug("pages: " .. #pages .. " for chapter " .. manga_id .. ":" .. chslug)
    return host.json.encode(pages)
end

-- Genre names + slugs for /series?genres=<slug>. Harvested from the
-- search-page genre[] checkbox markup (visible label in <span>, data-slug = param).
local GENRES = {
    { name = "Action",         slug = "action" },
    { name = "Adult",          slug = "adult" },
    { name = "Adventure",      slug = "adventure" },
    { name = "Boys Love",      slug = "boys-love" },
    { name = "Comedy",         slug = "comedy" },
    { name = "Demons",         slug = "demons" },
    { name = "Drama",          slug = "drama" },
    { name = "Ecchi",          slug = "ecchi" },
    { name = "Fantasy",        slug = "fantasy" },
    { name = "Full Color",     slug = "full-color" },
    { name = "Girls Love",     slug = "girls-love" },
    { name = "Harem",          slug = "harem" },
    { name = "Historical",     slug = "historical" },
    { name = "Horror",         slug = "horror" },
    { name = "Isekai",         slug = "isekai" },
    { name = "Josei",          slug = "josei" },
    { name = "Magic",          slug = "magic" },
    { name = "Manga",          slug = "manga" },
    { name = "Manhwa",         slug = "manhwa" },
    { name = "Martial Arts",   slug = "martial-arts" },
    { name = "Mature",         slug = "mature" },
    { name = "Mystery",        slug = "mystery" },
    { name = "One shot",       slug = "one-shot" },
    { name = "Others",         slug = "others" },
    { name = "Psychological",  slug = "psychological" },
    { name = "Romance",        slug = "romance" },
    { name = "School",         slug = "school" },
    { name = "School life",    slug = "school-life" },
    { name = "Sci-Fi",         slug = "sci-fi" },
    { name = "Seinen",         slug = "seinen" },
    { name = "Shoujo",         slug = "shoujo" },
    { name = "Shounen",        slug = "shounen" },
    { name = "Slice of Life",  slug = "slice-of-life" },
    { name = "Smut",           slug = "smut" },
    { name = "Sports",         slug = "sports" },
    { name = "Supernatural",   slug = "supernatural" },
    { name = "Thriller",       slug = "thriller" },
    { name = "Tragedy",        slug = "tragedy" },
    { name = "Webtoons",       slug = "webtoons" },
    { name = "Yaoi",           slug = "yaoi" },
}

function get_genres()
    local genres = {}
    for _, g in ipairs(GENRES) do
        genres[#genres + 1] = { name = g.name, slug = g.slug }
    end
    return host.json.encode(genres)
end
