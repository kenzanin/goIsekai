--  partials/search_filters.lua
--  Genre checkbox grid for the search page. Returns the checkbox HTML only;
--  the view keeps the Categories wrapper so it can stay hidden for plugins
--  that advertise no genres (empty string).
--  data.PluginID, data.Genres, data.GenresSelected
return function(data)
	local pluginID = data.PluginID or ""
	local genres = data.Genres or {}
	local genresSelected = data.GenresSelected or {}

	-- Build genre checkboxes (only when the selected plugin advertises genres).
	-- Rendered as a multi-column checkbox grid below the search bar; the area is
	-- hidden entirely for plugins without genre support (genreOpts == "").
	-- Browsers submit every checked [name="genre"] as separate ?genre=<slug> params.
	local genreBoxes = ""
	if pluginID ~= "" and #genres > 0 then
		for _, g in ipairs(genres) do
			local isSel = false
			for _, sel in ipairs(genresSelected) do
				if sel == g.Slug then
					isSel = true
					break
				end
			end
			genreBoxes = genreBoxes
				.. '<div class="flex items-center gap-2">'
				.. '<input type="checkbox" id="genre_'
				.. h(g.Slug)
				.. '" name="genre" value="'
				.. h(g.Slug)
				.. '"'
				.. (isSel and " checked" or "")
				.. ' class="rounded bg-neutral-800 border-neutral-600 text-indigo-600 focus:ring-indigo-500">'
				.. '<label for="genre_'
				.. h(g.Slug)
				.. '" class="text-sm text-neutral-300 cursor-pointer">'
				.. h(g.Name)
				.. "</label></div>"
		end
	end

	return genreBoxes
end
