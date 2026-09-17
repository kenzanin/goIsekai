-- MangaFire Lua plugin for goIsekai
-- Site: https://mangafire.to
-- VRF signing handled by host.crypto.vrf_sign (same base64 tables as JS plugin).
-- Image CDN (img-r1.2xstorage.com) requires Referer: https://mangafire.to/
--
-- Layout:
--   helpers.lua  generic helpers (normalizeStatus, http_get)
--   main.lua     THIS file — PLUGIN table + VRF tables + site parsers + ABI functions
-- Every sibling pre-executes before main.lua, so their globals are ready.

PLUGIN = {
    contract_version = 1,
    name = "MangaFire",
    site_url = "https://mangafire.to",
    logo = "logo.png",
    thumb_ratio = 0.677,
}

API_URL = "https://mangafire.to/api"
REFERER = "https://mangafire.to/"

-- VRF signing tables — same as the JS plugin.
-- host.crypto.vrf_sign(api_path, params, stages) handles the transform.
local VRF_K1 = "0Ec58JOY3uBzJK9m3zqIOpdlF7UFiax9DmA="
local VRF_T1 = "yINlmUNho8VYJT+ibTIP+9ESiULpVEtMOoD6U6lRE0R/xwXo/Xp9NrUgC4cw/Lmo33vUyjUE40kUoEWIr/fxfNNcq2s79ShQ5NhNrFnJ4hXPwOu/SuXzIbuTQKGFvfm08E9jvCfqAtoDqvQq3dVWPQFmJjgvkISBeXY3BgANR+yVnjGbcxZ47d6kLNfZPIayTq3/YGySb1KuVZodWp/WGNAO5pfMcpaK53Hhs0allBszaMaxuouOwdxbwgxIw6YunSsXjI05Yi0j9j4eHKfSXR8Ifo/Od+8iamRfCXTyvm7NGRGYdcQ0ywcK/u6RXhrbcCm4t2eCtrDgQVecJGkQ+A=="
local VRF_K2 = "AAdjb1iPY8CiDmq9H34tKTBF8a3oDQ=="
local VRF_T2 = "IUFltCxD3Oc2cwCgkJffthaOg9cgPUb0LgW6H/VtfcF0kc5F25t+aWj6JH9VOhOaY0rAFdUxlDnl5BLNvwEJvQtP5qcw7vdb/K+chnbwnspSHT8mz5lqwz41TezG0hkO06FTjJZhsyNuFLDpD2ZZxQj/QIRcF90zpmQ7Byu483WsQqUE0C342HL+JXngRB6fRzxRyVTaKu83h7UYTJ0QMt6ixFh6S3F8gqkKwrGTL3jHNBsD45UnifK8+RGtishQV2K3rujLKEkiZxpr2dYcudFW4oFsDKhad3CLBvuyTqsCo4B7mL5IKQ1vXo/MOOvq1I1d8ar9X6Ttu5KF4fZgiA=="
local VRF_K3 = "DELOJgPsVaCcblDtTGMdHzM="
local VRF_T3 = "NQHlu1/wVO5EmkwQymF810qqY2xG1k2obcas4Z9mCsPEIFl9pRIjFxbJ7ybMHbBckT5Ton85E0FOeHezbh/mjlEYpmpnlXOS8dgrqeq2KfxImTh1YK9y0PeMNhzA1OQzSY9brYOJq/l2QnE/hwOeZIhPixVSKIUlDb5vLcH6RWKxkIEMuP0bDwIqQ71AJJaEaMJL7A6YtyIwoRT+L5v4aZzodN/0+3nOGsfblFjgxSfPzVDjNFeNl5P26+kEC/8AHgdrpAbt3hHz3HrRN1Y6e+JHgF7ncFWnoF0y3THL1S71WgWGCa6KtSzTCCG58n68nTyj2T3Sshk7utqCtMi/ZQ=="

local VRF_STAGES = {
    {iv = 0x5A, key = VRF_K1, tbl = VRF_T1},
    {iv = 0x35, key = VRF_K2, tbl = VRF_T2},
    {iv = 0xBA, key = VRF_K3, tbl = VRF_T3},
}

-- Build a VRF-signed API URL. Parameters are sorted by key (Go net/url Encode).
local function vrf_url(path, params)
    if not params or next(params) == nil then
        params = {}
    end
    local sig = host.crypto.vrf_sign(path, params, VRF_STAGES)
    local parts = {}
    for k in pairs(params) do parts[#parts + 1] = k end
    table.sort(parts)
    local query = ""
    for i = 1, #parts do
        local k = parts[i]
        query = query .. host.text.url_encode(k) .. "=" .. host.text.url_encode(params[k])
        if i < #parts then query = query .. "&" end
    end
    local u = API_URL .. path
    if query ~= "" then
        return u .. "?" .. query .. "&vrf=" .. sig
    end
    return u .. "?vrf=" .. sig
end

-- ---------------------------------------------------------------------------
-- Text helpers (mirror JS plugin)
-- ---------------------------------------------------------------------------

function sanitize_title(s)
    return host.text.trim(host.text.unescape(s or ""))
end

-- ---------------------------------------------------------------------------
-- ABI functions
-- ---------------------------------------------------------------------------

-- Search — returns ALL results across upstream pages (host paginates).
function search_manga(arg)
    local f = host.json.decode(arg)
    local query = f and f.query or ""
    log.debug("mangafire search: q=" .. query)

    local all = {}
    local page = 1
    while true do
        local raw = http_get(vrf_url("/titles", {keyword = query, limit = "50", page = "" .. page}))
        if not raw then
            log.error("mangafire search: request failed page=" .. page)
            break
        end
        local body = host.json.decode(raw)
        if not body then break end
        local items = body.items or {}
        if #items == 0 then break end
        for _, it in ipairs(items) do
            all[#all + 1] = {
                id = it.hid,
                title = sanitize_title(it.title),
                cover_url = it.poster and it.poster.medium or "",
            }
        end
        local meta = body.meta or {}
        if not meta.has_next or page >= meta.last_page then break end
        page = page + 1
        if #all >= 2000 then break end
    end

    log.debug("mangafire search: found " .. #all .. " results for q=" .. query)
    return host.json.encode(all)
end

-- get_manga_detail — single title by hid.
function get_manga_detail(arg)
    local hid = host.json.decode(arg)
    if not hid then return host.json.encode(nil) end

    log.debug("mangafire detail: id=" .. hid)

    local raw = http_get(vrf_url("/titles/" .. hid, nil))
    if not raw then return host.json.encode(nil) end
    local body = host.json.decode(raw)
    if not body then return host.json.encode(nil) end
    local d = body.data
    if not d then return host.json.encode(nil) end

    local genres = {}
    if d.genres then
        for _, g in ipairs(d.genres) do genres[#genres + 1] = g.title end
    end
    local authors = {}
    if d.authors then
        for _, a in ipairs(d.authors) do authors[#authors + 1] = a.title end
    end

    return host.json.encode({
        id = d.hid or hid,
        title = sanitize_title(d.title),
        description = host.text.strip_html(d.synopsisHtml or ""),
        cover_url = d.poster and d.poster.medium or "",
        status = normalizeStatus(d.status),
        genres = genres,
        author = table.concat(authors, ", "),
    })
end

-- get_chapter_list — up to 3 pages (200/page), newest-first.
function get_chapter_list(arg)
    local hid = host.json.decode(arg)
    if not hid then return host.json.encode({}) end

    log.debug("mangafire chapters: id=" .. hid)

    local chapters = {}
    local page = 1
    while true do
        local raw = http_get(vrf_url("/titles/" .. hid .. "/chapters", {
            language = "en", limit = "200", order = "desc",
            page = "" .. page, sort = "number",
        }))
        if not raw then break end
        local body = host.json.decode(raw)
        if not body then break end
        local items = body.items or {}
        if #items == 0 then break end
        for _, c in ipairs(items) do
            local entry = {
                id = tostring(c.id),
                manga_id = hid,
                chapter_num = c.number,
                title = c.name or "",
                url = "",
            }
            -- created_at is unix seconds. A missing one becomes no released_at
            -- key, rather than the 1970 timestamp os.date used to emit.
            local iso = host.text.date_to_iso(tostring(c.created_at or 0))
            if iso ~= "" then entry.released_at = iso end
            chapters[#chapters + 1] = entry
        end
        local meta = body.meta or {}
        if page >= meta.last_page or not meta.has_next or page >= 3 then break end
        page = page + 1
    end

    table.sort(chapters, function(a, b) return a.chapter_num > b.chapter_num end)
    log.debug("mangafire chapters: found " .. #chapters .. " chapters for " .. hid)
    return host.json.encode(chapters)
end

-- get_page_list — pages for a chapter, each with Referer header.
function get_page_list(arg)
    local chapter_id = host.json.decode(arg)
    if not chapter_id then return host.json.encode({}) end

    log.debug("mangafire pages: chapter=" .. chapter_id)

    local raw = http_get(vrf_url("/chapters/" .. chapter_id, nil))
    if not raw then return host.json.encode({}) end
    local body = host.json.decode(raw)
    if not body then return host.json.encode({}) end
    local raw_pages = (body.data and body.data.pages) or {}
    if #raw_pages == 0 then return host.json.encode({}) end

    local pages = {}
    for i = 1, #raw_pages do
        pages[i] = {
            index = i - 1,
            url = raw_pages[i].url,
            headers = {Referer = REFERER},
        }
    end

    log.debug("mangafire pages: found " .. #pages .. " pages for " .. chapter_id)
    return host.json.encode(pages)
end
