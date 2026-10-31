--  partials/migrate_candidates.lua
--  Candidate grid on the migrate page: empty state, result cards with the
--  chapter-count comparison, and the Prev/Next pagination. Returned as one
--  HTML string so the view keeps its own body assembly.
--  data.PluginID, data.MangaID, data.Q, data.PluginIDs, data.Candidates,
--  data.Plugins, data.SourceChapterCount, data.Page, data.HasNext,
--  data.csrf_token
return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local q = data.Q or ""
	local selKey = data.PluginIDs or ""
	local cands = data.Candidates or {}
	local plugins = data.Plugins or {}
	local srcCh = data.SourceChapterCount or 0
	local selSet, selNames = {}, {}
	for id in string.gmatch(selKey, "[^,]+") do
		selSet[id] = true
	end
	for _, p in ipairs(plugins) do
		if selSet[p.ID] then
			selNames[#selNames + 1] = (p.Name ~= "" and p.Name or p.ID)
		end
	end

	local body = ""
	-- Results grid (search-like)
	if #cands == 0 then
		if q ~= "" then
			body = body
				.. '<div class="py-12 text-center text-neutral-500">No results for "'
				.. h(q)
				.. '"'
				.. (#selNames > 0 and " in " .. h(table.concat(selNames, ", ")) or "")
				.. "</div>"
		else
			body = body .. '<div class="py-12 text-center text-neutral-500">Type a query and search</div>'
		end
	else
		-- Build plugin badge map for results
		local badgeByID = {}
		for _, p in ipairs(plugins) do
			local icon = p.IconURL or ""
			local name = p.Name ~= "" and p.Name or p.ID
			local badge = '<span class="inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-300">'
				.. (icon ~= "" and '<img src="' .. h(icon) .. '" class="h-5 w-5 rounded-sm">' or "")
				.. h(name)
				.. "</span>"
			badgeByID[p.ID] = badge
		end
		body = body .. '<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">'
		for _, c in ipairs(cands) do
			local coverURL = c.CoverURL or ""
			local coverHTML
			if coverURL ~= "" then
				coverHTML = '<img src="/image?pluginID='
					.. h(c.PluginID)
					.. "&url="
					.. h(coverURL)
					.. '" alt="'
					.. h(c.Title or "")
					.. '" class="w-full aspect-[2/3] object-cover bg-neutral-800" loading="lazy" data-fallback="'
					.. h((c.Title or ""):sub(1, 2))
					.. '">'
			else
				coverHTML = '<div class="w-full aspect-[2/3] bg-neutral-800 flex items-center justify-center text-neutral-500 text-xl font-semibold">'
					.. h((c.Title or ""):sub(1, 2))
					.. "</div>"
			end
			local badge = badgeByID[c.PluginID] or ""
			local exactBadge = ""
			if c.IsExactMatch then
				exactBadge =
					'<div class="absolute top-2 left-2 z-10 bg-emerald-500 text-white text-sm font-bold uppercase tracking-wide px-3 py-1 rounded-md shadow-lg shadow-black/50">EXACT</div>'
			end
			body = body
				.. '<div class="flex flex-col bg-neutral-900 rounded-lg overflow-hidden hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40 hover:ring-1 hover:ring-amber-500/60 transition">'
			body = body .. '<div class="relative overflow-hidden">' .. exactBadge .. coverHTML .. "</div>"
			body = body .. '<div class="p-3 flex-1 flex flex-col">'
			body = body
				.. '<div class="font-medium text-sm mb-1 truncate" title="'
				.. h(c.Title or "")
				.. '">'
				.. h(c.Title or "")
				.. "</div>"
			-- A target with fewer chapters than the entry being moved loses the
			-- surplus on migration, and the surplus cannot be recovered, so the
			-- comparison belongs on the card and not after the fact.
			local chLine
			if c.ChapterCount == nil or c.ChapterCount < 0 then
				chLine = '<span class="text-xs text-neutral-600">chapter count unavailable</span>'
			elseif srcCh > 0 and c.ChapterCount < srcCh then
				chLine = '<span class="text-xs text-red-400 font-semibold">'
					.. tostring(c.ChapterCount)
					.. " / "
					.. tostring(srcCh)
					.. " — fewer, the surplus is lost</span>"
			else
				chLine = '<span class="text-xs text-neutral-400">'
					.. tostring(c.ChapterCount)
					.. (c.ChapterCount == 1 and " chapter" or " chapters")
					.. "</span>"
			end
			body = body .. '<div class="mb-1">' .. chLine .. "</div>"
			body = body .. '<div class="mb-2">' .. badge .. "</div>"
			body = body
				.. '<form method="post" action="/action/migrate-source/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="mt-auto">'
				.. csrfInput(data.csrf_token or "")
			body = body .. '<input type="hidden" name="targetPluginID" value="' .. h(c.PluginID) .. '">'
			body = body .. '<input type="hidden" name="targetMangaID" value="' .. h(c.SourceMangaID) .. '">'
			body = body
				.. '<button type="submit" class="w-full bg-amber-600 hover:bg-amber-500 text-white rounded-md px-3 py-1.5 text-xs font-medium">Migrate</button>'
			body = body .. "</form>"
			body = body .. "</div></div>"
		end
		body = body .. "</div>"
		-- Pagination (simple Prev/Next, preserves q and plugin filter)
		local page = data.Page or 1
		local hasNext = data.HasNext == true
		local hasPrev = page > 1
		if hasNext or hasPrev then
			local base = "/view/migrate/"
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. "?q="
				.. h(q)
				.. "&pluginIDs="
				.. h(selKey)
				.. "&page="
			body = body .. '<div class="flex items-center justify-center gap-2 mt-6">'
			if hasPrev then
				body = body
					.. '<a href="'
					.. base
					.. tostring(page - 1)
					.. '" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-sm">← Prev</a>'
			else
				body = body
					.. '<span class="border border-neutral-700 text-neutral-600 rounded-md px-3 py-1.5 text-sm opacity-50">← Prev</span>'
			end
			body = body
				.. '<span class="bg-neutral-800 text-neutral-300 rounded-md px-4 py-1.5 text-sm">'
				.. tostring(page)
				.. "</span>"
			if hasNext then
				body = body
					.. '<a href="'
					.. base
					.. tostring(page + 1)
					.. '" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-sm">Next →</a>'
			else
				body = body
					.. '<span class="border border-neutral-700 text-neutral-600 rounded-md px-3 py-1.5 text-sm opacity-50">Next →</span>'
			end
			body = body .. "</div>"
		end
	end

	return body
end
