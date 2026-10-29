-- comik.to plugin — translation hub (manga grouped by scanlation source)
-- Pure SSR (Qwik): search/title/source/chapter pages carry full data in HTML.
-- No CDP needed. Chapter released_at comes from <time data-time="epoch-ms">.

local BASE = "https://comik.to"

-- ─── helpers ──────────────────────────────────────────────────────────────

local function trim(s)
    return (string.gsub(s, "^%s+", ""):gsub("%s+$", ""))
end

local function html_decode(s)
    if not s then return "" end
    local ents = { ["&amp;"] = "&", ["&lt;"] = "<", ["&gt;"] = ">",
                   ["&quot;"] = '"', ["&#39;"] = "'", ["&apos;"] = "'",
                   ["&#x27;"] = "'", ["&nbsp;"] = " " }
    for ent, ch in pairs(ents) do
        s = string.gsub(s, ent, ch)
    end
    return (string.gsub(s, "&#(%d+);", function(n) return utf8 and utf8.char and utf8.char(tonumber(n)) or "" end))
end

-- epoch milliseconds (comik <time data-time="...">) -> "YYYY-MM-DD".
-- Pure-Lua civil-date conversion: os.date is unavailable in the plugin
-- runtime (returns nil, so chapters all got stamped with fetch time).
local function epoch_ms_to_iso(ms)
    local secs = tonumber(ms)
    if not secs then return nil end
    secs = math.floor(secs / 1000)
    local days = math.floor(secs / 86400)
    -- Howard Hinnant's civil_from_days algorithm
    local z = days + 719468
    local era = math.floor((z >= 0 and z or z - 146096) / 146097)
    local doe = z - era * 146097
    local yoe = math.floor((doe - math.floor(doe/1460) + math.floor(doe/36524) - math.floor(doe/146096)) / 365)
    local y = yoe + era * 400
    local doy = doe - (365*yoe + math.floor(yoe/4) - math.floor(yoe/100))
    local mp = math.floor((5*doy + 2)/153)
    local d = doy - math.floor((153*mp+2)/5) + 1
    local m = mp + (mp < 10 and 3 or -9)
    y = y + (m <= 2 and 1 or 0)
    -- Full RFC3339 midnight UTC: the host unmarshals released_at as
    -- time.Time, which rejects date-only strings and silently drops the
    -- whole chapter list.
    return string.format("%04d-%02d-%02dT00:00:00Z", y, m, d)
end

-- find_all over Go coregex; each match is a table of capture groups
local function find_all(body, pattern)
    local ok, res = pcall(function() return host.regex.find_all(body, pattern) end)
    if not ok then return nil end
    return res
end

local function get(url)
    local body = host.http.get_body(url)
    if not body or body == "" then
        return nil
    end
    return body
end

-- ─── ABI: search ──────────────────────────────────────────────────────────
-- arg: {"query":"..."}
-- Cards: <a href="/title/<id>" : class="block relative"><img alt="<title>"
--        src="<cover>"...> then title link repeats with the alias list.
-- Dedup by title id; first <img> in the card carries the cover.
function search(arg)
    local args = host.json.decode(arg)
    local query = (args and args.query) or ""
    if query == "" then
        return host.json.encode({})
    end
    local body = get(BASE .. "/search?word=" .. host.text.url_encode(query))
    if not body then
        log.error("search fetch failed for: " .. query)
        return nil, "upstream_unavailable"
    end
    -- cover img precedes the title link inside the same card
    local out, seen = {}, {}
    for _, m in ipairs(find_all(body,
        [[(?s)<a href="/title/([a-z0-9]+)" : class="block relative"><img[^>]*alt="([^"]*)"[^>]*src="([^"]+)"]]) or {}) do
        local id, title, cover = m[1], m[2], m[3]
        if not seen[id] then
            seen[id] = true
            out[#out + 1] = {
                id = id,
                title = html_decode(trim(title)),
                cover_url = cover:find("^http") and cover or (BASE .. cover),
            }
        end
    end
    log.info("search found " .. tostring(#out) .. " for " .. query)
    return host.json.encode(out)
end

-- ─── ABI: get_manga_detail ────────────────────────────────────────────────
-- arg: '"<title-id>"'
-- <title> holds the full title. Genre chips are plain text spans. Status is
-- rendered as Completed/Ongoing text. Cover is the first card-size image.
function get_manga_detail(arg)
    local id = host.json.decode(arg)
    if not id or id == "" then
        return host.json.encode({})
    end
    local body = get(BASE .. "/title/" .. id)
    if not body then
        log.error("detail fetch failed for: " .. id)
        return nil, "upstream_unavailable"
    end
    local title = html_decode(trim((string.match(body, "<title>([^<]*)</title>") or id)
        :gsub("%s*::%s*COMIK%.TO%s*$", "")))
    -- cover: first large _f webp referenced in the page header area
    local cover = string.match(body, '<img[^>]*src="(/_f/[^"]+%.webp)"') or ""
    -- genres: genre chip links on /latest/<slug> exist; on the title page the
    -- chips are plain text next to the status. Collect the known-genre words.
    local genres = {}
    local gseen = {}
    for g in string.gmatch(body, ">(%u%a+)<") do
        local gl = string.lower(g)
        if not gseen[gl] and (gl == "action" or gl == "adventure" or gl == "comedy"
            or gl == "drama" or gl == "fantasy" or gl == "romance" or gl == "isekai"
            or gl == "horror" or gl == "mystery" or gl == "sci" or gl == "slice"
            or gl == "sports" or gl == "supernatural" or gl == "thriller") then
            gseen[gl] = true
            genres[#genres + 1] = gl
        end
    end
    -- status: first occurrence of the rendered badge text
    local status = string.match(body, ">(Ongoing)<") and "ongoing"
        or (string.match(body, ">(Completed)<") and "completed" or "")
    local ch = {
        id = id,
        title = title,
        cover_url = cover:find("^http") and cover or (cover ~= "" and BASE .. cover or ""),
        genres = genres,
        status = status,
        description = "",
        is_library = true,
    }
    return host.json.encode(ch)
end

-- ─── ABI: get_chapter_list ────────────────────────────────────────────────
-- arg: '"<title-id>"'
-- comik.to is a translation hub: a title aggregates multiple scanlation
-- sources. The title page lists /source/<id> groups + a handful of latest
-- chapters; the FULL chapter lists live on each /source/<id> page. We merge
-- all source pages, dedup by chapter number text, and keep the earliest
-- released_at per chapter.
function get_chapter_list(arg)
    local id = host.json.decode(arg)
    if not id or id == "" then
        return host.json.encode({})
    end
    local body = get(BASE .. "/title/" .. id)
    if not body then
        log.error("chapter fetch failed for: " .. id)
        return nil, "upstream_unavailable"
    end

    -- collect source ids (dedup, keep order)
    local srcs, sseen = {}, {}
    for sid in string.gmatch(body, 'href="/source/([a-z0-9]+)"') do
        if not sseen[sid] then
            sseen[sid] = true
            srcs[#srcs + 1] = sid
        end
    end
    log.info("title " .. id .. " has " .. tostring(#srcs) .. " sources")

    -- Chapter row (source + title pages): <a href="/chapter/<id>" ...><label>
    -- </a> ...flex cells... <time data-time="<epoch-ms>">. Scanned with plain
    -- Lua string ops (coregex lazy quantifiers proved unreliable here — same
    -- lesson as sanascans get_page_list): anchor on the chapter link, then
    -- read the first data-time=" within the next 400 chars.
    local out, bynum = {}, {}
    local function harvest(pagebody)
        local pos = string.find(pagebody, 'href="/chapter/', 1, true)
        while pos do
            local ids = string.match(pagebody, '/chapter/([a-z0-9]+)', pos + 15)
            local close = string.find(pagebody, '</a>', pos, true)
            if not ids or not close then break end
            local window = string.sub(pagebody, pos, close + 400)
            local ms = string.match(window, 'data%-time="(%d+)"') or string.match(window, 'data-time="(%d+)"')
            local label = string.match(window, '>([^<]+)</a>')
            if ms then
                -- dedup by chapter id, not number: comik labels "Oneshot"
                -- and "Chapter 0" both coerce to num 0 but are real,
                -- distinct chapters
                local prev = bynum[ids]
                if prev then
                    -- keep the earliest release across groups
                    if iso and (not prev.released_at or iso < prev.released_at) then
                        prev.released_at = iso
                    end
                else
                    local num = label and label:match("(%d+%.?%d*)") or "0"
                    local iso = epoch_ms_to_iso(ms)
                    local item = {
                        id = ids,
                        title = html_decode(trim(label or "")),
                        chapter_num = tonumber(num) or 0,
                        released_at = iso,
                    }
                    bynum[ids] = item
                    out[#out + 1] = item
                end
            end
            pos = string.find(pagebody, 'href="/chapter/', close, true)
        end
    end

    -- latest chapters on the title page first (covers single-source titles)
    harvest(body)

    -- then every scanlation source page (full lists)
    for _, sid in ipairs(srcs) do
        local sb = get(BASE .. "/source/" .. sid)
        if sb then
            harvest(sb)
        end
    end

    if #out == 0 then
        log.error("no chapter rows found for " .. id)
        return nil, "no_chapters"
    end
    -- newest first (sources render newest-first; title page mixes)
    table.sort(out, function(a, b) return (a.chapter_num or 0) > (b.chapter_num or 0) end)
    log.info("found " .. tostring(#out) .. " chapters for " .. id)
    return host.json.encode(out)
end

-- ─── ABI: get_page_list ───────────────────────────────────────────────────
-- arg: '"<chapter-id>"'
-- Reader pages are SSR <img src="https://iXX.img00.org/...webp"> in order.
function get_page_list(arg)
    local chid = host.json.decode(arg)
    if not chid or chid == "" then
        return host.json.encode({})
    end
    local body = get(BASE .. "/chapter/" .. chid)
    if not body then
        log.error("page fetch failed for: " .. chid)
        return nil, "upstream_unavailable"
    end
    local out = {}
    -- Lua pattern scan (coregex find_all returned 0 here; same lesson as the
    -- chapter harvester). Reader pages: <img src="https://iXX.img00.org/...webp">
    for u in string.gmatch(body, 'https://i%d+%.img00%.org/[^"]+%.webp') do
        out[#out + 1] = { index = #out, url = u }
    end
    if #out == 0 then
        log.error("no reader images found for " .. chid)
        return nil, "no_pages"
    end
    log.info("found " .. tostring(#out) .. " pages for " .. chid)
    return host.json.encode(out)
end

-- ─── ABI: get_genres ──────────────────────────────────────────────────────
-- /latest/<slug> browse pages; genre list is the site's chip set.
function get_genres()
    local genres = {
        "action", "adaptation", "adventure", "comedy", "cooking", "crime",
        "drama", "fantasy", "historical", "horror", "isekai", "mystery",
        "romance", "school", "sci-fi", "slice-of-life", "sports",
        "supernatural", "thriller", "web-comic",
    }
    return host.json.encode(genres)
end

return {}
