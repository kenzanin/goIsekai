--  partials/search_results.lua
--  Result rows for the search grid: one <a> card per manga, plus the
--  "No results" notice. Returned as two strings so the view keeps owning
--  the grid wrapper and pagination placement.
--  data.PluginID, data.Q, data.Genre, data.PluginName, data.PluginBadge,
--  data.Results, data.CoverVersions
return function(data)
	local pluginID = data.PluginID or ""
	local q = data.Q or ""
	local genre = data.Genre or ""
	local pluginName = data.PluginName or ""
	local pluginBadge = data.PluginBadge or ""
	local results = data.Results or {}
	local coverVersions = data.CoverVersions or {}

	-- Build result rows
	local rows = {}
	for _, r in ipairs(results) do
		local title = r.Title or ""
		local coverURL = r.CoverURL or ""
		local mangaID = r.ID or ""

		local coverHTML
		if coverURL ~= "" then
			coverHTML = '<img src="/image?pluginID='
				.. h(pluginID)
				.. "&amp;url="
				.. h(coverURL)
				.. (coverVersions[coverURL] and coverVersions[coverURL] ~= "" and ("&amp;cb=" .. h(coverVersions[coverURL])) or "")
				.. '" alt="'
				.. h(title)
				.. '" class="w-full aspect-[2/3] object-cover" loading="lazy">'
		else
			coverHTML = '<div class="w-full aspect-[2/3] bg-neutral-800 flex items-center justify-center text-neutral-500 text-2xl font-semibold">'
				.. h(getInitials(title))
				.. "</div>"
		end

		rows[#rows + 1] = '<a href="/view/manga/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. '" class="flex flex-col bg-neutral-900 rounded-lg overflow-hidden hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40 hover:ring-1 hover:ring-indigo-500 transition">'
			.. '<div class="relative overflow-hidden">'
			.. coverHTML
			.. "</div>"
			.. '<div class="p-3 flex-1 flex flex-col justify-between">'
			.. '<div class="font-medium text-sm mb-1 truncate" title="'
			.. h(title)
			.. '">'
			.. h(title)
			.. "</div>"
			.. '<div class="flex items-center gap-1">'
			.. (pluginBadge ~= "" and '<span class="text-xs text-neutral-500">' .. pluginBadge .. "</span>" or "")
			.. "</div>"
			.. "</div></a>"
	end

	local noResults = ""
	if #results == 0 and (q ~= "" or genre ~= "") then
		noResults = '<div class="py-16 text-center text-neutral-500">No results'
		if q ~= "" then
			noResults = noResults .. ' for "' .. h(q) .. '"'
		end
		if pluginName ~= "" then
			noResults = noResults .. " on " .. h(pluginName)
		end
		noResults = noResults .. "</div>"
	end

	return rows, noResults
end
