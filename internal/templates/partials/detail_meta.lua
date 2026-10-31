--  partials/detail_meta.lua
--  Genre tags, related-manga tags and the collapsible synopsis for the manga
--  detail page. Returns the three HTML strings the view splices into the info
--  column, so the view keeps owning their placement.
--  data.PluginID, data.MangaID, data.Manga, data.Related,
--  data.OverrideGenres, data.csrf_token
return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local manga = data.Manga or {}
	local rels = data.Related or {}

	-- Genre tags — toggle via Alpine (click adds/removes from user override)
	local overrideGenres = data.OverrideGenres or {}
	local maxVisible = 6
	local genres = manga.Genres or {}
	local genreCount = #genres
	local genreTagsHTML = ""
	for i, g in ipairs(genres) do
		if not g or g == "" then
			goto next
		end
		local isInOverride = false
		for _, og in ipairs(overrideGenres) do
			if og == g then
				isInOverride = true
				break
			end
		end
		local hidden = i > maxVisible and ' style="display:none"' or ""
		genreTagsHTML = genreTagsHTML
			.. '<form method="post" action="/action/'
			.. (isInOverride and "remove" or "add")
			.. "-genre/"
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. '" class="inline genre-tag"'
			.. hidden
			.. ">"
			.. csrfInput(data.csrf_token or "")
			.. '<input type="hidden" name="genre" value="'
			.. h(g)
			.. '">'
			.. '<span class="inline-flex items-center gap-1 rounded-full bg-indigo-500/15 text-indigo-400 text-xs pl-2 pr-1 cursor-pointer hover:bg-indigo-500/25" onclick="submitForm(this.closest(\'form\'))">'
			.. h(g)
			.. (isInOverride and '<span class="size-4 inline-flex items-center justify-center rounded-full text-indigo-500 hover:text-red-400 hover:bg-neutral-700" onclick="event.preventDefault();submitForm(this.closest(\'form\'))">&times;</span>' or "")
			.. "</span></form>"
		::next::
	end

	-- Show more toggle
	local showMoreHTML = ""
	if genreCount > maxVisible then
		showMoreHTML = '<button type="button" id="genre-showmore" onclick="toggleGenreTags(\'genre-showmore\','
			.. maxVisible
			.. ')" class="text-xs text-indigo-400 hover:text-indigo-300 ml-1 cursor-pointer" data-show="0">Show more</button>'
	end
	genreTagsHTML = genreTagsHTML .. showMoreHTML

	-- Related manga tags — click to remove (no confirm, capture-phase dispatch)
	local relatedTagsHTML = ""
	local seenRelated = {}
	for _, r in ipairs(rels) do
		local name = r.Value or ""
		if name == "" or seenRelated[name] then
			goto next
		end
		seenRelated[name] = true
		relatedTagsHTML = relatedTagsHTML
			.. '<form method="post" action="/action/remove-related/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. '" class="inline">'
			.. csrfInput(data.csrf_token or "")
			.. '<input type="hidden" name="title" value="'
			.. h(name)
			.. '">'
			.. '<span class="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 text-emerald-400 text-xs pl-2 pr-1 cursor-pointer hover:bg-emerald-500/20">'
			.. (r.URL and r.URL ~= "" and ('<a href="' .. h(r.URL) .. '" target="_blank" class="hover:underline">' .. h(
				name
			) .. "</a>") or h(name))
			.. '<span class="size-4 inline-flex items-center justify-center rounded-full text-emerald-500 hover:text-red-400 hover:bg-neutral-700" onclick="submitForm(this.closest(\'form\'))">&times;</span>'
			.. "</span></form>"
		::next::
	end

	-- Synopsis with read-more toggle (only when > 10 lines)
	local synopsisHTML = ""
	if manga.Description and manga.Description ~= "" then
		-- Count lines robustly: split on newlines AND <br> tags
		local lineCount = 1
		local pos = 1
		while true do
			local nl = string.find(manga.Description, "\n", pos, true)
			local br = string.find(manga.Description, "<br", pos, true)
			if not nl and not br then
				break
			end
			if nl and (not br or nl < br) then
				lineCount = lineCount + 1
				pos = nl + 1
			else
				lineCount = lineCount + 1
				pos = br + 3
			end
		end
		local needsToggle = lineCount > 10
		local cls = "text-sm text-neutral-400 mt-1.5"
		if needsToggle then
			cls = cls .. " max-h-[4.5rem] overflow-hidden transition-all duration-300 relative"
		end
		synopsisHTML = '<div class="mb-3">'
			.. '<span class="text-xs font-semibold text-neutral-300 uppercase tracking-wide">Synopsis</span>'
			.. '<div id="synopsis-text" class="'
			.. cls
			.. '">'
			.. h(manga.Description)
			.. "</div>"
		if needsToggle then
			synopsisHTML = synopsisHTML
				.. '<span id="synopsis-fade" class="absolute inset-x-0 bottom-0 h-8 bg-gradient-to-t from-neutral-950 to-transparent pointer-events-none"></span>'
				.. "<button type=\"button\" id=\"synopsis-toggle\" onclick=\"var t=document.getElementById('synopsis-text');var f=document.getElementById('synopsis-fade');var b=document.getElementById('synopsis-toggle');t.classList.toggle('max-h-[4.5rem]');t.classList.toggle('max-h-none');f.style.display=t.classList.contains('max-h-none')?'none':'block';b.textContent=t.classList.contains('max-h-none')?'Show less':'Read more';\" class=\"text-xs text-indigo-400 hover:text-indigo-300 transition mt-1 cursor-pointer\">Read more</button>"
		end
		synopsisHTML = synopsisHTML .. "</div>"
	end

	return genreTagsHTML, relatedTagsHTML, synopsisHTML
end
