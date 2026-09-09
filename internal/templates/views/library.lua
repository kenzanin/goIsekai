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
		local function addStat(value, label, color, longText, accent)
			local cls = color and (' class="text-sm font-medium ' .. h(color) .. '"')
				or (longText and ' class="text-sm font-medium"' or (accent and ' class="text-2xl font-bold"' or ' class="text-lg font-semibold"'))
			local outer = accent
					and 'bg-neutral-900 border border-indigo-500/30 rounded-lg px-4 py-3'
				or 'bg-neutral-900 border border-neutral-800 rounded-lg px-4 py-3'
			statCards = statCards
				.. '<div class="'
				.. outer
				.. '">'
				.. "<div"
				.. cls
				.. ">"
				.. h(tostring(value))
				.. "</div>"
				.. '<div class="text-xs text-neutral-400">'
				.. h(label)
				.. "</div>"
				.. "</div>"
		end

		if stats.TotalTitles then
			addStat(stats.TotalTitles, "titles", "", false, true)
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
						cover = '<img src="/image?pluginID='
							.. h(m.PluginID or "")
							.. "&amp;url="
							.. h(m.CoverURL)
							.. '" alt="'
							.. h(m.Title or "")
							.. '" class="w-8 h-11 object-cover rounded shrink-0" loading="lazy">'
					else
						cover = '<div class="w-8 h-11 rounded bg-neutral-800 flex items-center justify-center text-[10px] text-neutral-500 font-semibold shrink-0">'
							.. h(getInitials(m.Title or ""))
							.. "</div>"
					end
					members = members
						.. '<a href="/view/manga/'
						.. h(m.PluginID or "")
						.. "/"
						.. h(m.SourceMangaID or "")
						.. '" class="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-neutral-800/60 transition group">'
						.. cover
						.. '<span class="text-xs text-neutral-300 line-clamp-2 group-hover:text-neutral-100">'
						.. h(m.Title or "")
						.. "</span></a>"
				end
				dupItems = dupItems
					.. '<div class="flex flex-col gap-2">'
					.. '<div class="text-xs font-medium text-neutral-300 line-clamp-2" title="'
					.. h(g.Title or "")
					.. '">'
					.. h(g.Title or "")
					.. "</div>"
					.. '<div class="flex flex-col gap-1.5">'
					.. members
					.. "</div>"
					.. "</div>"
			end
		end

		sidebarHTML = [[<div class="flex flex-col lg:flex-row gap-4 items-start">
<aside class="lg:w-56 shrink-0 flex flex-col gap-3">
    <div class="bg-neutral-900 border border-neutral-800 rounded-lg px-4 py-3">
        <div class="flex flex-col gap-3">]] .. statCards .. [[</div>
    </div>

    <details class="bg-neutral-900 border border-neutral-800 rounded-lg overflow-hidden]] .. (duplicateCount == 0 and " open" or "") .. [[">
        <summary class="px-4 py-3 cursor-pointer select-none text-xs font-medium text-neutral-200 hover:bg-neutral-800/60 transition list-none flex items-center justify-between gap-2">
                <span>Duplicate <span class="text-indigo-400 font-semibold">]] .. h(tostring(duplicateCount)) .. [['</span></span>
                <svg class="w-4 h-4 text-neutral-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path></svg>
        </summary>
        <div class="border-t border-neutral-800 px-3 py-3 flex flex-col gap-4">
]] .. dupItems .. [[
        </div>
    </details>]]

		-- Plugin counts
		if #pluginCounts > 0 then
			local pluginRows = ""
			for _, p in ipairs(pluginCounts) do
				local iconHTML = ""
				if p.Icon and p.Icon ~= "" then
					iconHTML = '<img src="'
						.. h(p.Icon)
						.. '" alt="" class="h-2.5 w-2.5 rounded-sm object-cover shrink-0">'
				end
				pluginRows = pluginRows
					.. '<div class="flex items-center justify-between gap-2 text-xs rounded-md px-1.5 py-1">'
					.. '<span class="flex items-center gap-1.5 min-w-0">'
					.. iconHTML
					.. '<span class="text-neutral-300 truncate">'
					.. h(p.Name or "")
					.. "</span>"
					.. "</span>"
					.. '<span class="font-semibold text-indigo-400 shrink-0">'
					.. h(tostring(p.Count or 0))
					.. "</span></div>"
			end
			sidebarHTML = sidebarHTML
				.. [[
        <div class="bg-neutral-900 border border-neutral-800 rounded-lg px-4 py-3">
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
    <svg class="w-14 h-14 mx-auto mb-4 text-neutral-700" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M5 5h6v14H5zM13 5h6v3h-6z"/></svg>
    <p class="mb-4">Your library is empty — search for manga first</p>
    <a href="/view/search" class="inline-block bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Search manga</a>
</div>]]
	else
		mangaCards = '<div class="view-container grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">'
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
					coverHTML = '<img src="/image?pluginID='
						.. h(pluginID)
						.. "&amp;url="
						.. h(coverURL)
						.. '" alt="'
						.. h(title)
						.. '" style="aspect-ratio: '
						.. h(tostring(ratio))
						.. '" class="w-full object-cover" loading="lazy">'
				else
					coverHTML = '<img src="/image?pluginID='
						.. h(pluginID)
						.. "&amp;url="
						.. h(coverURL)
						.. '" alt="'
						.. h(title)
						.. '" class="w-full aspect-[2/3] object-cover" loading="lazy">'
				end
			else
				coverHTML = '<div class="w-full aspect-[2/3] bg-neutral-800 flex items-center justify-center text-neutral-500 text-2xl font-semibold">'
					.. h(getInitials(title))
					.. "</div>"
			end

			local pluginName = statsObj and (statsObj.PluginName or "") or ""
			local pluginIcon = statsObj and (statsObj.PluginIcon or "") or ""

			-- Badges overlaid on the thumbnail
			local statusBadge = status ~= ""
					and ('<span class="absolute top-2 left-2 bg-black/60 backdrop-blur rounded-full px-2 py-0.5 text-xs text-neutral-200">' .. h(status) .. "</span>")
				or ""
			local pluginBadge = ""
			if pluginName ~= "" then
				local pIcon = pluginIcon ~= ""
					and ('<img src="' .. h(pluginIcon) .. '" alt="" class="h-3 w-3 rounded-sm object-cover">')
				or ""
				pluginBadge = '<span class="absolute bottom-2 left-2 bg-black/60 backdrop-blur rounded-full px-2 py-0.5 text-[11px] text-neutral-200 flex items-center gap-1 max-w-[90%]">'
					.. pIcon
					.. '<span class="truncate">' .. h(pluginName) .. "</span></span>"
			end

			-- read/total in the title area
			local statsBadge = ""
			if statsObj then
				statsBadge = '<span class="text-sm font-semibold text-indigo-400">'
					.. h(tostring(statsObj.ReadChapters or 0))
					.. '</span><span class="text-xs text-neutral-500">/</span><span class="text-sm font-medium text-neutral-300">'
					.. h(tostring(statsObj.TotalChapters or 0))
					.. "</span>"
					.. (statsObj.HasNew and ' <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-red-500 text-white font-medium align-middle">New</span>' or "")
			end

			mangaCards = mangaCards
				.. '<a href="/view/manga/'
				.. h(pluginID)
				.. "/"
				.. h(sourceMangaID)
				.. '" class="view-item bg-neutral-900 rounded-lg overflow-hidden relative hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40 transition flex flex-col">'
				.. '<div class="relative flex-1 min-w-0" data-key="'
				.. h(pluginID)
				.. ":"
				.. h(sourceMangaID)
				.. '">' 
				.. coverHTML
				.. '<div class="lib-dim" style="display:none;position:absolute;inset:0;background:rgba(0,0,0,0.82);border-radius:0.5rem;"></div>'
				.. statusBadge
				.. pluginBadge
				.. "</div>"
				.. '<div class="p-3 flex flex-col gap-1">'
				.. '<div class="text-sm font-medium line-clamp-2" title="'
				.. h(title)
				.. '">' 
				.. h(title)
				.. "</div>"
				.. '<div class="flex items-center gap-1">'
				.. statsBadge
				.. "</div>"
				.. "</div></a>"
		end
		mangaCards = mangaCards
			.. "</div>"
			.. pagination({
				Pagination = { Base = "/", Param = "page", Current = page, Total = totalPages },
			})
	end

	local subtitle = tostring(stats.TotalTitles or 0)
		.. " titles · "
		.. tostring(totalPages)
		.. " page"
		.. (totalPages > 1 and "s" or "")

	return '<div class="flex items-center gap-3 flex-wrap mb-6">'
		.. '<div class="shrink-0"><h1 class="text-xl font-semibold">Library</h1>'
		.. '<div class="text-xs text-neutral-500">'
		.. h(subtitle)
		.. "</div></div>"
		.. '<form method="get" action="/view/library" class="flex-1 min-w-[180px] max-w-md" role="search">'
		.. '<div class="relative">'
		.. '<svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-neutral-500 pointer-events-none" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path stroke-linecap="round" d="m21 21-4.35-4.35"/></svg>'
		.. '<input type="search" name="q" value="'
		.. h(q)
		.. '" placeholder="Search library…" class="w-full bg-neutral-900 border border-neutral-700 rounded-md pl-9 pr-3 py-1.5 text-sm placeholder-neutral-500 focus:outline-none focus:border-indigo-500">'
		.. "</div></form>"
		.. '<div class="view-mode-toggle shrink-0 flex items-center gap-1" role="group" aria-label="View mode" data-view-mode="grid">'
		.. '<button type="button" id="view-grid-btn" aria-pressed="true" class="p-1.5 rounded-md hover:bg-neutral-800 transition" title="Grid view">'
		.. '<svg class="w-4 h-4 text-neutral-400" fill="currentColor" viewBox="0 0 16 16"><rect x="1" y="1" width="6" height="6" rx="1"/><rect x="9" y="1" width="6" height="6" rx="1"/><rect x="1" y="9" width="6" height="6" rx="1"/><rect x="9" y="9" width="6" height="6" rx="1"/></svg></button>'
		.. '<button type="button" id="view-list-btn" aria-pressed="false" class="p-1.5 rounded-md hover:bg-neutral-800 transition" title="List view">'
		.. '<svg class="w-4 h-4 text-neutral-400" fill="currentColor" viewBox="0 0 16 16"><rect x="1" y="1" width="14" height="3" rx="1"/><rect x="1" y="6.5" width="14" height="3" rx="1"/><rect x="1" y="12" width="14" height="3" rx="1"/></svg></button></div>'
		.. '<form method="post" action="/action/sync">'
		.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium inline-flex items-center gap-1.5">'
		.. '<svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 0 0 4.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 0 1-15.357-2m15.357 2H15"/></svg>Update</button>'
		.. "</form></div>"
		.. sidebarHTML
		.. mangaCards
		.. (sidebarHTML ~= "" and "</div></div>" or "")
		.. [[
<style>
  .view-container[data-view-mode="list"] {
    display: flex;
    flex-direction: column;
  }
  .view-container[data-view-mode="list"] .view-item {
    flex-direction: row;
  }
  .view-container[data-view-mode="list"] .view-item > div:first-child {
    width: 80px;
    height: 110px;
    border-radius: 0.5rem 0 0 0.5rem;
    overflow: hidden;
  }
  .view-container[data-view-mode="list"] .view-item .relative {
    width: 80px;
    height: 110px;
  }
  .view-container[data-view-mode="list"] .view-item > .relative img,
  .view-container[data-view-mode="list"] .view-item > .relative > div {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .view-container[data-view-mode="list"] .view-item .lib-dim {
    border-radius: 0.5rem 0 0 0.5rem;
  }
  .view-container[data-view-mode="list"] .view-item .p-3 {
    flex: 1;
    padding: 0.75rem 1rem;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }
  .view-container[data-view-mode="list"] .view-item:hover {
    transform: none;
  }
  .view-mode-toggle button[aria-pressed="true"] svg,
  .view-mode-toggle button.active svg {
    color: #818cf8;
  }
</style>]]
end
