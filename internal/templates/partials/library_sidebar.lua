-- partials/library_sidebar.lua
-- Library page: stats cards, duplicate groups, per-plugin counts.
-- Returns the sidebar markup and the content-column opener, or "" when hidden.
-- Called as: library_sidebar(data) -> string
-- data.Mangas, data.Q, data.Stats, data.DuplicateCount, data.DuplicateGroups, data.PluginCounts

local libraryPluginCounts = require("partials.library_plugin_counts")

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
		-- The duplicate view is sticky; keep the sidebar (and with it the card
		-- that clears the view) reachable even when the filter matches nothing.
		or viewFilter == "duplicate"
	-- Always show plugin card when we have pluginCounts; otherwise keep previous stats-gating.
	-- Override: plugin card must stay visible so user can hit All after any filter.
	if #pluginCounts > 0 then
		showSidebar = true
	end
	local sidebarHTML = ""
	if showSidebar then
		-- Build stats cards
		local statCards = ""
		-- reset renders the card as a link to a bare /view/library that drops the
		-- persisted filters and view mode too — the same action as the toolbar's
		-- reset-view button (window.resetLibraryView). Used by the "titles" card.
		local function addStat(value, label, color, longText, accent, reset)
			local cls = (color and color ~= "") and (' class="text-sm font-medium ' .. h(color) .. '"')
				or (
					longText and ' class="text-sm font-medium"'
					or (accent and ' class="text-3xl font-bold text-indigo-300"' or ' class="text-lg font-semibold"')
				)
			local outer = accent and "bg-neutral-900 border border-indigo-500/30 rounded-lg px-4 py-3"
				or "bg-neutral-900 border border-neutral-800 rounded-lg px-4 py-3"
			local tag = "div"
			local attrs = ""
			if reset then
				tag = "a"
				attrs = ' href="/view/library" onclick="window.resetLibraryView();return false" title="Reset view (grid + all filters)"'
				outer = outer .. " hover:border-indigo-500 transition"
			end
			statCards = statCards
				.. "<"
				.. tag
				.. attrs
				.. ' class="'
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
				.. "</"
				.. tag
				.. ">"
		end

		-- addStatLink renders the same card shape but wraps the value+label in an
		-- <a> so the card is clickable. Used for the "updates" card which filters
		-- to ?status=new.
		local function addStatLink(value, label, color, href, isActive, clearView)
			local active = isActive
			if active == nil then
				active = (statusFilter == "new")
			end
			local onclick = ""
			if clearView then
				onclick = ' onclick="try{localStorage.removeItem(\'gsk:library-view\')}catch(e){}"'
			end
			local cls = (color and color ~= "") and (' class="text-sm font-medium ' .. h(color) .. '"')
				or ' class="text-lg font-semibold"'
			local outer = active
				and "bg-indigo-500/15 border border-indigo-500 rounded-lg px-4 py-3"
				or "bg-neutral-900 border border-neutral-800 rounded-lg px-4 py-3 hover:border-indigo-500/30 transition"
			statCards = statCards
				.. '<a href="'
				.. h(href)
				.. '"'
				.. onclick
				.. ' class="'
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
			addStat(stats.TotalTitles, "titles", "", false, true, true)
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
			local updatesHref = activeNew and sidebarLink({ status = "all" }) or sidebarLink({ status = "new" })
			addStatLink(stats.HasUpdates, "updates", "text-indigo-400", updatesHref, activeNew, activeNew)
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

		-- Duplicate card: plain clickable card, no dropdown. Click toggles the
	-- ?view=duplicate grid filter; the card only shows the count.
	-- Also rendered when the filter is active but the count has dropped to 0:
	-- the view is sticky (localStorage gsk:library-view), so hiding the card
	-- would strand the reader on an empty grid with no way back to all titles.
	-- MUST run before the statCards embed below: addStatLink appends to
	-- statCards, which is concatenated into sidebarHTML at the embed.
	if duplicateCount > 0 or viewFilter == "duplicate" then
		local activeDup = (viewFilter == "duplicate")
		local dupHref = activeDup and sidebarLink({ view = "" }) or sidebarLink({ view = "duplicate" })
		addStatLink(duplicateCount, "duplicates", "text-indigo-400", dupHref, activeDup, activeDup)
	end

	sidebarHTML = [[<div class="flex flex-row gap-4 items-start">
<aside class="w-56 shrink-0 flex flex-col gap-3">
    <div class="bg-neutral-900 border border-neutral-800 rounded-lg px-4 py-3">
        <div class="flex flex-col gap-3">]] .. statCards .. [[</div>
    </div>

]]

		local pluginCardHTML = libraryPluginCounts({
			PluginCounts = pluginCounts,
			PluginID = data.PluginID,
		})
		if pluginCardHTML ~= "" then
			sidebarHTML = sidebarHTML .. pluginCardHTML
		end

		sidebarHTML = sidebarHTML .. [[</aside>
    <div class="flex-1 min-w-0">]]
	end

	return sidebarHTML
end
