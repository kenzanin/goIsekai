-- views/library.lua
-- Manga library grid with stats sidebar and duplicate detection.
-- Replaces views/library.jet.
-- Called as: library(data) -> string (body HTML only, layout wraps it)
-- data.Mangas: []database.Manga, data.Q: string, data.Page/TotalPages: int
-- data.Stats: {TotalTitles, StatusLine, ReadLine, HasUpdates, ReadingTime, MostLine, HasFewest, FewestLine}
-- data.LibraryStats: map[string]map[string]any, data.Ratios: map[string]float64
-- data.DuplicateCount: int, data.DuplicateGroups: []DuplicateGroup
-- data.PluginCounts: []PluginCount

local pagination = require("partials.pagination")

return function(data)
	local mangas = data.Mangas or {}
	local q = data.Q or ""
	local page = data.Page or 1
	local totalPages = data.TotalPages or 1
	local stats = data.Stats or {}
	local libraryStats = data.LibraryStats or {}
	local ratios = data.Ratios or {}
	local duplicateCount = data.DuplicateCount or 0
	local duplicateGroups = data.DuplicateGroups or {}
	local pluginCounts = data.PluginCounts or {}

	local sidebarHTML = ""
	if #mangas > 0 and q == "" then
		-- Build stats cards
		local statCards = ""
		local function addStat(value, label, color, longText)
			local cls = color
				and (' class="text-sm font-medium ' .. h(color) .. '"')
				or (longText and ' class="text-sm font-medium"' or ' class="text-lg font-semibold"')
			statCards = statCards
				.. '<div class="bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-3">'
				.. '<div' .. cls .. '>' .. h(tostring(value)) .. '</div>'
				.. '<div class="text-xs text-neutral-400">' .. h(label) .. '</div>'
				.. '</div>'
		end

		if stats.TotalTitles then
			addStat(stats.TotalTitles, "titles", "")
		end
		if stats.StatusLine and stats.StatusLine ~= "" then
			addStat(stats.StatusLine, "status", "", true)
		end
		if stats.ReadLine and stats.ReadLine ~= "" then
			addStat(stats.ReadLine, "read", "", true)
		end
		if stats.HasUpdates and stats.HasUpdates > 0 then
			addStat(stats.HasUpdates, "updates", "text-indigo-400", false)
		end
		if stats.ReadingTime and stats.ReadingTime ~= "" then
			addStat(stats.ReadingTime, "reading time", "", true)
		end
		if stats.MostLine and stats.MostLine ~= "" then
			addStat(stats.MostLine, "most chapters", "", true)
		end
		if stats.HasFewest and stats.FewestLine and stats.FewestLine ~= "" then
			addStat(stats.FewestLine, "fewest chapters", "", true)
		end

		-- Duplicate groups
		local dupItems = ""
		if duplicateCount == 0 then
			dupItems = '<div class="text-xs text-neutral-500">Tidak ada judul yang berpotensi duplikat.</div>'
		else
			for _, g in ipairs(duplicateGroups) do
				local members = ""
				for _, m in ipairs(g.Members or {}) do
					local cover = ""
					if m.CoverURL and m.CoverURL ~= "" then
						cover = '<img src="/image?pluginID=' .. h(m.PluginID or "") .. '&amp;url=' .. h(m.CoverURL)
							.. '" alt="' .. h(m.Title or "")
							.. '" class="w-8 h-11 object-cover rounded shrink-0" loading="lazy">'
					else
						cover = '<div class="w-8 h-11 rounded bg-neutral-800 flex items-center justify-center text-[10px] text-neutral-500 font-semibold shrink-0">'
							.. h(getInitials(m.Title or ""))
							.. '</div>'
					end
					members = members
						.. '<a href="/view/manga/' .. h(m.PluginID or "") .. '/' .. h(m.SourceMangaID or "")
						.. '" class="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-neutral-800/60 transition group">'
						.. cover
						.. '<span class="text-xs text-neutral-300 line-clamp-2 group-hover:text-neutral-100">' .. h(m.Title or "")
						.. '</span></a>'
				end
				dupItems = dupItems
					.. '<div class="flex flex-col gap-2">'
					.. '<div class="text-xs font-medium text-neutral-300 line-clamp-2" title="' .. h(g.Title or "")
					.. '">' .. h(g.Title or "") .. '</div>'
					.. '<div class="flex flex-col gap-1.5">' .. members .. '</div>'
					.. '</div>'
			end
		end

		sidebarHTML = [[<aside class="lg:w-56 shrink-0 flex flex-col gap-3">
    <div class="bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-3">
        <div class="flex flex-col gap-3">]]
			.. statCards
			.. [[</div>
    </div>

    <details class="bg-neutral-900 border border-neutral-700 rounded-lg overflow-hidden]]
			.. (duplicateCount == 0 and " open" or "")
			.. [[">
        <summary class="px-4 py-3 cursor-pointer select-none text-xs font-medium text-neutral-200 hover:bg-neutral-800/60 transition list-none flex items-center justify-between gap-2">
                <span>Duplicate <span class="text-indigo-400 font-semibold">]] .. h(tostring(duplicateCount))
			.. [['</span></span>
                <svg class="w-4 h-4 text-neutral-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path></svg>
        </summary>
        <div class="border-t border-neutral-800 px-3 py-3 flex flex-col gap-4">
]]
			.. dupItems
			.. [[
        </div>
    </details>]]

		-- Plugin counts
		if #pluginCounts > 0 then
			local pluginRows = ""
			for _, p in ipairs(pluginCounts) do
				local iconHTML = ""
				if p.Icon and p.Icon ~= "" then
					iconHTML = '<img src="' .. h(p.Icon) .. '" alt="" class="h-2.5 w-2.5 rounded-sm object-cover shrink-0">'
				end
				pluginRows = pluginRows
					.. '<div class="flex items-center justify-between gap-2 text-xs rounded-md px-1.5 py-1">'
					.. '<span class="flex items-center gap-1.5 min-w-0">'
					.. iconHTML
					.. '<span class="text-neutral-300 truncate">' .. h(p.Name or "") .. '</span>'
					.. '</span>'
					.. '<span class="font-semibold text-indigo-400 shrink-0">' .. h(tostring(p.Count or 0))
					.. '</span></div>'
			end
			sidebarHTML = sidebarHTML
				.. [[
        <div class="bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-3">
            <div class="text-xs font-medium text-neutral-400 mb-2">titles per plugin</div>
            <div class="flex flex-col gap-1.5">]]
				.. pluginRows
				.. [[</div>
        </div>]]
		end

		sidebarHTML = sidebarHTML .. [[</aside>
    <div class="flex-1 min-w-0">]]
	end

	-- Build manga cards
	local mangaCards = ""
	if #mangas == 0 then
		mangaCards = [[<div class="py-16 text-center text-neutral-500">
    <p class="mb-2">Your library is empty — search for manga first</p>
    <a href="/view/search" class="text-indigo-400 hover:text-indigo-300 text-sm">Search manga</a>
</div>]]
	else
		local pagHTML = pagination({
			Pagination = { Base = "/", Param = "page", Current = page, Total = totalPages },
		})
		mangaCards = pagHTML
			.. '<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6 gap-4">'
		for _, m in ipairs(mangas) do
			local mid = m.ID or ""
			local pluginID = m.PluginID or ""
			local sourceMangaID = m.SourceMangaID or ""
			local title = m.Title or ""
			local coverURL = m.CoverURL or ""
			local status = m.Status or ""
			local statsObj = libraryStats[mid]
			local ratio = ratios[pluginID] or 0

			local coverHTML
			if coverURL ~= "" then
				if ratio > 0 then
					coverHTML = '<img src="/image?pluginID=' .. h(pluginID) .. '&amp;url=' .. h(coverURL)
						.. '" alt="' .. h(title)
						.. '" style="aspect-ratio: ' .. h(tostring(ratio)) .. '" class="w-full object-cover" loading="lazy">'
				else
					coverHTML = '<img src="/image?pluginID=' .. h(pluginID) .. '&amp;url=' .. h(coverURL)
						.. '" alt="' .. h(title)
						.. '" class="w-full aspect-[2/3] object-cover" loading="lazy">'
				end
			else
				coverHTML = '<div class="w-full aspect-[2/3] bg-neutral-800 flex items-center justify-center text-neutral-500 text-2xl font-semibold">'
					.. h(getInitials(title))
					.. '</div>'
			end

			local statsHTML = ""
			if statsObj then
				local totalCh = statsObj.TotalChapters or 0
				local readCh = statsObj.ReadChapters or 0
				local unread = totalCh - readCh
				local pluginIcon = statsObj.PluginIcon or ""
				local pluginName = statsObj.PluginName or ""
				local hasNew = statsObj.HasNew or false
				local iconHTML = pluginIcon ~= ""
					and ('<img src="' .. h(pluginIcon) .. '" alt="" class="inline h-2.5 w-2.5 rounded-sm object-cover align-[-0.5px]"> ')
					or ""
				statsHTML = '<div class="flex items-center gap-2 mt-1">'
					.. '<span class="rounded-full text-xs px-2 py-0.5 bg-neutral-800 text-neutral-400"><span class="text-indigo-400 font-medium">'
					.. h(tostring(unread))
					.. ' unread</span> of '
					.. h(tostring(totalCh))
					.. ' · '
					.. iconHTML
					.. h(pluginName)
					.. '</span>'
					.. (hasNew and '<span class="text-[10px] px-1.5 py-0.5 rounded-full bg-red-500 text-white font-medium">New</span>' or "")
					.. '</div>'
			end

			mangaCards = mangaCards
				.. '<a href="/view/manga/' .. h(pluginID) .. '/' .. h(sourceMangaID)
				.. '" class="bg-neutral-900 rounded-lg overflow-hidden relative hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40 transition">'
				.. '<div class="relative" data-key="' .. h(pluginID) .. ':' .. h(sourceMangaID) .. '">'
				.. coverHTML
				.. '<div class="lib-dim" style="display:none;position:absolute;inset:0;background:rgba(0,0,0,0.82);border-radius:0.5rem;"></div>'
				.. '</div>'
				.. (status ~= ""
					and ('<span class="absolute top-2 left-2 bg-black/60 backdrop-blur rounded-full px-2 py-0.5 text-[10px] text-neutral-200">' .. h(status) .. '</span>')
					or "")
				.. '<div class="p-3">'
				.. '<div class="text-sm font-medium line-clamp-2" title="' .. h(title) .. '">' .. h(title) .. '</div>'
				.. statsHTML
				.. '</div></a>'
		end
		mangaCards = mangaCards .. '</div>'
			.. pagination({
				Pagination = { Base = "/", Param = "page", Current = page, Total = totalPages },
			})
	end

	return '<div class="flex items-center justify-between gap-3 mb-6">'
		.. '<h1 class="text-xl font-semibold shrink-0">Library</h1>'
		.. '<form method="get" action="/view/library" class="flex-1 max-w-md" role="search">'
		.. '<div class="flex gap-2">'
		.. '<input type="search" name="q" value="' .. h(q)
		.. '" placeholder="Search library…" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-1.5 text-sm placeholder-neutral-500 focus:outline-none focus:border-indigo-500">'
		.. '<button type="submit" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-sm font-medium">Search</button>'
		.. '</div></form>'
		.. '<form method="post" action="/action/sync">'
		.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">⟳ Update</button>'
		.. '</form></div>'
		.. sidebarHTML
		.. mangaCards
		.. (sidebarHTML ~= "" and "</div></div>" or "")
end
