-- MangaUpdates enrichment script for goIsekai.
--
-- Lives in the info directory (default app_data/info, one folder per source),
-- NOT in the plugin directory: this fetches manga metadata rather than
-- scraper content, so it never appears as a manga source and is never used to
-- read a chapter.
--
-- Fields served here:
--   titles      alternative titles
--   summaries   synopsis
--   categories  genres
--   authors     author/artist names
--   related     the site's "Recommendations" block, each linked to its page
--
-- MangaUpdates answers a search with the series id only, so one title costs two
-- requests (search, then detail). The detail response carries every field, so it
-- is memoized in the VM and reused by the remaining kinds of the same title.

PLUGIN = {
    contract_version = 1,
    name = "MangaUpdates Info",
    site_url = "https://www.mangaupdates.com",
    enrichment_providers = {
        {
            id = "mangaupdates",
            name = "MangaUpdates",
            kinds = { "titles", "summaries", "categories", "authors", "related" },
        },
    },
}

local API = "https://api.mangaupdates.com/v1"
local SITE = "https://www.mangaupdates.com"

-- Single-slot memo: enrichment runs kind by kind for one manga at a time, so
-- the previous title's detail is never needed again.
local memoTitle, memoDetail

local function decode(resp)
    if not resp or resp.status ~= 200 then
        return nil
    end
    local body, err = host.json.decode(resp.body)
    if err or not body then
        return nil
    end
    return body
end

-- detail searches for the best match and loads its full record, memoized.
-- Only the first result is used: the rest are unrelated series. The search
-- record already carries genres and description, so it stands in when the
-- detail response leaves them out.
local function detail(title)
    if memoTitle == title then
        return memoDetail
    end
    memoTitle, memoDetail = title, nil

    local search = decode(host.http.post(API .. "/series/search", host.json.encode({ search = title }), {
        ["Content-Type"] = "application/json",
        ["Accept"] = "application/json",
    }))
    local results = search and search.results
    if type(results) ~= "table" or #results == 0 then
        log.debug("mangaupdates info: no match for " .. title)
        return nil
    end

    -- A light novel tops the results for the title of the manga adapted from
    -- it, and its metadata carries a "(Novel)" suffix that makes the alternative
    -- titles differ from every other source's. Take the first manga instead.
    local record
    for _, hit in ipairs(results) do
        if hit.record and hit.record.type ~= "Novel" then
            record = hit.record
            break
        end
    end
    if not record then
        log.debug("mangaupdates info: no match for " .. title)
        return nil
    end

    -- Verify the search result matches the searched title
    local recordTitle = record.name or ""
    if host.text.normalize_title(recordTitle) ~= host.text.normalize_title(title) then
        -- Also check against associated titles
        local matched = false
        for _, assoc in ipairs(results[1].associated or {}) do
            if assoc.title and host.text.normalize_title(assoc.title) == host.text.normalize_title(title) then
                matched = true
                break
            end
        end
        if not matched then
            log.debug("mangaupdates info: title mismatch for " .. title)
            return nil
        end
    end

    local full = decode(host.http.get(API .. "/series/" .. ("%d"):format(record.series_id), {
        ["Accept"] = "application/json",
    }))
    local data = full or {}
    data.url = data.url or record.url or SITE
    data.genres = data.genres or record.genres
    if not data.description or data.description == "" then
        data.description = record.description
    end

    memoDetail = data
    return memoDetail
end

local function items(list)
    return host.json.encode(list)
end

-- collected turns a list of {name=..., url=...} pairs into deduped items.
local function collected(data, entries, name)
    local out, seen = {}, {}
    for _, entry in ipairs(entries or {}) do
        local value = name(entry)
        if value and value ~= "" and not seen[value] then
            seen[value] = true
            out[#out + 1] = { value = value, url = entry.url or data.url }
        end
    end
    return items(out)
end

local function titles(data)
    local associated = {}
    for _, entry in ipairs(data.associated or {}) do
        associated[#associated + 1] = { name = entry.title, url = data.url }
    end
    return collected(data, associated, function(entry)
        return entry.name
    end)
end

local function summaries(data)
    if not data.description or data.description == "" then
        return items({})
    end
    -- Strip link blocks and markdown, then remove trailing URLs
    local desc = host.text.strip_link_blocks(host.text.strip_markdown(data.description))
    return items({ { value = desc, url = data.url } })
end

local function categories(data)
    local genres = {}
    for _, entry in ipairs(data.genres or {}) do
        genres[#genres + 1] = { name = entry.genre, url = data.url }
    end
    return collected(data, genres, function(entry)
        return entry.name
    end)
end

local function authors(data)
    return collected(data, data.authors, function(entry)
        return entry.name
    end)
end

-- related returns category_recommendations[], the list the site renders under
-- "Recommendations". related_series[] is deliberately unused: it carries only
-- direct plot relations, which are empty or single-entry for most series.
local function related(data)
    local recs = {}
    for _, entry in ipairs(data.category_recommendations or {}) do
        recs[#recs + 1] = { name = entry.series_name, url = entry.series_url }
    end
    return collected(data, recs, function(entry)
        return entry.name
    end)
end

local BY_KIND = {
    titles = titles,
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
    if not data then
        return items({})
    end

    return build(data)
end
