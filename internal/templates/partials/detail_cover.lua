--  partials/detail_cover.lua
--  Left cover column of the manga detail page: the cover image (or the
--  initials placeholder) plus the Covers/Clear/Refetch strip beneath it.
--  data.PluginID, data.MangaID, data.Manga, data.CurrentTitle,
--  data.CoverVersions, data.CoverDim, data.csrf_token
return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local manga = data.Manga or {}
	local currentTitle = data.CurrentTitle or manga.Title or ""
	local coverVersions = data.CoverVersions or {}

	local coverWrap = '<div id="cover-wrap" class="relative" data-key="' .. h(pluginID) .. ":" .. h(mangaID) .. '">'
	local coverHTML
	if manga.CoverURL and manga.CoverURL ~= "" then
		coverHTML = coverWrap
			.. '<img src="/image?pluginID='
			.. h(pluginID)
			.. "&url="
			.. h(manga.CoverURL)
			.. "&prio=high"
			.. (coverVersions[manga.CoverURL] and coverVersions[manga.CoverURL] ~= "" and ("&cb=" .. h(
				coverVersions[manga.CoverURL]
			)) or "")
			.. '" alt="'
			.. h(manga.Title or "")
			.. '" class="w-full aspect-[2/3] rounded-xl object-cover">'
			.. '<div id="cover-dim" class="lib-dim" style="display:'
			.. (data.CoverDim == 1 and "block" or "none")
			.. ';position:absolute;top:0;left:0;right:0;bottom:0;border-radius:0.75rem;background:rgba(0,0,0,0.82);backdrop-filter:blur(3px);"></div>'
			.. "</div>"
			.. '<div class="mt-2 flex items-center gap-3 text-xs">'
			.. '<form action="/action/toggle-cover-dim/'
			.. h(pluginID)
			.. "/"
			.. h(data.MangaID)
			.. '" method="POST" style="display:inline">'
			.. '<button type="submit" id="cover-dim-btn" title="Dim the cover image" class="inline-flex items-center text-neutral-400 hover:text-neutral-200 transition cursor-pointer">'
			.. (data.CoverDim == 1 and "Show cover" or "Hide cover")
			.. "</button>"
			.. "</form>"
			.. '<form action="/action/fetch-covers/'
			.. h(pluginID)
			.. "/"
			.. h(data.MangaID)
			.. '" method="POST" style="display:inline" data-open-covers="1">'
			.. csrfInput(data.csrf_token or "")
			.. '<input type="hidden" name="manga_title" value="'
			.. h(currentTitle)
			.. '">'
			.. '<button type="submit" title="Fetch cover candidates if needed, then pick the main cover" class="inline-flex items-center text-neutral-400 hover:text-neutral-200 transition cursor-pointer">Covers</button>'
			.. "</form>"
			.. '<form action="/action/clear-covers/'
			.. h(pluginID)
			.. "/"
			.. h(data.MangaID)
			.. '" method="POST" style="display:inline">'
			.. csrfInput(data.csrf_token or "")
			.. '<button type="submit" title="Drop stored cover candidates so the next fetch re-pulls them fresh" class="inline-flex items-center text-neutral-400 hover:text-neutral-200 transition cursor-pointer">Clear</button>'
			.. "</form>"
			.. '<form action="/action/refetch-cover/'
			.. h(pluginID)
			.. "/"
			.. h(data.MangaID)
			.. '" method="POST" style="display:inline">'
			.. '<button type="submit" title="Re-download the cover from the source" class="inline-flex items-center text-neutral-400 hover:text-neutral-200 transition cursor-pointer">Refetch</button>'
			.. "</form>"
			.. "</div>"
	else
		coverHTML = '<div class="w-full aspect-[2/3] bg-neutral-800 rounded-xl flex items-center justify-center text-neutral-500 text-4xl font-semibold">'
			.. h(getInitials(manga.Title or ""))
			.. "</div>"
			.. '<div class="mt-2 flex items-center gap-3 text-xs">'
			.. '<form action="/action/fetch-covers/'
			.. h(pluginID)
			.. "/"
			.. h(data.MangaID)
			.. '" method="POST" style="display:inline" data-open-covers="1">'
			.. csrfInput(data.csrf_token or "")
			.. '<input type="hidden" name="manga_title" value="'
			.. h(currentTitle)
			.. '">'
			.. '<button type="submit" title="Fetch cover candidates if needed, then pick the main cover" class="inline-flex items-center text-neutral-400 hover:text-neutral-200 transition cursor-pointer">Covers</button>'
			.. "</form>"
			.. '<form action="/action/clear-covers/'
			.. h(pluginID)
			.. "/"
			.. h(data.MangaID)
			.. '" method="POST" style="display:inline">'
			.. csrfInput(data.csrf_token or "")
			.. '<button type="submit" title="Drop stored cover candidates so the next fetch re-pulls them fresh" class="inline-flex items-center text-neutral-400 hover:text-neutral-200 transition cursor-pointer">Clear</button>'
			.. "</form>"
			.. "</div>"
	end

	return coverHTML
end
