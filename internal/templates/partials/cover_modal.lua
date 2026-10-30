--  partials/cover_modal.lua
--  Cover picker modal: current cover + fetched alternative candidates.
--  Rendered standalone by /action/fetch-covers (X-Partial) so the Covers
--  button refreshes only this modal instead of the whole detail page.
--  data.PluginID, data.MangaID, data.Manga.CoverURL,
--  data.AltCovers ([]AltCoverRow), data.CoverVersions, data.csrf_token,
--  data.CurrentTitle

local function h(s)
	if s == nil then
		return ""
	end
	return tostring(s):gsub("&", "&amp;"):gsub("<", "&lt;"):gsub(">", "&gt;"):gsub('"', "&quot;")
end

local function thumbURL(url)
	if url:match("https://uploads%.mangadex%.org/covers/") then
		local base = url:match("^(https://uploads%.mangadex%.org/covers/[^/]+/[^/]+)")
		if base then
			return base .. ".256.jpg"
		end
	end
	return url
end

return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local manga = data.Manga or {}
	local altCovers = data.AltCovers or {}
	local coverVersions = data.CoverVersions or {}
	local currentTitle = data.CurrentTitle or manga.Title or ""
	local token = data.csrf_token or ""

	local coverItems = {
		{ url = manga.CoverURL or "", source = "current", current = true },
	}
	local seenCovers = {}
	if manga.CoverURL and manga.CoverURL ~= "" then
		seenCovers[manga.CoverURL] = true
	end
	for _, c in ipairs(altCovers) do
		local u = c.URL or ""
		if u ~= "" and not seenCovers[u] then
			seenCovers[u] = true
			coverItems[#coverItems + 1] = { url = u, source = c.Source or "" }
		end
	end

	local coverGrid = ""
	for _, c in ipairs(coverItems) do
		if c.url ~= "" then
			coverGrid = coverGrid
				.. '<form action="/action/set-cover/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" method="POST" class="group relative w-32">'
				.. '<input type="hidden" name="csrf_token" value="'
				.. h(token)
				.. '">'
				.. '<input type="hidden" name="url" value="'
				.. h(c.url)
				.. '">'
				.. '<img src="/image?pluginID='
				.. h(pluginID)
				.. "&url="
				.. h(thumbURL(c.url))
				.. "&prio=high"
				.. (coverVersions[c.url] and coverVersions[c.url] ~= "" and ("&cb=" .. h(coverVersions[c.url])) or "")
				.. '" alt="cover" loading="lazy" class="w-32 aspect-[2/3] rounded-lg object-cover border-2 transition cursor-pointer '
				.. (c.current and "border-emerald-500" or "border-transparent group-hover:border-blue-400")
				.. '" onclick="submitForm(this.closest(\'form\'))" title="Use this cover">'
				.. '<div class="text-xs text-neutral-400 text-center mt-1 truncate">'
				.. h(c.current and "current" or c.source)
				.. "</div>"
				.. "</form>"
		end
	end

	local body = '<div id="cover-modal" class="hidden fixed inset-0 z-50 flex items-center justify-center" style="display:none">'
		.. '<div class="absolute inset-0 bg-black/70" onclick="var m=this.parentElement;m.classList.add(\'hidden\');m.style.display=\'none\'"></div>'
		.. '<div class="relative bg-neutral-900 border border-neutral-700 rounded-xl p-4 max-w-3xl max-h-[80vh] overflow-y-auto">'
		.. '<div class="flex items-center justify-between mb-3">'
		.. '<h3 class="text-sm font-semibold text-neutral-200">Pick a cover</h3>'
		.. '<button onclick="var m=this.closest(\'#cover-modal\');m.classList.add(\'hidden\');m.style.display=\'none\'" class="text-neutral-400 hover:text-neutral-200 text-lg leading-none" aria-label="Close">&times;</button>'
		.. "</div>"
		.. '<div class="flex flex-wrap gap-3">'
		.. coverGrid
		.. "</div>"
		.. '<div class="mt-4 flex items-center gap-2">'
		.. '<form action="/action/clear-covers/'
		.. h(pluginID)
		.. "/"
		.. h(mangaID)
		.. '" method="POST" class="inline">'
		.. '<input type="hidden" name="csrf_token" value="' .. h(token) .. '">'
		.. '<button type="submit" class="text-xs text-red-400 hover:text-red-300 border border-red-600/50 rounded-md px-3 py-1" onclick="return confirm(\'Clear all cover candidates and fetch fresh?\');">Clear Covers</button>'
		.. "</form>"
		.. '<form action="/action/fetch-covers/'
		.. h(pluginID)
		.. "/"
		.. h(mangaID)
		.. '" method="POST" class="inline">'
		.. '<input type="hidden" name="csrf_token" value="' .. h(token) .. '">'
		.. '<input type="hidden" name="manga_title" value="' .. h(currentTitle) .. '" />'
		.. '<button type="submit" class="text-xs text-blue-400 hover:text-blue-300 border border-blue-600/50 rounded-md px-3 py-1">Fetch More</button>'
		.. "</form>"
		.. "</div>"
		.. "</div>"
		.. "</div>"
	return body
end
