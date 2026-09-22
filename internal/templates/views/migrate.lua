-- views/migrate.lua
-- Search-like picker for migration targets. Looks like /view/search.

return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local manga = data.Manga or {}
	local plugins = data.Plugins or {}
	local q = data.Q or ""
	local targetPluginID = data.TargetPluginID or ""
	local cands = data.Candidates or {}
	local failures = data.Failures or {}
	local searchErr = data.SearchError or ""

	-- Back
	local body = '<div class="mb-4"><a href="/view/manga/' .. h(pluginID) .. '/' .. h(mangaID) .. '" class="inline-flex items-center gap-1.5 text-sm text-neutral-400 hover:text-neutral-200 transition"><svg xmlns="http://www.w3.org/2000/svg" class="size-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg> Back to detail</a></div>'
	-- Original manga header with cover + plugin badge (so user knows what is being migrated)
	local origCover = manga.CoverURL or ""
	local origCoverHTML = ""
	if origCover ~= "" then
		origCoverHTML = '<img src="/image?pluginID=' .. h(pluginID) .. '&url=' .. h(origCover) .. '" alt="" class="w-20 h-28 object-cover rounded-md border border-neutral-800">'
	end
	local origPluginName, origPluginIcon = "", ""
	for _, p in ipairs(plugins) do if p.ID == pluginID then origPluginName = p.Name ~= "" and p.Name or p.ID; origPluginIcon = p.IconURL or ""; break end end
	local origBadge = '<span class="inline-flex items-center gap-1.5 text-sm px-2.5 py-1 rounded-full bg-neutral-800 text-neutral-300">' .. (origPluginIcon ~= "" and '<img src="' .. h(origPluginIcon) .. '" class="h-5 w-5 rounded-sm">' or '') .. h(origPluginName) .. '</span>'
	body = body .. '<div class="flex gap-4 p-4 bg-neutral-900 rounded-lg border border-neutral-800 mb-4">'
	body = body .. origCoverHTML
	body = body .. '<div class="flex-1 min-w-0">'
	body = body .. '<div class="text-xs text-neutral-500 mb-1">Migrating from</div>'
	body = body .. '<h1 class="text-lg font-semibold mb-1 leading-tight">' .. h(manga.Title or "") .. '</h1>'
	body = body .. origBadge
	body = body .. '</div></div>'
	body = body .. '<p class="text-sm text-neutral-500 mb-4">Pick which source to move this entry to. Edit the query or filter by plugin — results are live search.</p>'
	body = body .. '<div class="p-4 rounded-lg border border-amber-500/30 bg-amber-500/10 mb-4"><p class="text-sm text-amber-300">Migrating will discard downloaded pages and per-chapter reading position on the previous source and cannot be recovered. Read/unread state carries over by chapter number (chapters with number ≤ last read become read).</p></div>'

	if searchErr ~= "" then
		body = body .. '<div class="bg-red-500/15 border border-red-500/40 text-red-300 rounded-lg p-3 mb-4 text-sm">' .. h(searchErr) .. '</div>'
	end
	if #failures > 0 then
		body = body .. '<p class="text-xs text-amber-400/70 mb-3">Some sources could not be reached: ' .. h(table.concat(failures, ", ")) .. '</p>'
	end

	-- Plugin options for the filter (same as search page)
	local pluginOpts = '<option value="">All plugins</option>\n'
	for _, p in ipairs(plugins) do
		if p.IsActive then
			local selected = (targetPluginID == p.ID) and ' selected' or ''
			pluginOpts = pluginOpts .. '<option value="' .. h(p.ID) .. '"' .. selected .. '>' .. h(p.Name ~= "" and p.Name or p.ID) .. '</option>\n'
		end
	end

	-- Search form (GET to same migrate page)
	body = body .. '<form method="get" action="/view/migrate/' .. h(pluginID) .. '/' .. h(mangaID) .. '" class="flex flex-wrap gap-2 mb-6">'
	body = body .. '<input type="text" name="q" value="' .. h(q) .. '" placeholder="Search title" class="flex-1 min-w-[200px] bg-neutral-900 border border-neutral-800 rounded-md px-3 py-2 text-sm">'
	body = body .. '<select name="pluginID" class="bg-neutral-900 border border-neutral-800 rounded-md px-3 py-2 text-sm">' .. pluginOpts .. '</select>'
	body = body .. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Search</button>'
	body = body .. '</form>'

	-- Results grid (search-like)
	if #cands == 0 then
		if q ~= "" then
			body = body .. '<div class="py-12 text-center text-neutral-500">No results for "' .. h(q) .. '"' .. (targetPluginID ~= "" and ' in ' .. h(targetPluginID) or '') .. '</div>'
		else
			body = body .. '<div class="py-12 text-center text-neutral-500">Type a query and search</div>'
		end
	else
		-- Build plugin badge map for results
		local badgeByID = {}
		for _, p in ipairs(plugins) do
			local icon = p.IconURL or ""
			local name = p.Name ~= "" and p.Name or p.ID
			local badge = '<span class="inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-300">' .. (icon ~= "" and '<img src="' .. h(icon) .. '" class="h-5 w-5 rounded-sm">' or '') .. h(name) .. '</span>'
			badgeByID[p.ID] = badge
		end
		body = body .. '<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">'
		for _, c in ipairs(cands) do
			local coverURL = c.CoverURL or ""
			local coverHTML
			if coverURL ~= "" then
				coverHTML = '<img src="/image?pluginID=' .. h(c.PluginID) .. '&url=' .. h(coverURL) .. '" alt="' .. h(c.Title or "") .. '" class="w-full aspect-[2/3] object-cover bg-neutral-800" loading="lazy" data-fallback="' .. h((c.Title or ""):sub(1,2)) .. '">'
			else
				coverHTML = '<div class="w-full aspect-[2/3] bg-neutral-800 flex items-center justify-center text-neutral-500 text-xl font-semibold">' .. h((c.Title or ""):sub(1,2)) .. '</div>'
			end
			local badge = badgeByID[c.PluginID] or ""
			local exactBadge = ''
			if c.IsExactMatch then
				exactBadge = '<div class="absolute top-2 left-2 z-10 bg-emerald-500 text-white text-sm font-bold uppercase tracking-wide px-3 py-1 rounded-md shadow-lg shadow-black/50">EXACT</div>'
			end
			body = body .. '<div class="flex flex-col bg-neutral-900 rounded-lg overflow-hidden hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40 hover:ring-1 hover:ring-amber-500/60 transition">'
			body = body .. '<div class="relative overflow-hidden">' .. exactBadge .. coverHTML .. '</div>'
			body = body .. '<div class="p-3 flex-1 flex flex-col">'
			body = body .. '<div class="font-medium text-sm mb-1 truncate" title="' .. h(c.Title or "") .. '">' .. h(c.Title or "") .. '</div>'
			body = body .. '<div class="mb-2">' .. badge .. '</div>'
			body = body .. '<form method="post" action="/action/migrate-source/' .. h(pluginID) .. '/' .. h(mangaID) .. '" class="mt-auto">'
			body = body .. '<input type="hidden" name="targetPluginID" value="' .. h(c.PluginID) .. '">'
			body = body .. '<input type="hidden" name="targetMangaID" value="' .. h(c.SourceMangaID) .. '">'
			body = body .. '<button type="submit" class="w-full bg-amber-600 hover:bg-amber-500 text-white rounded-md px-3 py-1.5 text-xs font-medium">Migrate</button>'
			body = body .. '</form>'
			body = body .. '</div></div>'
		end
		body = body .. '</div>'
		-- Pagination (simple Prev/Next, preserves q and plugin filter)
		local page = data.Page or 1
		local hasNext = data.HasNext == true
		local hasPrev = page > 1
		if hasNext or hasPrev then
			local base = '/view/migrate/' .. h(pluginID) .. '/' .. h(mangaID) .. '?q=' .. h(q) .. '&pluginID=' .. h(targetPluginID) .. '&page='
			body = body .. '<div class="flex items-center justify-center gap-2 mt-6">'
			if hasPrev then
				body = body .. '<a href="' .. base .. tostring(page - 1) .. '" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-sm">← Prev</a>'
			else
				body = body .. '<span class="border border-neutral-700 text-neutral-600 rounded-md px-3 py-1.5 text-sm opacity-50">← Prev</span>'
			end
			body = body .. '<span class="bg-neutral-800 text-neutral-300 rounded-md px-4 py-1.5 text-sm">' .. tostring(page) .. '</span>'
			if hasNext then
				body = body .. '<a href="' .. base .. tostring(page + 1) .. '" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-sm">Next →</a>'
			else
				body = body .. '<span class="border border-neutral-700 text-neutral-600 rounded-md px-3 py-1.5 text-sm opacity-50">Next →</span>'
			end
			body = body .. '</div>'
		end
	end

	return body
end
