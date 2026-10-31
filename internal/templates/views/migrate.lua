-- views/migrate.lua
-- Search-like picker for migration targets. Looks like /view/search.

local migrateCandidates = require("partials.migrate_candidates")

return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local manga = data.Manga or {}
	local plugins = data.Plugins or {}
	local q = data.Q or ""
	local selKey = data.PluginIDs or ""
	local selSet, selNames = {}, {}
	for id in string.gmatch(selKey, "[^,]+") do
		selSet[id] = true
	end
	local cands = data.Candidates or {}
	local failures = data.Failures or {}
	local searchErr = data.SearchError or ""

	-- Back
	local body = '<div class="mb-4"><a href="/view/manga/'
		.. h(pluginID)
		.. "/"
		.. h(mangaID)
		.. '" class="inline-flex items-center gap-1.5 text-sm text-neutral-400 hover:text-neutral-200 transition"><svg xmlns="http://www.w3.org/2000/svg" class="size-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg> Back to detail</a></div>'
	-- Original manga header with cover + plugin badge (so user knows what is being migrated)
	local origCover = manga.CoverURL or ""
	local origCoverHTML = ""
	if origCover ~= "" then
		origCoverHTML = '<img src="/image?pluginID='
			.. h(pluginID)
			.. "&url="
			.. h(origCover)
			.. '" alt="" class="w-20 h-28 object-cover rounded-md border border-neutral-800">'
	end
	local origPluginName, origPluginIcon = "", ""
	for _, p in ipairs(plugins) do
		if p.ID == pluginID then
			origPluginName = p.Name ~= "" and p.Name or p.ID
			origPluginIcon = p.IconURL or ""
			break
		end
	end
	local origBadge = '<span class="inline-flex items-center gap-1.5 text-sm px-2.5 py-1 rounded-full bg-neutral-800 text-neutral-300">'
		.. (origPluginIcon ~= "" and '<img src="' .. h(origPluginIcon) .. '" class="h-5 w-5 rounded-sm">' or "")
		.. h(origPluginName)
		.. "</span>"
	body = body .. '<div class="flex gap-4 p-4 bg-neutral-900 rounded-lg border border-neutral-800 mb-4">'
	body = body .. origCoverHTML
	body = body .. '<div class="flex-1 min-w-0">'
	body = body .. '<div class="text-xs text-neutral-500 mb-1">Migrating from</div>'
	body = body .. '<h1 class="text-lg font-semibold mb-1 leading-tight">' .. h(manga.Title or "") .. "</h1>"
	local srcCh = data.SourceChapterCount or 0
	body = body
		.. '<div class="text-xs text-neutral-400 mt-1">'
		.. tostring(srcCh)
		.. (srcCh == 1 and " chapter in this entry" or " chapters in this entry")
		.. "</div>"
	body = body .. origBadge
	body = body .. "</div></div>"
	body = body
		.. '<p class="text-sm text-neutral-500 mb-4">Pick which source to move this entry to. Edit the query or filter by plugin — results are live search.</p>'
	body = body
		.. '<div class="p-4 rounded-lg border border-amber-500/30 bg-amber-500/10 mb-4"><p class="text-sm text-amber-300">Migrating will discard downloaded pages and per-chapter reading position on the previous source and cannot be recovered. Read/unread state carries over by chapter number (chapters with number ≤ last read become read).</p></div>'

	if searchErr ~= "" then
		body = body
			.. '<div class="bg-red-500/15 border border-red-500/40 text-red-300 rounded-lg p-3 mb-4 text-sm">'
			.. h(searchErr)
			.. "</div>"
	end
	if #failures > 0 then
		body = body
			.. '<p class="text-xs text-amber-400/70 mb-3">Some sources could not be reached: '
			.. h(table.concat(failures, ", "))
			.. "</p>"
	end

	-- Form: [search box] [plugin checkboxes] [Search button]. No checkbox checked
	-- means every active plugin is searched.
	local cbHTML = ""
	for _, p in ipairs(plugins) do
		if p.IsActive then
			local ic = p.IconURL or ""
			local pname = p.Name ~= "" and p.Name or p.ID
			if selSet[p.ID] then
				selNames[#selNames + 1] = pname
			end
			cbHTML = cbHTML
				.. '<label class="inline-flex items-center gap-2 text-sm text-neutral-300 hover:text-neutral-100 cursor-pointer select-none">'
			.. '<input type="checkbox" name="pluginIDs" value="'
				.. h(p.ID)
				.. '"'
				.. (selSet[p.ID] and " checked" or "")
				.. ' class="accent-indigo-500 w-4 h-4">'
				.. (ic ~= "" and ('<img src="' .. h(ic) .. '" alt="" class="w-5 h-5 rounded object-cover">') or "")
				.. '<span class="font-medium">' .. h(pname) .. "</span>"
				.. "</label>"
		end
	end
	body = body
		.. '<form method="get" action="/view/migrate/'
		.. h(pluginID)
		.. "/"
		.. h(mangaID)
		.. '" id="migrateForm" class="mb-3">'
	body = body .. '<div class="mb-3">'
	body = body
		.. '<div class="mb-2"><label class="block text-xs text-neutral-400 mb-1">Search</label><input type="text" id="migrateQ" name="q" value="'
		.. h(q)
		.. '" placeholder="Manga title..." class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm"></div>'
	body = body
		.. '<div class="mb-2"><label class="block text-xs text-neutral-400 mb-1">Source plugins (select all to search all)</label><div class="grid grid-cols-3 sm:grid-cols-4 lg:grid-cols-5 gap-2 border border-neutral-700 rounded-md px-3 py-3 bg-neutral-900">'
		.. cbHTML
		.. "</div></div>"
	body = body
		.. '<div class="flex justify-end"><button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Search</button></div>'
	.. "</form>"

	body = body .. migrateCandidates({
		csrf_token = data.csrf_token or "",
		PluginID = pluginID,
		MangaID = mangaID,
		Q = q,
		PluginIDs = selKey,
		Candidates = cands,
		Plugins = plugins,
		SourceChapterCount = srcCh,
		Page = data.Page,
		HasNext = data.HasNext,
	})

	return body
end
