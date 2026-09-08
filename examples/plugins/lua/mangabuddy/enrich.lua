-- enrich.lua — generic alt-title/alt-summary enrichment providers.
-- Copy this file into any plugin folder unchanged and declare the servers you
-- want in your PLUGIN table, e.g.:
--   alt_title_servers = {
--       {id = "mangadex",     name = "MangaDex",     kind = "titles"},
--       {id = "mangaupdates", name = "MangaUpdates", kind = "both"}
--   },
-- kind: "titles" (default) = appears in the alt-title picker only;
--       "summaries"         = alt-summary picker only;
--       "both"              = shows up in both pickers.
-- Requires the sandbox globals http_request / json / log and url_encode from
-- helpers.lua (already present in this folder's split). Sibling modules
-- pre-execute before main.lua, so the ABI exports below are visible at call
-- time regardless of file order.
--
-- ABI exports provided:
--   getAltTitles(arg)   arg {"title","server"} -> {source, titles[]}
--   getAltSummary(arg)  arg {"title","server"} -> {source, summaries[]}
--     server values handled: "mangadex", "mangaupdates"
--     unknown server -> empty result for the requested kind

-- ─── MangaUpdates helpers (local to this module) ───────────────────────────

-- search the MangaUpdates v1 API by title; returns best-match record or nil.
local function mangaupdates_search(title)
    local body = json.encode({search = title, stype = "title", perpage = 5})
    local resp = http_request({
        url = "https://api.mangaupdates.com/v1/series/search",
        method = "POST",
        headers = {
            ["Content-Type"] = "application/json",
            ["Accept"] = "application/json"
        },
        body = body
    })
    if not resp or resp.status ~= 200 then
        log.error("mangaupdates search http " .. tostring(resp and resp.status or "nil"))
        return nil
    end
    local ok, data = pcall(json.decode, resp.body)
    if not ok or not data or not data.results or #data.results == 0 then
        return nil
    end
    -- best match = first result (API sorts by relevance)
    return data.results[1].record
end

local function mangaupdates_detail(series_id)
    local resp = http_request({
        url = "https://api.mangaupdates.com/v1/series/" .. tostring(series_id),
        method = "GET",
        headers = {["Accept"] = "application/json"}
    })
    if not resp or resp.status ~= 200 then
        log.error("mangaupdates detail http " .. tostring(resp and resp.status or "nil"))
        return nil
    end
    local ok, data = pcall(json.decode, resp.body)
    if not ok or not data then return nil end
    return data
end

-- ─── MangaDex alt titles ───────────────────────────────────────────────────

local function altTitles_mangadex(title)
    local url = "https://api.mangadex.org/manga?title=" .. url_encode(title) ..
        "&limit=5&includes[]=manga"
    local resp = http_request({url = url, method = "GET", headers = {}})
    if not resp or resp.status ~= 200 then
        return json.encode({source = "MangaDex", titles = {}})
    end
    local ok, body = pcall(json.decode, resp.body)
    if not ok or not body or not body.data or #body.data == 0 then
        return json.encode({source = "MangaDex", titles = {}})
    end
    local attrs = body.data[1].attributes
    local out = {}
    local seen = {}
    local keep_langs = {en = true, ja = true, ["ja-ro"] = true, ko = true, ["ko-ro"] = true}
    if attrs.altTitles then
        for _, alt in ipairs(attrs.altTitles) do
            for lang, val in pairs(alt) do
                if keep_langs[lang] and val and not seen[val] then
                    seen[val] = true
                    out[#out + 1] = val
                end
            end
        end
    end
    return json.encode({source = "MangaDex", titles = out})
end

-- ─── MangaUpdates alt titles ───────────────────────────────────────────────

local function altTitles_mangaupdates(title)
    local record = mangaupdates_search(title)
    if not record then
        return json.encode({source = "MangaUpdates", titles = {}})
    end
    local detail = mangaupdates_detail(record.series_id)
    if not detail or not detail.associated then
        return json.encode({source = "MangaUpdates", titles = {}})
    end
    local out = {}
    local seen = {}
    for _, item in ipairs(detail.associated) do
        if item.title and item.title ~= "" and not seen[item.title] then
            seen[item.title] = true
            out[#out + 1] = item.title
        end
    end
    return json.encode({source = "MangaUpdates", titles = out})
end

-- ─── MangaUpdates alt summary ──────────────────────────────────────────────

local function altSummary_mangaupdates(title)
    local record = mangaupdates_search(title)
    if not record then
        return json.encode({source = "MangaUpdates", summaries = {}})
    end
    -- The search response already includes the description in the record.
    local desc = record.description or ""
    if desc == "" then
        -- Fallback: fetch full detail.
        local detail = mangaupdates_detail(record.series_id)
        if detail then desc = detail.description or "" end
    end
    if desc == "" then
        return json.encode({source = "MangaUpdates", summaries = {}})
    end
    return json.encode({source = "MangaUpdates", summaries = {desc}})
end

-- ─── ABI dispatch ──────────────────────────────────────────────────────────

function getAltTitles(arg)
    local input = json.decode(arg)
    local title = input.title or ""
    local server = input.server or "mangadex"

    if server == "mangaupdates" then
        return altTitles_mangaupdates(title)
    end
    -- default: MangaDex
    return altTitles_mangadex(title)
end

function getAltSummary(arg)
    local input = json.decode(arg)
    local title = input.title or ""
    local server = input.server or "mangaupdates"

    if server == "mangaupdates" then
        return altSummary_mangaupdates(title)
    end
    -- No other summary provider known; return empty.
    return json.encode({source = server, summaries = {}})
end
