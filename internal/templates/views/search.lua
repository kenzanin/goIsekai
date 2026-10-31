-- views/search.lua
-- Search form and results grid with plugin selector.
-- Replaces views/search.jet.
-- Called as: search(data) -> string (body HTML only, layout wraps it)
-- data.Plugins: []database.Plugin, data.Q: string, data.PluginID: string
-- data.Results: []types.Manga, data.Page: int, data.TotalPages: int
-- data.ThumbRatio: float, data.Challenge: bool

local pagination = require("partials.pagination")
local verifyWizard = require("partials.verify_wizard")
local searchResults = require("partials.search_results")
local searchFilters = require("partials.search_filters")
local searchPicker = require("partials.search_picker")
local searchScripts = require("partials.search_scripts")

return function(data)
	local plugins = data.Plugins or {}
	local q = data.Q or ""
	local pluginID = data.PluginID or ""
	local results = data.Results or {}
	local page = data.Page or 1
	local totalPages = data.TotalPages or 1
	local challenge = data.Challenge or false
	local genres = data.Genres or {}
	local genre = data.Genre or ""
	-- GenresSelected is every genre the request asked for (genre is repeatable:
	-- ?genre=a&genre=b). `genre` is just the first one, kept for display.
	local genresSelected = data.GenresSelected or {}
	-- Cache-buster per cover URL, same contract the detail page uses: /image is
	-- cached for a week under an unversioned URL, so the grid must vary it.
	local coverVersions = data.CoverVersions or {}

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
		pluginBadge =
			'<span class="inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-500">'
		if pluginIcon ~= "" then
			pluginBadge = pluginBadge
				.. '<img src="'
				.. h(pluginIcon)
				.. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover">'
		end
		pluginBadge = pluginBadge .. h(pluginName) .. "</span>"
	end

	local genreBoxes = searchFilters({
		PluginID = pluginID,
		Genres = genres,
		GenresSelected = genresSelected,
	})

	local rows, noResults = searchResults({
		PluginID = pluginID,
		Q = q,
		Genre = genre,
		PluginName = pluginName,
		PluginBadge = pluginBadge,
		Results = results,
		CoverVersions = coverVersions,
	})

	-- Challenge warning
	local challengeHTML = ""
	if challenge then
		challengeHTML = '<div class="bg-amber-500/15 border border-amber-500/40 text-amber-300 rounded-lg p-4 mb-6">'
			.. 'This site needs human verification — go to <a href="/view/plugins" class="underline">Plugins → Human Verification</a>'
			.. " to paste your session cookies.</div>"
	end

	-- Human verification wizard modal (shared partial; renders "" when hidden).
	-- MangaID "" = search context: save keeps the query string.
	local verifyHTML = verifyWizard({
		csrf_token = data.csrf_token or "",
		PluginID = pluginID,
		MangaID = "",
		NeedsHumanVerify = data.NeedsHumanVerify or false,
		VerifyURL = data.VerifyURL or "",
		VerifyCookies = data.VerifyCookies or "",
		VerifyUserAgent = data.VerifyUserAgent or "",
		Challenge = challenge,
	})

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
				Extra = { "q", q, "pluginID", pluginID, "genre", genresSelected },
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
				Extra = { "q", q, "pluginID", pluginID, "genre", genresSelected },
				Compact = false,
			},
		})
	end

	local pickerBtn, pickerDrop, selName = searchPicker({
		Plugins = plugins,
		PluginID = pluginID,
	})
	local pickerScript, historyScript = searchScripts({})
	return [[<h1 class="text-xl font-semibold mb-6">Search]]
		.. pluginBadge
		.. [[</h1>]]
		.. '<form method="get" action="/view/search" id="searchForm" class="mb-6">'
		.. '<div class="flex gap-2 items-end">'
		.. '<div class="relative"><label class="block text-xs text-neutral-400 mb-1">Plugin</label>'
		.. pickerBtn
		.. pickerDrop
		.. '<input type="hidden" id="pluginID" name="pluginID" value="'
		.. h(pluginID)
		.. '" data-pname="'
		.. h(selName)
		.. '"><select id="pluginIDNative" class="hidden">'
		.. pluginOpts
		.. "</select></div>"
		.. '<div class="flex-1"><label for="q" class="block text-xs text-neutral-400 mb-1">&nbsp;</label><input id="q" name="q" type="text" value="'
		.. h(q)
		.. '" placeholder="Manga title..." class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm"></div>'
		.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Search</button>'
		.. "</div>"
		-- Genre/tag selector area: only when the selected plugin advertises genres.
		-- Full-width multi-select below the search bar so category browsing has its
		-- own region, and is hidden entirely for plugins without genre support.
		.. (genreBoxes ~= "" and ('<div class="mt-4"><label class="block text-xs text-neutral-400 mb-1">Categories</label><div class="max-h-40 overflow-y-auto grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-x-4 gap-y-2 bg-neutral-900 border border-neutral-700 rounded-md px-3 py-3">' .. genreBoxes .. "</div></div>") or "")
		.. (topPagination ~= "" and ('<div class="mt-3">' .. topPagination .. "</div>") or "")
		.. "</form>"
		.. '<div id="search-history" class="mb-8"></div>'
		.. pickerScript
		.. resultsHTML
		.. "\n"
		.. noResults
		.. "\n"
		.. challengeHTML
		.. "\n"
		.. verifyHTML
		.. "\n"
		.. bottomPagination
		.. historyScript
end
