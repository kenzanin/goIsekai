-- partials/library_sidebar.lua
-- Library page: stats cards, duplicate groups, per-plugin counts.
-- Returns the sidebar markup and the content-column opener, or "" when hidden.
-- Called as: library_sidebar(data) -> string
-- data.Mangas, data.Q, data.Stats, data.DuplicateCount, data.DuplicateGroups, data.PluginCounts

return function(data)
	local mangas = data.Mangas or {}
	local q = data.Q or ""
	local stats = data.Stats or {}
	local duplicateCount = data.DuplicateCount or 0
	local duplicateGroups = data.DuplicateGroups or {}
	local pluginCounts = data.PluginCounts or {}

	local sidebarHTML = ""
	if #mangas > 0 and q == "" then
		-- Build stats cards
		local statCards = ""
		local function addStat(value, label, color, longText, accent)
			local cls = color and (' class="text-sm font-medium ' .. h(color) .. '"')
				or (
					longText and ' class="text-sm font-medium"'
					or (accent and ' class="text-2xl font-bold"' or ' class="text-lg font-semibold"')
				)
			local outer = accent and "bg-neutral-900 border border-indigo-500/30 rounded-lg px-4 py-3"
				or "bg-neutral-900 border border-neutral-800 rounded-lg px-4 py-3"
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

	return sidebarHTML
end
