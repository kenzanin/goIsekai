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
--   main.lua     THIS file — the only one you edit: PLUGIN table + BASE/CDN/UA
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
CDN = "https://cdn1.love4awalk.xyz"
UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

-- ─── site-specific helpers ────────────────────────────────────────────────

-- Full manga id is "{slug}.{zid}"; several endpoints want the bare slug.
function bare_slug(manga_id)
    return (manga_id:match("^(.-)%."))
end

-- Info rows on the detail page carry their value as a link target, not a
-- text node: <a href="/series?status=Ongoing"> and <a href="/author/NAME">.
-- (site-specific shape; keep the anchor scan in this plugin's main)
function label_value(html, label, href_prefix)
    -- no ">" anchor: the label is preceded by a newline+indent in real markup
    local pos = string.match(html, label .. "%s*</h1>()")
    if not pos then return "" end
    local seg = string.sub(html, pos, pos + 400)
    return string.match(seg, 'href="' .. host.text.lua_escape(href_prefix) .. '([^"]+)"') or ""
end

-- ─── ABI: search_manga(arg) ────────────────────────────────────────────────
-- arg: {"query":"...","page":1}  ->  array of {id, title, cover_url}
function search_manga(arg)
    local args = host.json.decode(arg)
    local query = args.query or ""
    log.debug("search q=" .. query)
    -- /home?keyword= renders the plain home feed (no server-side filtering);
    -- the search modal calls /api/search?search=Q instead.
    local raw = http_get(BASE .. "/api/search?search=" .. host.text.url_encode(query), true)
    if not raw then
        return host.json.encode({})
    end
    local ok, data = pcall(host.json.decode, raw)
    if not ok or not data or type(data.comics) ~= "table" then
        return host.json.encode({})
    end
    local out = {}
    for _, c in ipairs(data.comics) do
        if c.slug_hash and c.slug_hash ~= "" then
            out[#out + 1] = {
                id = c.slug_hash,
                title = c.title or "",
                cover_url = c.image or ""
            }
        end
    end
    log.debug("search: found " .. #out .. " results for q=" .. query)
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

    local title = string.match(html, '<meta property="og:title" content="([^"]*)"') or ""
    title = title:gsub("^Read%s+", "")
    title = title:gsub("%s*|%s*MangaBuddy%s*$", "")
    title = title:gsub("%s+Online$", "")
    title = title:gsub("%s+(Manga|Manhwa|Comic)$", "")
    title = host.text.html_decode(title)

    local cover = string.match(html, '<meta property="og:image" content="([^"]*)"') or ""

    local desc = string.match(html, '<meta name="description" content="([^"]*)"') or ""
    desc = desc:gsub("^Read%s+[^.]+%.%s*", ""):gsub("%s*Read the latest chapters online for free at MangaBuddy%.?%s*$", "")
    desc = host.text.html_decode(desc)

    local status = label_value(html, "Status", "/series?status=")
    local author = label_value(html, "Author", "/author/")
    if author == "Unknown" then author = "" end

    local genres = {}
    local seen_g = {}
    for g in string.gmatch(html, 'href="/genre/([a-z0-9%-]+)"') do
        if not seen_g[g] then
            seen_g[g] = true
            genres[#genres + 1] = g:gsub("%-", " "):gsub("(%a)([%w']*)", function(a, b)
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
    local manga_id, chslug = string.match(chapter_id, "^(.-):(.+)$")
    if not manga_id or not chslug then
        return host.json.encode({})
    end
    local html = http_get(BASE .. "/series/" .. manga_id .. "/" .. chslug)
    if not html then
        return host.json.encode({})
    end

    -- Page images: data-src="https://cdn1.love4awalk.xyz/{slug}/{ch}/{i}.webp"
    -- (chapter cover /thumb/ images and the discord gif don't match this shape).
    local pages = {}
    local seen = {}
    for u in string.gmatch(html, 'data%-src="(https://cdn1%.love4awalk%.xyz/[^"]-/%d+/%d+%.webp)"') do
        if not seen[u] then
            seen[u] = true
            pages[#pages + 1] = {url = u}
        end
    end
    log.debug("pages: " .. #pages .. " for chapter " .. manga_id .. ":" .. chslug)
    return host.json.encode(pages)
end
