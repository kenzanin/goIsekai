--  partials/library_toolbar.lua
--  Everything that builds a URL or a control on the library page: the query
--  rebuilder (libraryLink), status chips, tag bar, the carried hidden inputs,
--  the view-mode toggle, the Update button and the pagination. Returns the
--  eight strings the view splices into its toolbar markup.
--  data.Q, data.PluginID, data.Sort, data.Status, data.Tag, data.View,
--  data.CategoryCounts, data.csrf_token
local pagination = require("partials.pagination")

return function(data)
	local q = data.Q or ""
	local pluginID = data.PluginID or ""
	local page = data.Page or 1
	local totalPages = data.TotalPages or 1

	local query = ""
	local sortKey = data.Sort or "updated"
	local statusFilter = data.Status or "all"
	local tagFilter = data.Tag or ""
	local viewFilter = data.View or ""

	-- Every control on this page changes one parameter and keeps the rest. That
	-- cannot be done by appending to a base URL: the base already carries the
	-- parameter being changed, so "?status=reading&status=done" comes out and
	-- the server reads the first - the link looks right and does nothing.
	-- Instead each link rebuilds the whole query from the current parameters with
	-- one key replaced, so a parameter can never appear twice.
	local function libraryLink(overrides)
		local params = {
			q = (q ~= "" and q or nil),
			pluginID = (pluginID ~= "" and pluginID or nil),
			sort = (sortKey ~= "updated" and sortKey or nil),
			status = (statusFilter ~= "all" and statusFilter or nil),
			tag = (tagFilter ~= "" and tagFilter or nil),
			view = (viewFilter ~= "" and viewFilter or nil),
		}
		for key, val in pairs(overrides or {}) do
			if val == nil or val == "" or val == "all" or val == "updated" then
				params[key] = nil
			else
				params[key] = val
			end
		end
		return link("/view/library", urlQuery(params))
	end

	local pagBase = libraryLink({})

	local statusOpts = {
		{ "all", "All" },
		{ "reading", "In progress" },
		{ "unread", "Unread" },
		{ "done", "Finished" },
		{ "new", "New" },
	}
	local statusChips = ""
	for _, o in ipairs(statusOpts) do
		local on = o[1] == statusFilter
		statusChips = statusChips
			.. '<a href="'
			.. libraryLink({ status = o[1] })
			.. '" class="inline-flex items-center gap-1 rounded-md border px-2.5 py-1 text-xs '
			.. (on and "border-indigo-500 bg-indigo-500/15 text-indigo-200" or "border-neutral-700 text-neutral-400 hover:bg-neutral-800")
			.. '">'
			.. h(o[2])
			.. "</a>"
	end

	-- Tag chips: the categories already attached to library entries, most common
	-- first. Empty when nothing is tagged, rather than an empty bar.
	local catCounts = data.CategoryCounts or {}
	local catsSorted = {}
	for name, n in pairs(catCounts) do
		catsSorted[#catsSorted + 1] = { name, n }
	end
	table.sort(catsSorted, function(a, b)
		if a[2] ~= b[2] then
			return a[2] > b[2]
		end
		return a[1] < b[1]
	end)
	local tagBar = ""
	if #catsSorted > 0 then
		tagBar = '<div class="flex items-center gap-1.5 flex-wrap mt-2">'
			.. '<span class="text-xs text-neutral-500">Tag:</span>'
		-- "All" reset chip: drops the tag filter so users can recover
		-- (when a tag is persisted, clicking a chip action saved by the
		-- JS persistence leaves no other reset). Active when no tag is set.
		tagBar = tagBar
			.. '<a href="'
			.. libraryLink({ tag = "" })
			.. '" class="inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs '
			.. (tagFilter == "" and "border-emerald-500 bg-emerald-500/15 text-emerald-200" or "border-neutral-700 text-neutral-400 hover:bg-neutral-800")
			.. '">All</a>'
		for _, c in ipairs(catsSorted) do
			local on = c[1] == tagFilter
			tagBar = tagBar
				.. '<a href="'
				.. libraryLink({ tag = c[1] })
				.. '" class="inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs '
				.. (on and "border-emerald-500 bg-emerald-500/15 text-emerald-200" or "border-neutral-700 text-neutral-400 hover:bg-neutral-800")
				.. '">'
				.. h(c[1])
				.. '<span class="text-neutral-500">'
				.. tostring(c[2])
				.. "</span></a>"
		end
		tagBar = tagBar .. "</div>"
	end

	-- The sort control submits only the parameters that are not its own, so it
	-- can never collide with a stale hidden input carrying the same key.
	-- Each form carries the parameters it does not itself submit. A hidden input
	-- for a key the form also submits would appear twice, and the server would
	-- read the first and ignore what the user just picked - so the search form
	-- carries everything except q, which its own input supplies.
	local searchCarried = ""
	searchCarried = searchCarried
		.. (pluginID ~= "" and '<input type="hidden" name="pluginID" value="' .. h(pluginID) .. '">' or "")
	searchCarried = searchCarried
		.. (sortKey ~= "updated" and '<input type="hidden" name="sort" value="' .. h(sortKey) .. '">' or "")
	searchCarried = searchCarried
		.. (statusFilter ~= "all" and '<input type="hidden" name="status" value="' .. h(statusFilter) .. '">' or "")
	searchCarried = searchCarried
		.. (tagFilter ~= "" and '<input type="hidden" name="tag" value="' .. h(tagFilter) .. '">' or "")
	searchCarried = searchCarried
		.. (viewFilter ~= "" and '<input type="hidden" name="view" value="' .. h(viewFilter) .. '">' or "")

	local sortCarried = ""
	sortCarried = sortCarried .. (q ~= "" and '<input type="hidden" name="q" value="' .. h(q) .. '">' or "")
	sortCarried = sortCarried
		.. (pluginID ~= "" and '<input type="hidden" name="pluginID" value="' .. h(pluginID) .. '">' or "")
	sortCarried = sortCarried
		.. (statusFilter ~= "all" and '<input type="hidden" name="status" value="' .. h(statusFilter) .. '">' or "")
	sortCarried = sortCarried
		.. (tagFilter ~= "" and '<input type="hidden" name="tag" value="' .. h(tagFilter) .. '">' or "")
	sortCarried = sortCarried
		.. (viewFilter ~= "" and '<input type="hidden" name="view" value="' .. h(viewFilter) .. '">' or "")

	local viewModeHTML = '<div class="view-mode-toggle flex items-center gap-1" role="group" aria-label="View mode" data-view-mode="grid">'
		.. '<button type="button" id="view-grid-btn" aria-pressed="true" class="p-1.5 rounded-md hover:bg-neutral-800 transition" title="Grid view">'
		.. '<svg class="w-4 h-4 text-neutral-400" fill="currentColor" viewBox="0 0 16 16"><rect x="1" y="1" width="6" height="6" rx="1"/><rect x="9" y="1" width="6" height="6" rx="1"/><rect x="1" y="9" width="6" height="6" rx="1"/><rect x="9" y="9" width="6" height="6" rx="1"/></svg></button>'
		.. '<button type="button" id="view-list-btn" aria-pressed="false" class="p-1.5 rounded-md hover:bg-neutral-800 transition" title="List view">'
		.. '<svg class="w-4 h-4 text-neutral-400" fill="currentColor" viewBox="0 0 16 16"><rect x="1" y="1" width="14" height="3" rx="1"/><rect x="1" y="6.5" width="14" height="3" rx="1"/><rect x="1" y="12" width="14" height="3" rx="1"/></svg></button>'
		.. '<button type="button" id="view-reset-btn" class="p-1.5 rounded-md hover:bg-neutral-800 transition" title="Reset view (grid + all filters)">'
		.. '<svg class="w-4 h-4 text-neutral-400" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/></svg></button></div>'
	local updateHTML = '<form method="post" action="/action/sync" data-sync-form data-job-action="sync">'
		.. csrfInput(data.csrf_token or "")
		.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium inline-flex items-center gap-1.5">'
		.. '<svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 0 0 4.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 0 1-15.357-2m15.357 2H15"/></svg>Update</button>'
		.. "</form>"
	local paginationOnly = pagination({
		Pagination = { Base = pagBase, Param = "page", Current = page, Total = totalPages, Compact = false },
	})
	-- 3-zone toolbar: view-mode left, pagination centered, update right. Pagination below grid stays centered on its own.
	local topToolbar = '<div class="flex items-center gap-3 mt-4 mb-4 flex-wrap"><div class="shrink-0">'
		.. viewModeHTML
		.. '</div><div class="flex-1 flex justify-center">'
		.. paginationOnly
		.. '</div><div class="shrink-0">'
		.. updateHTML
		.. "</div></div>"
	local bottomPagination = paginationOnly

	return statusChips, tagBar, sortCarried, searchCarried,
		topToolbar, bottomPagination
end
