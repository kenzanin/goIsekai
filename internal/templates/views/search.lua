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
	local genres = data.Genres or {}
	local genre = data.Genre or ""

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
			'<span class="inline-flex items-center gap-1.5 text-[10px] px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-500">'
		if pluginIcon ~= "" then
			pluginBadge = pluginBadge
				.. '<img src="'
				.. h(pluginIcon)
				.. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover">'
		end
		pluginBadge = pluginBadge .. h(pluginName) .. "</span>"
	end

	-- Build genre options (only when the selected plugin advertises genres)
	local genreOpts = ""
	if pluginID ~= "" and #genres > 0 then
		genreOpts = ' <option value="">All genres</option>\n'
		for _, g in ipairs(genres) do
			local selected = (genre == g.Slug) and " selected" or ""
			genreOpts = genreOpts
				.. ' <option value="'
				.. h(g.Slug)
				.. '"'
				.. selected
				.. ">"
				.. h(g.Name)
				.. "</option>\n"
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
			coverHTML = '<img src="/image?pluginID='
				.. h(pluginID)
				.. "&amp;url="
				.. h(coverURL)
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
			.. (pluginBadge ~= "" and '<span class="text-[10px] text-neutral-500">' .. pluginBadge .. "</span>" or "")
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

	-- Challenge warning
	local challengeHTML = ""
	if challenge then
		challengeHTML = '<div class="bg-amber-500/15 border border-amber-500/40 text-amber-300 rounded-lg p-4 mb-6">'
			.. 'This site needs human verification — go to <a href="/view/plugins" class="underline">Plugins → Human Verification</a>'
			.. " to paste your session cookies.</div>"
	end

	-- Human verification wizard (auto-open modal, mirrors detail.lua pattern)
	local verifyHTML = ""
	local needsHumanVerify = data.NeedsHumanVerify or false
	local verifyURL = data.VerifyURL or ""
	local verifyCookies = data.VerifyCookies or ""
	local verifyUserAgent = data.VerifyUserAgent or ""
	-- Show the wizard only when the plugin is actually blocked: either the
	-- search came back/stayed blocked (challenge) or no cookies are saved yet.
	-- Saved cookies suppress the modal so the search can run with them.
	if needsHumanVerify and verifyURL ~= "" and (challenge or verifyCookies == "") then
		local placeholder = verifyCookies ~= "" and verifyCookies or 'cf_clearance=...; session=...'
		local useragent = verifyUserAgent ~= "" and verifyUserAgent or "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
		verifyHTML = '<div id="verify-modal" class="fixed inset-0 z-50 flex items-center justify-center">'
			.. '<div class="absolute inset-0 bg-black/70" onclick="this.parentElement.classList.add(\'hidden\')"></div>'
			.. '<div class="relative bg-neutral-900 border border-neutral-700 rounded-xl p-4 max-w-lg max-h-[80vh] overflow-y-auto">'
			.. '<div class="space-y-4">'
			.. '<div class="bg-amber-500/10 border border-amber-600/50 rounded-lg p-3">'
			.. '<h3 class="text-sm font-semibold text-amber-200 mb-2">⚠️ Plugin blocked by bot verification</h3>'
			.. '<p class="text-xs text-amber-200/80 mb-2">This site requires manual verification. Open the link below in your browser, solve the challenge, then paste cookies below — either as name=value pairs or a Cookie-Editor JSON export.</p>'
			.. '<a href="' .. h(verifyURL) .. '" target="_blank" rel="noopener" class="text-xs text-amber-400 hover:text-amber-300 hover:underline break-all">' .. h(verifyURL) .. "</a>"
			.. "</div>"
			.. '<div class="space-y-2">'
			.. '<label class="block text-xs font-medium text-neutral-300">Saved cookies for this plugin</label>'
			.. '<textarea id="verify-cookies" rows="4" class="w-full bg-neutral-950 border border-neutral-700 rounded-lg p-2 text-xs text-neutral-200 font-mono" placeholder="' .. h(placeholder) .. '">' .. h(verifyCookies) .. "</textarea>"
			.. '<label class="block text-xs font-medium text-neutral-300">Saved browser user-agent</label>'
			.. '<input type="text" id="verify-useragent" value="' .. h(useragent) .. '" class="w-full bg-neutral-950 border border-neutral-700 rounded-lg p-2 text-xs text-neutral-200">'
			.. "</div>"
			.. '<div class="flex gap-2">'
			.. '<button id="save-verify-search" type="button" onclick="saveVerifySearch(\'' .. h(pluginID) .. '\')" class="bg-amber-600 hover:bg-amber-500 text-white rounded-md px-4 py-2 text-sm font-medium">Save & retry</button>'
			.. '<button type="button" onclick="this.closest(\'#verify-modal\').classList.add(\'hidden\')" class="border border-neutral-600 text-neutral-300 hover:bg-neutral-800 rounded-md px-4 py-2 text-sm">Cancel</button>'
			.. "</div>"
			.. "</div>"
			.. "</div></div>"
		.. "<script>"
		.. "function saveVerifySearch(pid) {"
		.. "  var cookies = document.getElementById('verify-cookies').value;"
		.. "  var ua = document.getElementById('verify-useragent').value;"
		.. "  var form = new URLSearchParams();"
		.. "  form.append('cookies', cookies);"
		.. "  form.append('user_agent', ua);"
		.. "  var qs = new URLSearchParams(window.location.search).toString();"
		.. "  var url = '/action/save-verify/' + encodeURIComponent(pid) + (qs ? '?' + qs : '');"
		.. "  fetch(url, {"
		.. "    method: 'POST',"
		.. "    headers: {"
		.. "      'Content-Type': 'application/x-www-form-urlencoded',"
		.. "      'X-CSRF-Token': '" .. h(data.csrf_token or "") .. "',"
		.. "    },"
		.. "    body: form.toString()"
		.. "  }).then(r => {"
		.. "    if (r.status === 303 || r.ok) {"
		.. "      document.getElementById('verify-modal').classList.add('hidden');"
		.. "      location.reload();"
		.. "    } else {"
		.. "      alert('Save failed: ' + r.status);"
		.. "    }"
		.. "  }).catch(e => { alert('Save failed: ' + e.message); });"
		.. "}"
		.. "</script>"
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
				Extra = { "q", q, "pluginID", pluginID, "genre", genre },
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
				Extra = { "q", q, "pluginID", pluginID, "genre", genre },
				Compact = false,
			},
		})
	end

	return [[<h1 class="text-xl font-semibold mb-6">Search]]
		.. pluginBadge
		.. [[</h1>]]
		.. '<form method="get" action="/view/search" class="flex gap-2 items-end mb-8">'
		.. "<div>"
		.. '<label for="pluginID" class="block text-xs text-neutral-400 mb-1">Plugin</label>'
		.. '<select id="pluginID" name="pluginID" onchange="this.form.submit()" class="bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">'
		.. pluginOpts
		.. "</select>"
		.. "</div>"
		.. '<div class="flex-1 max-w-md">'
		.. '<label for="q" class="block text-xs text-neutral-400 mb-1">Query</label>'
		.. '<input id="q" name="q" type="text" value="'
		.. h(q)
		.. '" placeholder="Manga title..." class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">'
		.. "</div>"
		.. (genreOpts ~= "" and ("<div>" .. '<label for="genre" class="block text-xs text-neutral-400 mb-1">Genre</label>' .. '<select id="genre" name="genre" class="bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">' .. genreOpts .. "</select>" .. "</div>") or "")
		.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Search</button>'
		.. (topPagination ~= "" and topPagination or "")
		.. "</form>"
		.. resultsHTML
		.. "\n"
		.. noResults
		.. "\n"
		.. challengeHTML
		.. "\n"
		.. verifyHTML
		.. "\n"
		.. bottomPagination
end
