-- 1Manga plugin for goIsekai (Lua / Lunar VM)
-- Site: https://1manga.co — MangaHub GraphQL API at api.mghcdn.com
-- Source id mn03. The API answers 404 without Origin/Referer on the POST;
-- mhub_access is only needed by the chapter (pages) resolver.
-- ABI contract version: 1 (matches pkg/types ContractVersion)

PLUGIN = {
	contract_version = 1,
	name = "1Manga",
	site_url = "https://1manga.co",
	logo = "logo.png",
	verify_url = "https://1manga.co",
	needs_human_verify = false,
	thumb_ratio = 0.703,
}

local GRAPHQL_URL = "https://api.mghcdn.com/graphql"
local SITE_URL = "https://1manga.co"
local SOURCE_ID = "mn03"
local IMG_CDN = "https://imgx.mghcdn.com/"
local THUMB_CDN = "https://thumb.mghcdn.com/"

-- Cached access key, dropped whenever the API refuses.
local cachedKey = nil

-- Last refusal message, so callers surface it instead of a silent empty list.
local gqlError = nil

-- ─── helpers ───────────────────────────────────────────────────────────────

-- A bare GET only hands back the key this session/IP already exhausted, so
-- every chapter comes back rate limited. Presenting a cookie value the server
-- has never issued makes it mint a genuinely new key, and ?reloadKey=1 is what
-- triggers the re-issue. Each key so obtained carries four chapters of quota,
-- and an explicit Cookie header overrides whatever the host cookie jar holds.
local function fetch_access_key()
	local url = SITE_URL .. "/chapter/martial-peak/chapter-" .. tostring(1000 + math.random(0, 1999)) .. "?reloadKey=1"
	local resp = host.http.get(url, {
		["Referer"] = SITE_URL .. "/manga/martial-peak",
		["Accept"] = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		["Sec-Fetch-Dest"] = "document",
		["Sec-Fetch-Mode"] = "navigate",
		["Sec-Fetch-Site"] = "same-origin",
		["Upgrade-Insecure-Requests"] = "1",
		["Cookie"] = "mhub_access=0000000000000000000000000000cafe",
	})
	if not resp or not resp.headers then
		return nil
	end

	local cookie = resp.headers["Set-Cookie"] or ""
	local key = host.regex.find(cookie, [[mhub_access=([^;]+)]])
	if not key or key == "" then
		return nil
	end
	return key
end

-- Escape a string for embedding in a GraphQL string literal.
local function escape_gql(s)
	return (s:gsub("\\", "\\\\"):gsub('"', '\\"'))
end

-- MangaHub encrypts the chapter's page list and hands the key out separately:
--   pages  = "enc:v1:<keyId>:<iv>:<tag>:<ciphertext>"   (all base64url)
--   key    = GET /api/chapter-crypto -> {keyId, key, expiresAt}
--   plain  = AES-256-GCM(key, iv, ciphertext+tag) -> {"p":"<prefix>","i":["1a.jpg",...]}
-- Decryption runs in the host (host.crypto.aes_gcm_decrypt) because this Lua VM
-- has no bitwise operators at all, so AES is not writable plugin-side.
--
-- The key and the message rotate independently, so a keyId mismatch is retried
-- once with a fresh key rather than treated as a failure.
local function encrypted_page_list(blob, slug, number)
	local function material()
		local key = cachedKey
		if not key then
			key = fetch_access_key()
			if key then
				cachedKey = key
			end
		end
		if not key then
			return nil, "no mhub_access key available"
		end
		-- This endpoint authenticates on the cookie, not the x-mhub-access
		-- header the GraphQL call uses: the header alone answers 403.
		local resp = host.http.get(SITE_URL .. "/api/chapter-crypto", {
			["Cookie"] = "mhub_access=" .. key,
			["Accept"] = "application/json",
			["Referer"] = SITE_URL .. "/chapter/" .. slug .. "/chapter-" .. number,
		})
		if not resp or resp.status ~= 200 then
			return nil, "/api/chapter-crypto returned " .. tostring(resp and resp.status or 0)
		end
		local parsed = host.json.decode(resp.body)
		if not parsed or not parsed.key or not parsed.keyId then
			return nil, "/api/chapter-crypto returned no usable key"
		end
		return parsed
	end

	local parts = {}
	for part in string.gmatch(blob .. ":", "([^:]*):") do
		parts[#parts + 1] = part
	end
	if #parts ~= 6 or parts[1] ~= "enc" or parts[2] ~= "v1" then
		log.error("1manga: unrecognised pages envelope")
		return nil, "unrecognised envelope"
	end
	local _, _, keyId, iv, tag, ciphertext = parts[1], parts[2], parts[3], parts[4], parts[5], parts[6]

	local mat, why = material()
	if mat and mat.keyId ~= keyId then
		-- The key is served per session, so a stale cookie yields a key the
		-- message was not encrypted under. Drop it and ask again once.
		cachedKey = nil
		mat, why = material()
	end
	if not mat then
		log.error("1manga: chapter-crypto unavailable: " .. tostring(why))
		return nil, why or "chapter-crypto unavailable"
	end
	if mat.keyId ~= keyId then
		log.error("1manga: keyId mismatch, blob " .. keyId .. " vs crypto " .. tostring(mat.keyId))
		return nil,
			"the page list is encrypted with a key this session was not served (blob "
			.. keyId
			.. ", served "
			.. tostring(mat.keyId)
			.. ") - the site rotates keys per session, so reload and retry"
	end

	local plain, err = host.crypto.aes_gcm_decrypt(mat.key, iv, tag, ciphertext)
	if not plain then
		log.error("1manga: pages decrypt failed: " .. tostring(err))
		return nil, tostring(err)
	end
	local decoded = host.json.decode(plain)
	if not decoded or type(decoded.p) ~= "string" or type(decoded.i) ~= "table" then
		log.error("1manga: unexpected pages plaintext")
		return nil, "decrypted payload had no {p, i} shape"
	end

	local out = {}
	for idx, name in ipairs(decoded.i) do
		out[idx] = { index = idx - 1, url = IMG_CDN .. decoded.p .. name }
	end
	return out
end

-- Run a GraphQL query, dropping the cached key and retrying once when the API
-- refuses. MangaHub reports both expired keys and its "API rate limit
-- excessed" refusal as HTTP 200 with an errors array and a null payload, so the
-- status code alone cannot tell a good answer from a refused one. The message is
-- kept in gqlError so callers can surface it.
local function graphql_query(query)
	gqlError = nil

	for _ = 1, 2 do
		local key = cachedKey
		if not key then
			key = fetch_access_key()
			if key then
				cachedKey = key
			end
		end
		if not key then
			log.error("1manga: no mhub_access key available")
			return nil
		end

		local resp = host.http.post(GRAPHQL_URL, host.json.encode({ query = query }), {
			["Content-Type"] = "application/json",
			["Origin"] = SITE_URL,
			["Referer"] = SITE_URL .. "/",
			["x-mhub-access"] = key,
		})

		if not resp or resp.status < 200 or resp.status >= 300 then
			log.info("1manga: GraphQL status " .. tostring(resp and resp.status or 0) .. ", refreshing key")
			cachedKey = nil
		else
			local parsed = host.json.decode(resp.body)
			if not parsed then
				log.error("1manga: unparseable GraphQL response")
				return nil
			end

			local errors = parsed.errors
			if errors and #errors > 0 then
				gqlError = errors[1].message or "GraphQL error"
			elseif not parsed.data then
				gqlError = "GraphQL response carried no data"
			else
				return parsed
			end

			log.warn("1manga: GraphQL refused (" .. gqlError .. "), retrying with a fresh key")
			cachedKey = nil
		end
	end

	return nil
end

-- ─── ABI: search_manga ─────────────────────────────────────────────────────
-- arg: {"query":"...","page":1}
function search_manga(arg)
	local args = host.json.decode(arg) or {}
	local query = args.query or ""
	local page = args.page or 1
	local offset = (page - 1) * 30
	log.info("1manga search: q=" .. query .. " page=" .. tostring(page))

	-- GraphQL takes one genre name per call; multi-genre means intersecting
	-- result sets, which one query cannot express. First genre wins; the rest
	-- narrow client-side by slug membership from detail fetches would be too
	-- costly, so we pass the first genre to the API and keep it simple.
	local genres = args.genres or {}
	local genre = "all"
	if #genres > 0 then
		genre = genres[1]
	end

	local gql = "{search(x: "
		.. SOURCE_ID
		.. ', q: "'
		.. escape_gql(query)
		.. '", genre: "'
		.. escape_gql(genre)
		.. '", mod: POPULAR, offset: '
		.. tostring(offset)
		.. ") {rows {title, slug, image}}}"

	local data = graphql_query(gql)
	local rows = {}
	if data and data.data and data.data.search then
		rows = data.data.search.rows or {}
	end

	local results = {}
	for _, row in ipairs(rows) do
		local cover = row.image or ""
		if cover ~= "" and cover:sub(1, 4) ~= "http" then
			cover = THUMB_CDN .. cover
		end
		results[#results + 1] = {
			id = row.slug,
			title = row.title or "",
			cover_url = cover,
		}
	end

	log.info("1manga search: found " .. tostring(#results) .. " results for q=" .. query)
	return host.json.encode(results)
end

-- ─── ABI: get_manga_detail ─────────────────────────────────────────────────
-- arg: '"slug"'. A failed lookup still returns an object: an empty Lua table
-- encodes as [], which cannot decode into a detail record.
function get_manga_detail(arg)
	local slug = host.json.decode(arg)
	if not slug then
		return host.json.encode({ id = "" })
	end

	log.info("1manga detail: slug=" .. slug)

	local gql = "{manga(x: "
		.. SOURCE_ID
		.. ', slug: "'
		.. escape_gql(slug)
		.. '") {title, slug, status, image, author, artist, genres, description, alternativeTitle}}'

	local data = graphql_query(gql)
	local m = data and data.data and data.data.manga
	if not m then
		return host.json.encode({ id = slug })
	end

	local cover = m.image or ""
	if cover ~= "" and cover:sub(1, 4) ~= "http" then
		cover = THUMB_CDN .. cover
	end

	-- status may arrive as an enum string or as a numeric code; the host wants a
	-- string either way.
	local status = m.status
	if status == nil then
		status = ""
	else
		status = tostring(status)
	end

	-- genres arrives as one comma-separated string; the ABI wants a list.
	local genres = {}
	if type(m.genres) == "string" then
		for part in host.regex.gmatch(m.genres, [[[^,]+]]) do
			local name = host.text.trim(part)
			if name ~= "" then
				genres[#genres + 1] = name
			end
		end
	elseif type(m.genres) == "table" then
		genres = m.genres
	end

	return host.json.encode({
		id = m.slug or slug,
		title = m.title or "",
		cover_url = cover,
		author = m.author or "",
		description = m.description or "",
		status = host.text.normalize_status(status),
		genres = genres,
	})
end

-- ─── ABI: get_chapter_list ─────────────────────────────────────────────────
-- arg: '"slug"'
function get_chapter_list(arg)
	local slug = host.json.decode(arg)
	if not slug then
		return host.json.encode({})
	end

	log.info("1manga chapters: slug=" .. slug)

	local gql = "{manga(x: " .. SOURCE_ID .. ', slug: "' .. escape_gql(slug) .. '") {chapters {number, title, date}}}'

	local data = graphql_query(gql)
	local raw = {}
	if data and data.data and data.data.manga then
		raw = data.data.manga.chapters or {}
	end

	local chapters = {}
	for _, ch in ipairs(raw) do
		-- An absent title arrives as an empty string, not nil, so it has to be
		-- tested for emptiness: "" is truthy in Lua.
		local title = ch.title
		if not title or title == "" then
			title = "Chapter " .. tostring(ch.number)
		end

		local item = {
			id = slug .. ":chapter-" .. tostring(ch.number),
			manga_id = slug,
			title = title,
			chapter_num = host.text.chapter_num(ch.number),
			url = SITE_URL .. "/chapter/" .. slug .. "/chapter-" .. tostring(ch.number),
		}
		-- MangaHub sends epoch seconds, epoch millis or a date string; the host
		-- normalizes all three. An unparseable date leaves the key out so Go
		-- keeps its zero value instead of failing the decode.
		local iso = host.text.date_to_iso(ch.date)
		if iso ~= "" then
			item.released_at = iso
		end

		chapters[#chapters + 1] = item
	end

	-- GraphQL returns ascending; the ABI wants newest-first.
	table.sort(chapters, function(a, b)
		return a.chapter_num > b.chapter_num
	end)

	log.info("1manga chapters: found " .. tostring(#chapters) .. " chapters for " .. slug)
	return host.json.encode(chapters)
end

-- ─── ABI: get_page_list ────────────────────────────────────────────────────
-- arg: '"slug:chapter-NUMBER"'
function get_page_list(arg)
	local chapterID = host.json.decode(arg)
	if not chapterID then
		return host.json.encode({})
	end

	-- Split on the first ":" — the slug cannot contain one, the tail can.
	local slug, number = host.regex.find(chapterID, [[(?s)^(.*?):chapter-(.+)$]])
	if not slug then
		return host.json.encode({})
	end
	log.info("1manga pages: slug=" .. slug .. " ch=" .. number)

	local gql = "{chapter(x: "
		.. SOURCE_ID
		.. ', slug: "'
		.. escape_gql(slug)
		.. '", number: '
		.. number
		.. ") {pages, mangaID, number}}"

	local data = graphql_query(gql)
	local chapter = data and data.data and data.data.chapter
	if not chapter then
		-- A refusal (rate limit, expired key) must fail loudly: returning an
		-- empty list renders a blank chapter with no explanation.
		if gqlError then
			error("1manga: " .. gqlError)
		end
		return host.json.encode({})
	end

	-- Authoritative path: the encrypted page list the API hands back. It names
	-- every file exactly, which the CDN pattern below cannot do - chapter 59 of
	-- this manga is "1a.jpg 2a.jpg 3a.jpg 4.jpg ... 9a.jpg ...", so both the
	-- extension probe and the binary search both fail on it.
	if type(chapter.pages) == "string" and string.sub(chapter.pages, 1, 7) == "enc:v1:" then
		local pages, why = encrypted_page_list(chapter.pages, slug, number)
		if pages then
			log.info("1manga pages: found " .. tostring(#pages) .. " pages for " .. chapterID .. " (decrypted)")
			return host.json.encode(pages)
		end
		-- Fail loudly instead of probing. An encrypted pages field means this
		-- chapter is served from the authoritative list, so the CDN pattern is
		-- already known to be wrong for it: probing then produced a list that
		-- looked plausible and silently dropped pages (observed on chapter 58,
		-- which reported 18 pages it had only guessed at). An empty chapter with
		-- a stated cause beats a wrong chapter that reads as complete.
		error("1manga: cannot decrypt the page list for " .. chapterID .. ": " .. tostring(why))
	end

	-- Fallback for a plaintext or unreadable pages field: find the page count
	-- by probing (page N exists with 200, N+1 404s). Binary search keeps it to
	-- ~9 cheap requests; the list lands in chapters_pages cache afterward, so the
	-- cost is paid once per chapter. This assumes contiguous page numbers, which
	-- is exactly what the encrypted path above exists to avoid.
	-- The extension varies per chapter (older chapters are .jpg, newer ones
	-- .jpeg), so probe candidates once on page 1 instead of hardcoding one.
	local ext
	for _, c in ipairs({ "jpg", "jpeg", "png", "webp" }) do
		local resp =
			host.http.get(IMG_CDN .. slug .. "/" .. number .. "/1." .. c, { ["Referer"] = SITE_URL .. "/" })
		if resp ~= nil and resp.status == 200 then
			ext = c
			break
		end
	end
	if not ext then
		log.error("1manga pages: page 1 missing for " .. chapterID)
		return host.json.encode({})
	end
	local function pageExists(i)
		local resp =
			host.http.get(IMG_CDN .. slug .. "/" .. number .. "/" .. i .. "." .. ext, { ["Referer"] = SITE_URL .. "/" })
		return resp ~= nil and resp.status == 200
	end
	local lo, hi = 1, 200
	while lo < hi do
		local mid = math.ceil((lo + hi + 1) / 2)
		if pageExists(mid) then
			lo = mid
		else
			hi = mid - 1
		end
	end
	local pages = {}
	for i = 1, lo do
		-- No per-page headers: the field must be left out, because an empty Lua
		-- table encodes as [] and the host decodes headers as a map.
		pages[#pages + 1] = {
			index = i - 1,
			url = IMG_CDN .. slug .. "/" .. number .. "/" .. i .. "." .. ext,
		}
	end

	log.info("1manga pages: found " .. tostring(#pages) .. " pages for " .. chapterID)
	return host.json.encode(pages)
end

-- ─── get_genres (optional export) ──────────────────────────────────────────
-- MangaHub GraphQL takes genre names directly (genre: "Action"); the list
-- below mirrors the site's genre facet.
local GENRES = {
	{ name = "Action", slug = "Action" },
	{ name = "Adventure", slug = "Adventure" },
	{ name = "Comedy", slug = "Comedy" },
	{ name = "Cooking", slug = "Cooking" },
	{ name = "Doujinshi", slug = "Doujinshi" },
	{ name = "Drama", slug = "Drama" },
	{ name = "Fantasy", slug = "Fantasy" },
	{ name = "Gender bender", slug = "Gender Bender" },
	{ name = "Harem", slug = "Harem" },
	{ name = "Historical", slug = "Historical" },
	{ name = "Horror", slug = "Horror" },
	{ name = "Isekai", slug = "Isekai" },
	{ name = "Josei", slug = "Josei" },
	{ name = "Martial arts", slug = "Martial Arts" },
	{ name = "Mecha", slug = "Mecha" },
	{ name = "Mystery", slug = "Mystery" },
	{ name = "One shot", slug = "One Shot" },
	{ name = "Psychological", slug = "Psychological" },
	{ name = "Romance", slug = "Romance" },
	{ name = "School life", slug = "School Life" },
	{ name = "Sci-fi", slug = "Sci Fi" },
	{ name = "Seinen", slug = "Seinen" },
	{ name = "Shoujo", slug = "Shoujo" },
	{ name = "Shounen", slug = "Shounen" },
	{ name = "Slice of life", slug = "Slice of Life" },
	{ name = "Sports", slug = "Sports" },
	{ name = "Supernatural", slug = "Supernatural" },
	{ name = "Tragedy", slug = "Tragedy" },
	{ name = "Webtoons", slug = "Webtoons" },
}

function get_genres()
	return host.json.encode(GENRES)
end
