-- views/search.lua
-- Search form and results grid. Replaces views/search.jet.
-- Called as: search(data) -> string (body HTML only, layout wraps it)

local pagination = require("partials.pagination")

return function(data)
	local q = data.Q or ""
	local pluginID = data.PluginID or ""
	local pluginName = data.PluginName or ""
	local pluginIcon = data.PluginIcon or ""
	local results = data.Results or {}
	local page = data.Page or 1
	local totalPages = data.TotalPages or 1
	local challenge = data.Challenge or false
	local plugins = data.Plugins or {}

	-- Build plugin options
	local pluginOpts = ""
	for _, p in ipairs(plugins) do
		if p.IsActive then
			local selected = (pluginID == p.ID) and " selected" or ""
			pluginOpts = pluginOpts .. "            <option value=" .. h(p.ID) .. selected .. ">" .. h(p.Name) .. "</option>\n"
		end
	end

	-- Build result rows
	local rows = {}
	for _, r in ipairs(results) do
		local title = r.Title or ""
		local coverURL = r.CoverURL or ""
		local mangaID = r.ID or ""
		local coverHTML
		if coverURL ~= "" then
			coverHTML = '<img src="/image?pluginID=' .. h(pluginID) .. '&amp;url=' .. h(coverURL)
				.. '" alt="' .. h(title) .. '" class="w-full aspect-[2/3] object-cover" loading="lazy">'
		else
			coverHTML = '<div class="w-full aspect-[2/3] bg-neutral-800 flex items-center justify-center text-neutral-500 text-2xl font-semibold">'
				.. h(getInitials(title)) .. '</div>'
		end

		local pluginBadge = ""
		if pluginName ~= "" then
			pluginBadge = '<span class="inline-flex items-center gap-1.5 text-[10px] px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-500">'
			if pluginIcon ~= "" then
				pluginBadge = pluginBadge .. '<img src="' .. h(pluginIcon) .. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover">'
			end
			pluginBadge = pluginBadge .. h(pluginName) .. '</span>'
		end

		rows[#rows + 1] = '<a href="/view/manga/' .. h(pluginID) .. '/' .. h(mangaID)
			.. '" class="bg-neutral-900 rounded-lg overflow-hidden hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40 hover:ring-1 hover:ring-indigo-500 transition">'
			.. '<div class="p-3">'
			.. '<div class="text-sm font-medium line-clamp-2" title="' .. h(title) .. '">' .. h(title) .. '</div>'
			.. pluginBadge .. '</div></a>'
	end

	-- Plugin badge on header
	local pluginBadge = ""
	if pluginIcon ~= "" then
		pluginBadge = pluginBadge .. '<img src="' .. h(pluginIcon) .. '" alt="" class="inline h-3.5 w-3.5 rounded-sm object-cover align-[-1px]">'
	end
	if pluginName ~= "" then
		pluginBadge = pluginBadge .. ' <span class="text-neutral-500 font-normal">· ' .. h(pluginName) .. '</span>'
	end

	-- Challenge warning
	local challengeHTML = ""
	if challenge then
		challengeHTML = '<div class="bg-amber-500/15 border border-amber-500/40 text-amber-300 rounded-lg p-4 mb-6">'
			.. 'This site needs human verification — go to <a href="/view/plugins" class="underline">Plugins → Human Verification</a>'
			.. ' to paste your session cookies.</div>'
	end

	-- No results
	local noResults = ""
	if #results == 0 and q ~= "" then
		noResults = '<div class="py-16 text-center text-neutral-500">No results for "' .. h(q) .. '"</div>'
	end

	-- Results grid + pagination
	local resultsHTML = ""
	if #results > 0 then
		resultsHTML = '<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">'
			.. table.concat(rows, "\n") .. '</div>'
		if page > 1 or totalPages > 1 then
			resultsHTML = resultsHTML .. pagination({
				Pagination = {
					Base = "/view/search",
					Param = "page",
					Current = page,
					Total = totalPages,
					Extra = { "q", q, "pluginID", pluginID },
				},
			})
		end
	end

	return [[<h1 class="text-xl font-semibold mb-6">Search]] .. pluginBadge .. [[</h1>

<form method="get" action="/view/search" class="flex gap-2 items-end mb-8">
    <div>
        <label for="pluginID" class="block text-xs text-neutral-400 mb-1">Plugin</label>
        <select id="pluginID" name="pluginID" class="bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">
]] .. pluginOpts .. [[        </select>
    </div>
    <div class="flex-1 max-w-md">
        <label for="q" class="block text-xs text-neutral-400 mb-1">Query</label>
        <input id="q" name="q" type="text" value="]] .. h(q) .. [[" placeholder="Manga title..." class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">
    </div>
    <button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Search</button>
</form>

]] .. resultsHTML
		.. "\n" .. noResults
		.. "\n" .. challengeHTML
end
