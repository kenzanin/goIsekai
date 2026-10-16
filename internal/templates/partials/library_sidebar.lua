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
	local statusFilter = data.Status or "all"
	local viewFilter = data.View or ""
	local tagFilter = data.Tag or ""

	-- Build a sidebar link that preserves the current q/pluginID/sort/status/tag/view
	-- params and overrides one or more of them, so a param can never appear twice
	-- (same rebuild-the-query pattern as libraryLink in views/library.lua).
	local function sidebarLink(overrides)
		local params = {
			q = (q ~= "" and q or nil),
			pluginID = ((data.PluginID or "") ~= "" and data.PluginID or nil),
			status = (statusFilter ~= "all" and statusFilter or nil),
			tag = (tagFilter ~= "" and tagFilter or nil),
			view = (viewFilter ~= "" and viewFilter or nil),
		}
		for key, val in pairs(overrides or {}) do
			if val == nil or val == "" or val == "all" then
				params[key] = nil
			else
				params[key] = val
			end
		end
		return link("/view/library", urlQuery(params))
	end

	local showSidebar = (#pluginCounts > 0)
		or (#mangas > 0 and q == "")
		or (q ~= "" or (data.PluginID or "") ~= "" or (data.Tag or "") ~= "" or (data.Status or "all") ~= "all")
	-- Always show plugin card when we have pluginCounts; otherwise keep previous stats-gating.
	-- Override: plugin card must stay visible so user can hit All after any filter.
	if #pluginCounts > 0 then
		showSidebar = true
	end
	local sidebarHTML = ""
	if showSidebar then
		-- Build stats cards
		local statCards = ""
		local function addStat(value, label, color, longText, accent)
			local cls = (color and color ~= "") and (' class="text-sm font-medium ' .. h(color) .. '"')
				or (
					longText and ' class="text-sm font-medium"'
					or (accent and ' class="text-3xl font-bold text-indigo-300"' or ' class="text-lg font-semibold"')
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

		-- addStatLink renders the same card shape but wraps the value+label in an
		-- <a> so the card is clickable. Used for the "updates" card which filters
		-- to ?status=new.
		local function addStatLink(value, label, color, href)
			local active = (statusFilter == "new")
			local cls = (color and color ~= "") and (' class="text-sm font-medium ' .. h(color) .. '"')
				or ' class="text-lg font-semibold"'
			local outer = active
				and "bg-neutral-900 border border-indigo-500/30 rounded-lg px-4 py-3"
				or "bg-neutral-900 border border-neutral-800 rounded-lg px-4 py-3 hover:border-indigo-500/30 transition"
			statCards = statCards
				.. '<a href="'
				.. h(href)
				.. '" class="'
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
				.. "</a>"
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
			-- Clickable: filters the grid to only titles with the [New] badge.
			local activeNew = (statusFilter == "new")
			local updatesHref = activeNew and sidebarLink({}) or sidebarLink({ status = "new" })
			addStatLink(stats.HasUpdates, "updates", "text-indigo-400", updatesHref)
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
		-- When view=duplicate is active the grid already shows every duplicate
		-- title, so the card collapses to a count-only display (no thumbnails).
		local showThumbnails = (viewFilter ~= "duplicate")
		local dupItems = ""
		if duplicateCount == 0 then
			dupItems = '<div class="text-xs text-neutral-500">Tidak ada judul yang berpotensi duplikat.</div>'
		elseif not showThumbnails then
			-- Count-only: just the group titles, no per-member thumbnails.
			for _, g in ipairs(duplicateGroups) do
				dupItems = dupItems
					.. '<div class="flex items-center justify-between gap-2">'
					.. '<span class="text-xs font-medium text-neutral-300 line-clamp-2" title="'
					.. h(g.Title or "")
					.. '">'
					.. h(g.Title or "")
					.. "</span>"
					.. '<span class="text-xs text-indigo-400 font-semibold shrink-0">'
					.. h(tostring(#(g.Members or {})))
					.. "</span>"
					.. "</div>"
			end
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
						cover = '<div class="w-8 h-11 rounded bg-neutral-800 flex items-center justify-center text-xs text-neutral-500 font-semibold shrink-0">'
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

]]
	-- Duplicate card: the <details> summary is clickable. Clicking it toggles the
	-- ?view=duplicate grid filter; when the filter is active the card also drops
	-- the per-member thumbnails and shows just the per-group count.
		if viewFilter == "duplicate" then
			-- Active: summary links back to the unfiltered library (clears view).
			sidebarHTML = sidebarHTML .. [[
    <details class="bg-neutral-900 border border-indigo-500 rounded-lg overflow-hidden open">
        <summary class="px-4 py-3 cursor-pointer select-none text-xs font-medium text-neutral-200 hover:bg-neutral-800/60 transition list-none flex items-center justify-between gap-2">
]]
			.. '<a href="' .. h(sidebarLink({ view = "" })) .. '" onclick="try{localStorage.removeItem(\'gsk:library-view\')}catch(e){}" class="w-full flex items-center justify-between gap-2">'
			.. '<span>Duplicate <span class="text-indigo-400 font-semibold">' .. h(tostring(duplicateCount)) .. '</span></span>'
			.. '<span class="text-xs text-indigo-400">active ✓</span>'
			.. "</a>"
			.. [[
        </summary>
        <div class="border-t border-neutral-800 px-3 py-3 flex flex-col gap-2">]]
			.. dupItems
			.. [[
        </div>
    </details>]]
		else
			sidebarHTML = sidebarHTML .. [[
    <details class="bg-neutral-900 border border-neutral-800 rounded-lg overflow-hidden]] .. (duplicateCount == 0 and " open" or "") .. [[" >
        <summary class="px-4 py-3 cursor-pointer select-none text-xs font-medium text-neutral-200 hover:bg-neutral-800/60 transition list-none flex items-center justify-between gap-2">
]]
			.. '<a href="' .. h(sidebarLink({ view = "duplicate" })) .. '" class="w-full flex items-center justify-between gap-2">'
			.. '<span>Duplicate <span class="text-indigo-400 font-semibold">' .. h(tostring(duplicateCount)) .. '</span></span>'
			.. '<svg class="w-4 h-4 text-neutral-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path></svg>'
			.. "</a>"
			.. [[
        </summary>
        <div class="border-t border-neutral-800 px-3 py-3 flex flex-col gap-4">]]
			.. dupItems
			.. [[
        </div>
    </details>]]
		end

		-- Plugin counts — always visible so All can reset even on empty filter results
		if #pluginCounts > 0 then
			local curPluginID = data.PluginID or ""
			local pluginRows = '<a href="/view/library" onclick="try{localStorage.removeItem(\'gsk:library-view\')}catch(e){}" class="flex items-center justify-between gap-2 text-sm rounded-md px-1.5 py-1 mb-1 border border-dashed '
				.. (curPluginID == "" and "border-indigo-500 bg-indigo-500/15" or "border-neutral-700")
				.. '"><span class="text-sm font-medium '
				.. (curPluginID == "" and "text-indigo-200" or "text-neutral-300")
				.. '">All</span><span class="text-xs text-neutral-500">all manga</span></a>'
			for _, p in ipairs(pluginCounts) do
				local active = (curPluginID ~= "" and p.PluginID == curPluginID)
				local iconHTML = ""
				if p.Icon and p.Icon ~= "" then
					iconHTML = '<img src="'
						.. h(p.Icon)
						.. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover shrink-0">'
				end
				pluginRows = pluginRows
					.. '<a href="/view/library?pluginID='
					.. h(p.PluginID or "")
					.. '" class="flex items-center justify-between gap-2 text-sm rounded-md px-1.5 py-1 '
					.. (active and "border border-indigo-500 bg-indigo-500/15" or "border border-transparent")
					.. '">'
					.. '<span class="flex items-center gap-1.5 min-w-0">'
					.. iconHTML
					.. '<span class="text-sm font-medium truncate '
					.. (active and "text-indigo-200" or "text-neutral-200")
					.. '">'
					.. h(p.Name or "")
					.. "</span>"
					.. "</span>"
					.. '<span class="text-sm font-bold shrink-0 '
					.. (active and "text-indigo-300" or "text-indigo-400")
					.. '">'
					.. h(tostring(p.Count or 0))
					.. "</span></a>"
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
