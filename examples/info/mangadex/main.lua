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
--   related     recommended manga (what the site's Recommendations tab lists)
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
    return ""
end

-- normTitle normalizes a title for comparison: apostrophes are stripped
-- first so romanizations like "Ten'i" match the flattened "Teni" form other
-- sources use, then the host normalizer does the rest.
local function normTitle(s)
    return host.text.normalize_title((s:gsub("'", ""):gsub("\226\128\153", "")))
end

-- tokens splits a normalized title into words.
local function tokens(normalized)
    local t = {}
    for w in normalized:gmatch("%S+") do
        t[#t + 1] = w
    end
    return t
end

-- fuzzyRank scores a romanization-variant match ("Haburaretara" vs
-- "Haburareta node", "Doukyuu" vs "Doukyuusei"): every searched token must
-- find an exact or prefix-tolerant partner in the candidate (prefix matching
-- only for tokens of 4+ chars so short words stay strict). Returns nil below
-- 70% coverage or on titles shorter than 3 tokens. Lower is better; the last
-- tier after exact and contained.
local function fuzzyRank(searchTokens, candidateTokens)
    if #searchTokens < 3 then
        return nil
    end
    local used, matched = {}, 0
    for _, want in ipairs(searchTokens) do
        for i, have in ipairs(candidateTokens) do
            if not used[i] and (want == have
                or (#want >= 4 and #have >= 4
                    and (want:sub(1, #have) == have or have:sub(1, #want) == want))) then
                used[i] = true
                matched = matched + 1
                break
            end
        end
    end
    if matched < #searchTokens * 0.7 then
        return nil
    end
    return { 2, (#searchTokens - matched) * 1000 + (#candidateTokens - matched) }
end

-- titleMatchScore ranks candidates so the right series wins. Lower is better:
-- an exact normalized match always wins first (score 0); a contained match
-- (bracketed suffixes penalized) follows; romanization variants take the
-- fuzzy tier last. nil means no match at all.
local function titleMatchScore(record, searchedTitle)
    local normalizedSearch = normTitle(searchedTitle)
    local searchTokens = tokens(normalizedSearch)
    local candidates = {}
    local mainTitle = pick(record.attributes and record.attributes.title)
    if mainTitle ~= "" then
        candidates[#candidates + 1] = mainTitle
    end
    for _, entry in ipairs(record.attributes.altTitles or {}) do
        local alt = pick(entry)
        if alt ~= "" then
            candidates[#candidates + 1] = alt
        end
    end
    local best
    for _, candidate in ipairs(candidates) do
        local normalized = normTitle(candidate)
        if normalized == normalizedSearch then
            return 0
        end
        local rank
        if #normalized > #normalizedSearch and normalized:find(normalizedSearch, 1, true) then
            -- A candidate ending with the searched string is a publisher
            -- prefix ("Trapped in a Dating Sim: X") on the original; anything
            -- else carries a sequel or spin-off suffix after the match.
            local endsWith = normalized:sub(-#normalizedSearch) == normalizedSearch
            local score = #normalized - #normalizedSearch
            if normalized:sub(-1) == ")" and normalized:find("(", 1, true) then
                score = score + 50
            end
            rank = { endsWith and 0 or 1, score }
        else
            rank = fuzzyRank(searchTokens, tokens(normalized))
        end
        if rank and (not best or rank[1] < best[1] or (rank[1] == best[1] and rank[2] < best[2])) then
            best = rank
        end
    end
    return best
end

-- detail fetches the full record for the best search match, memoized.
local function detail(title)
    if memoTitle == title then
        return memoDetail
    end
    memoTitle, memoDetail = title, nil

    local search = get(API .. "/manga?limit=10&order[relevance]=desc&title=" .. host.text.url_encode(title))
    if not search or type(search.data) ~= "table" then
        return nil
    end

    -- The most relevant hit is often a sequel or spin-off whose alt title
    -- merely contains the searched string. Score every candidate so an exact
    -- match anywhere wins, and the least-divergent contained match follows.
    local record, bestScore
    for _, candidate in ipairs(search.data) do
        local score = titleMatchScore(candidate, title)
        if score == 0 then
            record = candidate
            break
        elseif score and (not bestScore
            or score[1] < bestScore[1]
            or (score[1] == bestScore[1] and score[2] < bestScore[2])) then
            record, bestScore = candidate, score
        end
    end
    if not record then
        return nil
    end

    local id = record.id
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
    -- Strip link blocks and markdown, then remove trailing URLs
    desc = host.text.strip_link_blocks(host.text.strip_markdown(desc))
    return items({ { value = desc, url = SITE .. "/title/" .. data.id } })
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

-- recommendedIDs fetches the list behind the site's Recommendations tab.
-- /manga/{id}/recommendation scores every other manga against this one and
-- returns bare ids, so the titles still need the batch lookup below. It takes
-- no limit or offset, so its first 50 are all it will ever hand back.
local function recommendedIDs(id)
    local body = get(API .. "/manga/" .. id .. "/recommendation")
    if not body or type(body.data) ~= "table" then
        return {}
    end
    local ids, seen = {}, {}
    for _, rec in ipairs(body.data) do
        for _, rel in ipairs(rec.relationships or {}) do
            if rel.type == "manga" and rel.id ~= id and not seen[rel.id] then
                seen[rel.id] = true
                ids[#ids + 1] = rel.id
            end
        end
    end
    return ids
end

-- related: the recommendations are the main list, matching what the MangaDex
-- page shows. The plot relations from the detail response are the fallback,
-- since only a minority of series have any.
local function related(data)
    local ids = recommendedIDs(data.id)
    if #ids == 0 then
        local seen = {}
        for _, rel in ipairs(data.relationships or {}) do
            if rel.type == "manga" and rel.id ~= data.id and not seen[rel.id] then
                seen[rel.id] = true
                ids[#ids + 1] = rel.id
            end
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
