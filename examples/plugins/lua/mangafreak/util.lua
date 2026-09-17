-- util.lua — Mangafreak HTML parsing helpers
-- Sibling module required by main.lua via require("util")
local util = {}

-- normalizeStatus maps a raw status string to the canonical host vocabulary.
-- Canonical set: Ongoing, Completed, Hiatus, Dropped, Upcoming.
-- The site appends " series" ("COMPLETED series"), so drop that suffix before
-- delegating to the host map.
local function normalizeStatus(s)
    local raw = (s or ""):gsub("%s+series%s*$", "")
    return host.text.normalize_status(raw)
end

-- ─── Search result parsing ─────────────────────────────────────────────────
-- GET /Find/QUERY page. Results in div.manga_search_item blocks.

function util.parse_search(html)
    local results = {}
    local seen = {}

    for block in string.gmatch(html, '<div%s+class="mangak?a?_search_item">(.-)</div>') do
        local href, title = block:match('<h%a+%s*>%s*<a[^>]*href="([^"]+)"[^>]*>([^<]+)</a>')
        if not href then
            href, title = block:match('<a[^>]*href="([^"]+)"[^>]*>([^<]+)</a>')
        end
        if href then
            local slug = href:match('/[Mm]anga/([^/"?]+)')
            if slug and not seen[slug] then
                seen[slug] = true
                local cover = block:match('data%-src="([^"]+)"') or block:match('<img[^>]*src="([^"]+)"') or ""
                results[#results + 1] = {
                    id = slug,
                    title = host.text.trim(title),
                    cover_url = cover
                }
            end
        end
    end

    -- Broader fallback
    if #results == 0 then
        for href, title in string.gmatch(html, '<a[^>]*href="([^"]*[Mm]anga/[^"]+)"[^>]*>([^<]+)</a>') do
            local slug = href:match('/[Mm]anga/([^/"?]+)')
            if slug and not seen[slug] and host.text.trim(title) ~= "" then
                seen[slug] = true
                results[#results + 1] = {
                    id = slug,
                    title = host.text.trim(title),
                    cover_url = ""
                }
            end
        end
    end

    return results
end

-- ─── Manga detail parsing ─────────────────────────────────────────────────

function util.parse_manga_detail(html, manga_id)
    local detail = { id = manga_id }

    -- Title: <h1>TITLE</h1>
    detail.title = host.text.strip_html(html:match('<h1>(.-)</h1>') or "")
    if detail.title == "" then
        local data_block = html:match('class="manga_series_data">(.-)%s*</div>%s*</div>') or ""
        detail.title = host.text.strip_html(data_block:match('<h5>(.-)</h5>')) or ""
    end

    -- Cover
    local img_block = html:match('class="manga_series_image">(.-)</div>') or ""
    detail.cover_url = img_block:match('<img[^>]*src="([^"]*)"') or ""

	-- Status, Author, Artist — matched by row label, not fixed index, because
	-- the info div set varies per page (some rows absent), which shifts indexes.
	local data_block = html:match('class="manga_series_data">(.-)%s*</div>%s*</div>')
		or html:match('class="manga_series_data">(.-)</div>') or ""
	local divs = {}
	for d in data_block:gmatch('<div[^>]*>(.-)</div>') do
		divs[#divs + 1] = host.text.strip_html(d)
	end
	local function rowValue(label)
		for _, v in ipairs(divs) do
			local trimmed = host.text.trim(v)
			if trimmed:sub(1, #label):lower() == label:lower() then
				local val = trimmed:sub(#label + 1)
				return host.text.trim(val:gsub("^%s*:?%s*", "")), true
			end
		end
		return nil, false
	end
	local statusVal, statusFound = rowValue("This is ")
	if statusFound then
		detail.status = normalizeStatus(statusVal)
	else
		detail.status = normalizeStatus(divs[3])
	end
	detail.author, _ = rowValue("Written By")
	detail.artist, _ = rowValue("Illustrated By")

    -- Genres
    local genres = {}
    local genre_block = html:match('class="series_sub_genre_list">(.-)</div>') or ""
    for g in genre_block:gmatch('<a[^>]*>([^<]+)</a>') do
        local name = host.text.trim(g)
        if name ~= "" then genres[#genres + 1] = name end
    end
    detail.genres = genres

    -- Description. The block leads with a "Synopsis" label div before the real
    -- text in a <p>. Matching to the first </div> stops at that label, and the
    -- fallback then strips the label down to itself, so anchor on the closing
    -- tag pair instead to reach the <p>.
    local desc_section = html:match('class="manga_series_description">(.-)</div>%s*</div>') or ""
    detail.description = host.text.strip_html(desc_section:match('<p>(.-)</p>') or desc_section)

    return detail
end

-- ─── Chapter list parsing ──────────────────────────────────────────────────
-- Scan ALL /Read1_ links across the full HTML page.
-- Series_sub_chapter_list block has nested divs that truncate (.-)</div>,
-- so we scan globally instead.

function util.parse_chapter_list(html, manga_id)
    local chapters = {}
    local seen = {}

    -- The authoritative complete chapter list is the manga_series_list TABLE
    -- (oldest-first: Chapter 1..N). The series_sub_chapter_list block above it
    -- holds only the latest ~4 chapters (newest-first), so scanning the whole
    -- page mixes both blocks and scrambles the order.
    local block = html:match('class="manga_series_list">(.-)</table>')
        or html

    for href, name in block:gmatch('<a[^>]*href="([^"]*Read1_[^"]+)"[^>]*>([^<]+)</a>') do
        local ch_name = host.text.trim(name)
        if ch_name ~= "" then
            local ch_path = href:match("/Read1_(.+)")
            local chapter_id = manga_id .. ":" .. (ch_path or ch_name)
            if not seen[chapter_id] then
                seen[chapter_id] = true
                chapters[#chapters + 1] = {
                    id = chapter_id, manga_id = manga_id,
                    chapter_num = host.text.chapter_num(ch_name),
                    title = ch_name, url = href
                }
            end
        end
    end

    -- Table lists oldest-first; reverse to the ABI newest-first order.
    local reversed = {}
    for i = #chapters, 1, -1 do
        reversed[#reversed + 1] = chapters[i]
    end
    return reversed
end

-- ─── Page list parsing ─────────────────────────────────────────────────────
-- Chapter reader: img#gohere[src] for each page.

function util.parse_page_list(html)
    local pages = {}
    local n = 0
    for src in string.gmatch(html, '<img[^>]*id="gohere"[^>]*src="([^"]*)"') do
        n = n + 1
        pages[n] = { url = src }
    end
    if #pages == 0 then
        for src in string.gmatch(html, '<img[^>]*src="(https?://[^"]+)"') do
            if not src:match('%.svg$') and not src:match('logo') and not src:match('icon') then
                n = n + 1
                pages[n] = { url = src }
            end
        end
    end
    return pages
end

return util
