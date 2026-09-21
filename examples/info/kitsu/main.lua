-- Kitsu enrichment script for goIsekai.
--
-- Lives in the info directory (default app_data/info, one folder per source).
-- Fetches manga metadata from Kitsu.io. Used as a secondary source when
-- MangaDex cross-source identifiers are available.
--
-- Inherits title matching, language selection, and link stripping practices
-- from the Mangadex info script.

PLUGIN = {
	contract_version = 1,
	name = "Kitsu Info",
	site_url = "https://kitsu.io",
	enrichment_providers = {
		{
			id = "kitsu",
			name = "Kitsu",
			precedence = 100, -- Lower than MangaDex (default 0) as per design
			kinds = { "titles", "summaries", "categories", "authors", "related" },
		},
	},
}

local API = "https://api.kitsu.io"
local SITE = "https://kitsu.io"

-- Languages preferred for a displayed string, most preferred first.
local PREFERRED = { "en", "ja-ro", "ko-ro", "ja", "ko" }

-- Single-slot memo: enrichment runs kind by kind for one manga at a time.
local memoTitle, memoDetail

-- stripLinkBlocks is also used from host.text if available
local function stripLinkBlocks(s)
	-- Remove "Links:" blocks and trailing URLs
	s = s:gsub("(?i)Links?:.*$\n?", "")
	s = s:gsub("(?m)^%s*https?://%S+%s*$", "")
	return s
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

-- titleMatches checks if a record's title matches the searched title
local function titleMatches(record, searchedTitle)
	local normalizedSearch = host.text.normalize_title(searchedTitle)
	local title = record.attributes and record.attributes.canonicalTitle or ""
	if title ~= "" and host.text.normalize_title(title) == normalizedSearch then
		return true
	end
	-- Check alternative titles
	local titles = record.attributes and record.attributes.title or {}
	for _, t in ipairs(titles) do
		if type(t) == "string" and t ~= "" and host.text.normalize_title(t) == normalizedSearch then
			return true
		end
	end
	return false
end

local function items(list)
	return host.json.encode(list)
end

-- detail fetches manga by Kitsu id or by title search
local function detail(title)
	if memoTitle == title then
		return memoDetail
	end
	memoTitle, memoDetail = title, nil

	-- First try: if we have a cross-source Kitsu ID from MangaDex
	-- The host should pass this in via some mechanism; for now, fall back to search
	-- This would need host support to work with MangaDex cross-ID

	-- Fallback: title search
	local search = host.http.get(
		API .. "/manga?filter[text]=" .. host.text.url_encode(title),
		{ ["Accept"] = "application/vnd.api+json" }
	)
	if not search or search.status ~= 200 then
		return nil
	end

	local body = host.json.decode(search.body)
	if not body or not body.data or #body.data == 0 then
		return nil
	end

	-- Verify the first result matches
	local record = body.data[1]
	if not titleMatches(record, title) then
		return nil
	end

	-- Fetch full details
	local detailResp = host.http.get(
		API .. "/manga/" .. record.id .. "?include=genres,authors,categories,squareCoverImage",
		{ ["Accept"] = "application/vnd.api+json" }
	)
	if not detailResp or detailResp.status ~= 200 then
		return nil
	end

	local full = host.json.decode(detailResp.body)
	if not full or not full.data then
		return nil
	end

	memoDetail = full.data
	return memoDetail
end

local function altTitles(data)
	local out, seen = {}, {}
	local function add(t)
		if t and t ~= "" and not seen[t] then
			seen[t] = true
			out[#out + 1] = { value = t, url = SITE .. "/manga/" .. data.id }
		end
	end
	local titles = data.attributes and data.attributes.title or {}
	if type(titles) == "table" then
		for _, t in ipairs(titles) do
			if type(t) == "string" then
				add(t)
			end
		end
	end
	local canonical = data.attributes and data.attributes.canonicalTitle
	if canonical and canonical ~= "" then
		add(canonical)
	end
	return items(out)
end

local function summaries(data)
	local desc = pick(data.attributes and data.attributes.description or {})
	if desc == "" then
		return items({})
	end
	desc = stripLinkBlocks(desc)
	return items({ { value = desc, url = SITE .. "/manga/" .. data.id } })
end

local function categories(data)
	local out, seen = {}, {}
	local included = data.included or {}
	for _, item in ipairs(included) do
		if item.type == "genres" then
			local name = pick(item.attributes and item.attributes.name or {})
			if name and name ~= "" and not seen[name] then
				seen[name] = true
				out[#out + 1] = { value = name, url = SITE .. "/manga/" .. data.id }
			end
		end
	end
	return items(out)
end

local function authors(data)
	local out, seen = {}, {}
	local included = data.included or {}
	for _, item in ipairs(included) do
		if item.type == "authors" then
			local name = item.attributes and item.attributes.name
			if name and name ~= "" and not seen[name] then
				seen[name] = true
				out[#out + 1] = { value = name, url = SITE .. "/manga/" .. data.id }
			end
		end
	end
	return items(out)
end

-- related: fetch related manga via "relationships" when available
local function related(data)
	local out = {}
	local related = data.relationships or {}
	for _, rel in ipairs(related) do
		if rel.type == "manga" and rel.id then
			-- Would need bulk fetch for titles; for now just note the ID
			out[#out + 1] = { value = rel.id, url = SITE .. "/manga/" .. rel.id }
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

-- getEnrichment(arg) — arg is {"title":..., "kind":..., "source":...}
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
		log.debug("kitsu info: no match for " .. req.title .. " (" .. req.kind .. ")")
		return items({})
	end

	return build(data)
end
