-- sanascans.lua — Sana Scans
--
-- SSR-first manga reader (TanStack Start). Chapters carry ISO dates on the
-- series page, search/list are server-rendered; no CDP required. No
-- authenticated API.
--
-- Probe evidence:
--   GET /comics?q=Dungeon            -> 650KB SSR; cards `<a href="/series/<slug>">`
--   GET /series/<slug>               -> 192KB SSR; chapter rows
--     <a href="/series/<slug>/chapter-N" class="flex ..."> Ch...N </a>
--     ... nearby <time datetime="YYYY-MM-DD"> or YYYY-MM-DD in same row
--   GET /series/<slug>/chapter-N/...  -> 150KB SSR; cover + content
--
-- Date strategy: series page embeds a `20YY-MM-DD` timestamp per chapter row.
-- host.text.date_to_iso accepts "2006-01-02" directly. Per your directive,
-- the plugin formats dates to RFC 3339 UTC before sending released_at to host.

local BASE = "https://sanascans.com"

local hostnet = host  -- kept for clarity; `host` is the global

-- tiny helpers (mirrors mangahere plugin style)
local function trim(s)
    return host.text.trim(s or "")
end

local function html_decode(s)
    return host.text.html_decode(s or "")
end

local function find_all(str, pat)
    return host.regex.find_all(str, pat)
end

-- `log` is a host-provided global (info/error/debug); no wrapper needed.

local function b64e(s)
    -- URL-safe base64, no padding — used by the (rejected) API path notes only.
    return host.encoding.base64_url_encode(s or "")
end

-- ─── Helpers ──────────────────────────────────────────────────────────────

-- Extract a slug from a series path like /series/<slug> or <slug>.
local function slug_of(path)
    if not path then return nil end
    local s = string.match(path, "/series/([^/]+)")
    if not s then s = path end
    s = string.match(s, "([^/?#]+)")
    return s or nil
end

-- Take the first ISO date found within a chunk of text.
local function first_iso(chunk)
    return (string.match(chunk or "", "%d%d%d%d%-%d%d%-%d%d")) or nil
end

-- ─── ABI: search ──────────────────────────────────────────────────────────
-- arg: '"query"'
-- /comics is fully SSR (650KB HTML); cards link to /series/<slug>.
function search(arg)
    local args = host.json.decode(arg)
    local query = (args and args.query) or ""
    if query == "" then
        return host.json.encode({})
    end
    local body = host.http.get_body(BASE .. "/comics?search=" .. host.text.url_encode(query))
    if not body or body == "" then
        log.error("search fetch failed for: " .. query)
        return nil, "upstream_unavailable"
    end
    -- Cards: <a href="/series/<slug>" ...><img ... alt="Cover of <title>" src="<cover>">
    -- (attribute order is alt BEFORE src; the site prefixes many card alts
    -- with "Cover of " — strip it so titles stay clean.)
    local out, seen = {}, {}
    for _, m in ipairs(find_all(body,
        [[(?s)<a href="/series/([^"]+)"[^>]*>.*?<img[^>]*alt="([^"]*)"[^>]*src="([^"]+)"]]) or {}) do
        local slug, title, cover = m[1], m[2], m[3]
        title = string.gsub(title, "^Cover [Oo]f%s+", "")
        if not seen[slug] then
            seen[slug] = true
            out[#out + 1] = {
                id = slug,
                title = html_decode(trim(title)),
                cover_url = cover,
            }
        end
    end
    log.info("search found " .. tostring(#out) .. " for " .. query)
    return host.json.encode(out)
end

-- ─── ABI: get_manga_detail ────────────────────────────────────────────────
-- arg: '"slug"'
function get_manga_detail(arg)
    local slug = host.json.decode(arg)
    if not slug or slug == "" then
        return host.json.encode({})
    end
    local url = BASE .. "/series/" .. host.text.url_encode(slug)
    local body = host.http.get_body(url)
    if not body or body == "" then
        log.error("detail fetch failed for: " .. slug)
        return nil, "upstream_unavailable"
    end
    -- Real cover: the itemProp="image" img src (full art). og:image on this
    -- site is a generated og-card with title overlay — cropped, so use it
    -- only as fallback.
    cover = html_decode((string.match(body, 'itemProp="image"[^>]-src="([^"]+)"'))
           or (string.match(body, 'property="og:image" content="([^"]+)"'))
           or (string.match(body, '"image":"([^"]+)"')) or "")
    local title = html_decode((string.match(body, 'property="og:title" content="([^"]+)"'))
            or (string.match(body, '"name":"([^"]*)"')) or slug)
    -- strip a trailing "Manga - Sana Scans" suffix if present
    title = string.match(title, "(.-)%s*[Mm]anga%s*-%s*[Ss]ana") or title
    local desc = html_decode((string.match(body, 'property="og:description" content="([^"]*)"'))
            or (string.match(body, '"description":"([^"]*)"')) or "")
    local status = html_decode(string.match(body, '"status":"([^"]*)"') or "")

    local genres = {}
    for g in string.gmatch(body, '"(genre|category)"%s*:%s*"([^"]*)"') do
        genres[#genres + 1] = g
    end

    local ch = {
        id = slug,
        title = title,
        cover_url = cover,
        genres = genres,
        status = status,
        description = desc,
        is_library = true,
    }
    return host.json.encode(ch)
end

-- ─── ABI: get_chapter_list ────────────────────────────────────────────────
-- arg: '"slug"'  (series slug, e.g. "the-villainess-became-a-single-mother")
function get_chapter_list(arg)
    local slug = host.json.decode(arg)
    if not slug or slug == "" then
        return host.json.encode({})
    end
    local url = BASE .. "/series/" .. host.text.url_encode(slug)
    local body = host.http.get_body(url)
    if not body or body == "" then
        log.error("chapter fetch failed for: " .. slug)
        return nil, "upstream_unavailable"
    end
    -- Chapter rows look like:
    --   <a href="/series/<slug>/chapter-18" class="flex ...">
    --     <span ...>Ch. 18</span>
    --     <span ...>2026-10-24</span>   (or <time datetime="YYYY-MM-DD">)
    -- We slice per-row by anchoring on the chapter link and reading the next
    -- ISO date / <time> in the same row.
    local out = {}
    local rows = find_all(body,
        [[(?s)<a href="(/series/[^"/]+/chapter-[\d.]+)"[^>]*>(.*?)</a>(.{0,400})]])
    if not rows or #rows == 0 then
        log.error("no chapter rows found for " .. slug)
        return nil, "no_chapters"
    end
    for _, m in ipairs(rows) do
        local path, cell, tail = m[1], m[2] or "", m[3] or ""
        local chid = path:match("chapter%-[%d.]+")
        if not chid then
            chid = path:match("/([^/]+)$") or path
        end
        -- chapter number: from the visible "Ch. N" text, else the URL tail
        local num = cell:match("Ch%.%s*([%d.]+)")
        if not num then
            num = chid:match("([%d.]+)$")
        end
        -- date: prefer <time datetime="..."> in the link or the row tail
        -- (the ISO date renders as a sibling cell after </a>), else first ISO
        local raw_date = cell:match('<time[^>]*datetime="([^"]+)"')
            or tail:match('<time[^>]*datetime="([^"]+)"')
        if not raw_date or raw_date == "" then
            raw_date = first_iso(tail) or first_iso(cell)
        end
        local ch = {
            id = slug .. ":" .. chid,
            chapter_num = tonumber(num) or 0,
            title = trim((cell:match(">%s*([^<]*)%s*<!--") or cell:match(">%s*([^<]*)"))),
        }
        if raw_date then
            local iso = host.text.date_to_iso(trim(raw_date))
            if iso ~= "" then ch.released_at = iso end
        end
        out[#out + 1] = ch
    end
    log.info("found " .. tostring(#out) .. " chapters for " .. slug)
    -- sanascans lists newest first; keep document order.
    return host.json.encode(out)
end

-- ─── ABI: get_page_list ────────────────────────────────────────────────────
-- arg: '"slug:chapter-id"'  (chapter-id as used in get_chapter_list, e.g.
--        "the-saint-of-warding...:chapter-3")
function get_page_list(arg)
    local composite = host.json.decode(arg)
    if type(composite) ~= "string" or composite == "" then
        return host.json.encode({})
    end
    local slug, chid = composite:match("^(.-):(.+)$")
    if not slug or not chid then
        return host.json.encode({})
    end
    -- sanascans reader pages are <img src> / data-src in a scroll container —
    -- SSR includes all page images; no CDP needed.
    local url = BASE .. "/series/" .. host.text.url_encode(slug) .. "/" .. chid
    local body = host.http.get_body(url)
    if not body or body == "" then
        log.error("page fetch failed for: " .. composite)
        return nil, "upstream_unavailable"
    end
    -- Reader pages are SSR <img data-reader-page-image="true"> tags inside
    -- <section itemProp="articleBody">, in order (data-reader-index 0..N):
    --   <img src="https://storage.sanascans.com/upload/series/<slug>/<ch>/01.webp"
    --        data-reader-page-image="true" data-reader-index="0" ...>
    log.info("page body len " .. tostring(#body) .. " marker=" .. tostring(string.find(body, "data%-reader%-page%-image", 1, false) ~= nil))
    -- coregex quietly returns 0 on some anchors here (marker=true yet 0 hits),
    -- so scan with plain Lua substring finds instead of the regex engine.
    -- NOTE: the site emits a double slash after the host
    -- (https://storage.sanascans.com//upload/series/...), so the anchor stops
    -- at the host and the URL is rebuilt without the duplicate slash.
    local result = {}
    local anchor = 'src="https://storage.sanascans.com/'
    local pos = string.find(body, anchor, 1, true)
    while pos do
        local ustart = pos + #anchor
        local close = string.find(body, '"', ustart, true)
        if not close then break end
        local path = string.sub(body, ustart, close - 1)
        if string.find(path, "upload/series/", 1, true) then
            local u = string.gsub("https://storage.sanascans.com/" .. path,
                "(https://[^/]+)/+", "%1/")
            result[#result + 1] = { index = #result, url = u }
        end
        pos = string.find(body, anchor, close, true)
    end
    if #result == 0 then
        log.error("no reader images found for " .. composite)
        return nil, "no_pages"
    end
    if #result == 0 then
        log.error("no images found for " .. composite)
        return nil, "no_pages"
    end
    log.info("found " .. tostring(#result) .. " pages for " .. composite)
    return host.json.encode(result)
end

return { b64e = b64e }
