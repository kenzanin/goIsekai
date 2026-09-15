-- MangaDex enrichment script for goIsekai.
--
-- Lives in the info directory (default app_data/info, one folder per source),
-- NOT in the plugin directory: this fetches manga metadata rather than
-- scraper content, so it never appears as a manga source and is never used to
-- read a chapter. Drop a sibling folder to add another info source.
--
-- The host calls getEnrichment once per kind and stores what comes back in the
-- matching enrichment section of the detail page, where the user picks what to
-- promote to main. Fields served here:
--   titles      alternative titles
--   summaries   synopsis
--   categories  genres
--   authors     author
--   related     related manga, each linked to its MangaDex page
--
-- One title maps to two upstream requests (search, then detail). The detail
-- response carries every field, so it is memoized in the VM and reused by the
-- remaining kinds of the same title; MangaDex rate-limits aggressively and
-- five kinds would otherwise mean ten requests.

PLUGIN = {
    contract_version = 1,
    name = "MangaDex Info",
    site_url = "https://mangadex.org",
    enrichment_providers = {
        {
            id = "mangadex",
            name = "MangaDex",
            kinds = { "titles", "summaries", "categories", "authors", "related" },
        },
    },
}

local API = "https://api.mangadex.org"
local SITE = "https://mangadex.org"

-- Languages preferred for a displayed string, most preferred first.
local PREFERRED = { "en", "ja-ro", "ko-ro", "ja", "ko" }

-- Single-slot memo: enrichment runs kind by kind for one manga at a time, so
-- the previous title's detail is never needed again.
local memoTitle, memoDetail

local function get(url)
    local resp = host.http.get(url, { ["Accept"] = "application/json" })
    if not resp or resp.status ~= 200 then
        return nil
    end
    local body, err = host.json.decode(resp.body)
    if err or not body then
        return nil
    end
    return body
end

-- pick returns the best available string from a {lang = text} map.
local function pick(map)
    if type(map) ~= "table" then
        return ""
    end
    for _, lang in ipairs(PREFERRED) do
        if map[lang] and map[lang] ~= "" then
            return map[lang]
        end
    end
    for _, text in pairs(map) do
        if text ~= "" then
            return text
        end
    end
    return ""
end

-- detail fetches the full record for the best search match, memoized.
local function detail(title)
    if memoTitle == title then
        return memoDetail
    end
    memoTitle, memoDetail = title, nil

    local search = get(API .. "/manga?limit=1&order[relevance]=desc&title=" .. host.text.url_encode(title))
    if not search or type(search.data) ~= "table" or #search.data == 0 then
        return nil
    end

    local id = search.data[1].id
    local full = get(API
        .. "/manga/"
        .. id
        .. "?includes[]=cover_art&includes[]=author&includes[]=artist&includes[]=manga")
    if not full or not full.data then
        return nil
    end

    memoDetail = full.data
    return memoDetail
end

local function items(list)
    return host.json.encode(list)
end

local function altTitles(data)
    local out, seen = {}, {}
    local function add(t)
        if t and t ~= "" and not seen[t] then
            seen[t] = true
            out[#out + 1] = { value = t, url = SITE .. "/title/" .. data.id }
        end
    end
    for _, entry in ipairs(data.attributes.altTitles or {}) do
        add(pick(entry))
    end
    add(pick(data.attributes.title))
    return items(out)
end

local function summaries(data)
    local desc = pick(data.attributes.description)
    if desc == "" then
        return items({})
    end
    return items({ { value = host.text.strip_markdown(desc), url = SITE .. "/title/" .. data.id } })
end

local function categories(data)
    local out, seen = {}, {}
    for _, tag in ipairs(data.attributes.tags or {}) do
        local attrs = tag.attributes or {}
        if attrs.group == "genre" then
            local name = pick(attrs.name)
            if name ~= "" and not seen[name] then
                seen[name] = true
                out[#out + 1] = { value = name, url = SITE .. "/title/" .. data.id }
            end
        end
    end
    return items(out)
end

local function authors(data)
    local out, seen = {}, {}
    for _, rel in ipairs(data.relationships or {}) do
        if rel.type == "author" then
            local name = rel.attributes and rel.attributes.name or ""
            if name ~= "" and not seen[name] then
                seen[name] = true
                out[#out + 1] = { value = name, url = SITE .. "/title/" .. data.id }
            end
        end
    end
    return items(out)
end

-- relatedTitles resolves relation ids to their display titles in one batch
-- request: MangaDex lists related manga as bare ids with no attributes.
local function relatedTitles(ids)
    if #ids == 0 then
        return {}
    end
    local query = {}
    for _, id in ipairs(ids) do
        query[#query + 1] = "ids[]=" .. id
    end
    local body = get(API .. "/manga?limit=100&" .. table.concat(query, "&"))
    if not body or type(body.data) ~= "table" then
        return {}
    end
    local byID = {}
    for _, manga in ipairs(body.data) do
        byID[manga.id] = pick(manga.attributes.title)
    end
    return byID
end

local function related(data)
    local ids, seen = {}, {}
    for _, rel in ipairs(data.relationships or {}) do
        if rel.type == "manga" and not seen[rel.id] then
            seen[rel.id] = true
            ids[#ids + 1] = rel.id
        end
    end
    local byID = relatedTitles(ids)
    local out = {}
    for _, id in ipairs(ids) do
        local title = byID[id]
        if title and title ~= "" then
            out[#out + 1] = { value = title, url = SITE .. "/title/" .. id }
        end
    end
    return items(out)
end

local BY_KIND = {
    titles = altTitles,
    summaries = summaries,
    categories = categories,
    authors = authors,
    related = related,
}

-- getEnrichment(arg) — arg is {"title":..., "kind":..., "source":...}.
-- Returns a JSON array of {value, url}; an unknown kind or a failed lookup
-- yields an empty array, which the host records as "nothing found".
function getEnrichment(arg)
    local req = host.json.decode(arg)
    if not req or not req.title or req.title == "" then
        return items({})
    end

    local build = BY_KIND[req.kind]
    if not build then
        return items({})
    end

    local data = detail(req.title)
    if not data or not data.attributes then
        log.debug("mangadex info: no match for " .. req.title .. " (" .. req.kind .. ")")
        return items({})
    end

    return build(data)
end
