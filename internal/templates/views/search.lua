-- views/search.lua
-- Search form and results grid with plugin selector.
-- Replaces views/search.jet.
-- Called as: search(data) -> string (body HTML only, layout wraps it)
-- data.Plugins: []database.Plugin, data.Q: string, data.PluginID: string
-- data.Results: []types.Manga, data.Page: int, data.TotalPages: int
-- data.ThumbRatio: float, data.Challenge: bool

local pagination = require("partials.pagination")

return function(data)
	local plugins = data.Plugins or {}
	local q = data.Q or ""
	local pluginID = data.PluginID or ""
	local results = data.Results or {}
	local page = data.Page or 1
	local totalPages = data.TotalPages or 1
	local challenge = data.Challenge or false

	-- Build plugin options
	local pluginOpts = ""
	for _, p in ipairs(plugins) do
		if p.IsActive then
			local selected = (pluginID == p.ID) and " selected" or ""
			pluginOpts = pluginOpts .. " <option value=" .. h(p.ID) .. selected .. ">" .. h(p.Name) .. "</option>\n"
		end
	end

	-- Get plugin name + icon for badge
	local pluginName = ""
	local pluginIcon = ""
	for _, p in ipairs(plugins) do
		if p.ID == pluginID then
			pluginName = p.Name
			pluginIcon = p.IconURL or ""
			break
		end
	end
	local pluginBadge = ""
	if pluginName ~= "" then
		pluginBadge = '<span class="inline-flex items-center gap-1.5 text-[10px] px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-500">'
		if pluginIcon ~= "" then
			pluginBadge = pluginBadge .. '<img src="' .. h(pluginIcon) .. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover">'
		end
		pluginBadge = pluginBadge .. h(pluginName) .. "</span>"
	end

	-- Build result rows
	local rows = {}
	for _, r in ipairs(results) do
		local title = r.Title or ""
		local coverURL = r.CoverURL or ""
		local mangaID = r.ID or ""

		local coverHTML
		if coverURL ~= "" then
			coverHTML = '<img src="/image?pluginID=' .. h(pluginID) .. '&amp;url=' .. h(coverURL) .. '" alt="' .. h(title) .. '" class="w-full aspect-[2/3] object-cover" loading="lazy">'
		else
			coverHTML = '<div class="w-full aspect-[2/3] bg-neutral-800 flex items-center justify-center text-neutral-500 text-2xl font-semibold">'
				.. h(getInitials(title)) .. "</div>"
		end

		rows[#rows + 1] = '<a href="/view/manga/' .. h(pluginID) .. "/" .. h(mangaID) .. '" class="flex flex-col bg-neutral-900 rounded-lg overflow-hidden hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40 hover:ring-1 hover:ring-indigo-500 transition">'
			.. '<div class="relative overflow-hidden">' .. coverHTML .. "</div>"
			.. '<div class="p-3 flex-1 flex flex-col justify-between">'
			.. '<div class="font-medium text-sm mb-1 truncate" title="' .. h(title) .. '">' .. h(title) .. "</div>"
			.. '<div class="flex items-center gap-1">' .. (pluginBadge ~= "" and '<span class="text-[10px] text-neutral-500">' .. pluginBadge .. "</span>" or "") .. "</div>"
			.. "</div></a>"
	end

	local noResults = ""
	if #results == 0 and q ~= "" then
		noResults = '<div class="py-16 text-center text-neutral-500">No results for "' .. h(q) .. '"</div>'
	end

	-- Challenge warning
	local challengeHTML = ""
	if challenge then
		challengeHTML = '<div class="bg-amber-500/15 border border-amber-500/40 text-amber-300 rounded-lg p-4 mb-6">'
			.. 'This site needs human verification — go to <a href="/view/plugins" class="underline">Plugins → Human Verification</a>'
			.. " to paste your session cookies.</div>"
	end

	-- Results grid
	local resultsHTML = ""
	if #results > 0 then
		resultsHTML = '<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">'
			.. table.concat(rows, "\n")
			.. "</div>"
	end

	-- Top pagination
	local topPagination = ""
	if #results > 0 then
		topPagination = pagination({
			Pagination = {
				Base = "/view/search",
				Param = "page",
				Current = page,
				Total = totalPages,
				Extra = { "q", q, "pluginID", pluginID },
				Compact = true,
			},
		})
	end

	-- Bottom pagination
	local bottomPagination = ""
	if #results > 0 then
		bottomPagination = pagination({
			Pagination = {
				Base = "/view/search",
				Param = "page",
				Current = page,
				Total = totalPages,
				Extra = { "q", q, "pluginID", pluginID },
				Compact = false,
			},
		})
	end

	return [[<h1 class="text-xl font-semibold mb-6">Search]]
		.. pluginBadge
		.. [[</h1>]]
		.. '<form method="get" action="/view/search" class="flex gap-2 items-end mb-8">'
		.. '<div>'
		.. '<label for="pluginID" class="block text-xs text-neutral-400 mb-1">Plugin</label>'
		.. '<select id="pluginID" name="pluginID" class="bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">'
		.. pluginOpts
		.. '</select>'
		.. '</div>'
		.. '<div class="flex-1 max-w-md">'
		.. '<label for="q" class="block text-xs text-neutral-400 mb-1">Query</label>'
		.. '<input id="q" name="q" type="text" value="'
		.. h(q)
		.. '" placeholder="Manga title..." class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">'
		.. '</div>'
		.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Search</button>'
		.. '</form>'
		.. (topPagination ~= "" and topPagination or "")
		.. resultsHTML
		.. "\n"
		.. noResults
		.. "\n"
		.. challengeHTML
		.. "\n"
		.. bottomPagination
end
