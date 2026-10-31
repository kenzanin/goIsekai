--  views/detail.lua
--  Manga detail page: 2-column layout (cover | info), chapters below.
--  Called as: detail(data) -> string (body HTML only, layout wraps it)
--  data.PluginID, data.PluginName, data.PluginIcon, data.MangaID
--  data.Manga: types.Manga, data.AltTitles: []AltTitle, data.CurrentTitle: string
--  data.Chapters: []types.Chapter, data.Progress: map[string]database.ChapterProgress
--  data.Continue: *ContinuePoint, data.InLibrary: bool, data.Challenge: bool
--  data.CachedData: bool (true when serving cached data due to offline/plugin error)
--  data.ChCurrentPage/ChTotalPages: int
--  data.Categories, data.Related (enrichment)

local detailAlt = require("partials.detail_alt")
local detailChapters = require("partials.detail_chapters")
local detailCover = require("partials.detail_cover")
local detailCoverModal = require("partials.detail_cover_modal")
local detailMeta = require("partials.detail_meta")
local detailActions = require("partials.detail_actions")
local detailNotices = require("partials.detail_notices")
local verifyWizard = require("partials.verify_wizard")

return function(data)
	local pluginID = data.PluginID or ""
	local pluginName = data.PluginName or ""
	local pluginIcon = data.PluginIcon or ""
	local mangaID = data.MangaID or ""
	local manga = data.Manga or {}
	local altTitles = data.AltTitles or {}
	local currentTitle = data.CurrentTitle or manga.Title or ""
	local altSummaries = data.AltSummaries or {}
	local chapters = data.Chapters or {}
	local progress = data.Progress or {}
	local continuePoint = data.Continue
	local inLibrary = data.InLibrary or false
	local challenge = data.Challenge or false
	local cachedData = data.CachedData or false
	local chPage = data.ChCurrentPage or 1
	local chTotalPages = data.ChTotalPages or 1
	local cats = data.Categories or {}
	local rels = data.Related or {}
	local coverVersions = data.CoverVersions or {}
	local altCovers = data.AltCovers or {}
	-- Human verification wizard data
	local needsHumanVerify = data.NeedsHumanVerify or false
	local verifyURL = data.VerifyURL or ""
	local verifyCookies = data.VerifyCookies or ""
	local verifyUserAgent = data.VerifyUserAgent or ""

	local body = detailNotices({Challenge = challenge, CachedData = cachedData})

	-- Cover (left column)
	local coverHTML = detailCover({
		csrf_token = data.csrf_token or "",
		PluginID = pluginID,
		MangaID = mangaID,
		Manga = manga,
		CurrentTitle = currentTitle,
		CoverVersions = coverVersions,
		CoverDim = data.CoverDim,
	})

	local genreTagsHTML, relatedTagsHTML, synopsisHTML = detailMeta({
		csrf_token = data.csrf_token or "",
		PluginID = pluginID,
		MangaID = mangaID,
		Manga = manga,
		Related = rels,
		OverrideGenres = data.OverrideGenres,
	})

	local actionsHTML = detailActions({
		csrf_token = data.csrf_token or "",
		PluginID = pluginID,
		MangaID = mangaID,
		InLibrary = inLibrary,
		Continue = continuePoint,
		LastSynced = data.LastSynced,
	})

	-- Main 2-column layout
	body = body
		.. '<div class="flex flex-col md:flex-row gap-6 mb-4 items-start">'
		-- Left: cover (300px)
		.. '<div class="md:w-[300px] md:shrink-0">'
		.. coverHTML
		.. "</div>"
		-- Right: info column
		.. '<div class="flex-1 min-w-0">'
		.. '<h1 class="text-2xl font-semibold mb-2 leading-tight">'
		.. h(manga.Title or "")
		.. "</h1>"

	-- Plugin badge + status
	body = body
		.. '<div class="flex flex-wrap gap-2 mb-3">'
		.. '<span class="inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-500">'
		.. (pluginIcon ~= "" and ('<img src="' .. h(pluginIcon) .. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover">') or "")
		.. h(pluginName)
		.. "</span>"
		.. (manga.Status and manga.Status ~= "" and ('<span class="text-xs px-2 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">' .. h(
			manga.Status
		) .. "</span>") or "")
		.. "</div>"

	-- Genres
	if genreTagsHTML ~= "" then
		body = body .. '<div class="flex flex-wrap gap-1.5 mb-3">' .. genreTagsHTML .. "</div>"
	end

	-- Synopsis
	if synopsisHTML ~= "" then
		body = body .. synopsisHTML
	end

	-- Related manga with read-more toggle
	if relatedTagsHTML ~= "" then
		body = body
			.. '<div class="mb-3">'
			.. '<span class="text-xs font-semibold text-neutral-300 uppercase tracking-wide">Related / Recommended</span>'
			.. '<div id="related-text" class="text-sm text-neutral-400 mt-1.5 max-h-[6rem] overflow-hidden transition-all duration-300 relative">'
			.. '<div class="flex flex-wrap gap-1.5">'
			.. relatedTagsHTML
			.. "</div>"
			.. '<span id="related-fade" class="absolute inset-x-0 bottom-0 h-8 bg-gradient-to-t from-neutral-950 to-transparent pointer-events-none"></span></div>'
			.. '<button type="button" id="related-toggle" onclick="var t=document.getElementById(&quot;related-text&quot;);var f=document.getElementById(&quot;related-fade&quot;);var b=document.getElementById(&quot;related-toggle&quot;);t.classList.toggle(&quot;max-h-[6rem]&quot;);t.classList.toggle(&quot;max-h-none&quot;);f.style.display=t.classList.contains(&quot;max-h-none&quot;)?&quot;none&quot;:&quot;block&quot;;b.textContent=t.classList.contains(&quot;max-h-none&quot;)?&quot;Show less&quot;:&quot;Read more&quot;;" class="text-xs text-indigo-400 hover:text-indigo-300 transition mt-1 cursor-pointer">Read more</button></div>'
	end
	if manga.Author and manga.Author ~= "" then
		body = body .. '<p class="text-sm text-neutral-400 mb-2">' .. h(manga.Author) .. "</p>"
	end

	-- Action buttons + enrichment panel (below info, still in right column)
	body = body
		.. detailAlt({
			-- detailAlt renders 7 CSRF-protected forms and its own data table is a
			-- literal, so the token has to be handed in explicitly. Without it
			-- every one of those forms posts an empty token.
			csrf_token = data.csrf_token or "",
			PluginID = pluginID,
			MangaID = mangaID,
			CurrentTitle = currentTitle,
			AltTitles = altTitles,
			AltSummaries = altSummaries,
			Manga = manga,
			Author = manga.Author,
			Categories = cats,
			PluginGenres = manga.RawGenres,
			OverrideGenres = data.OverrideGenres or {},
		})
		.. actionsHTML
		.. "</div></div>"

	-- Chapters section
	body = body
		.. detailChapters({
			-- 5 CSRF-protected forms per chapter row (mark read, reset progress,
			-- CBZ export, ...); same reason as detailAlt above.
			csrf_token = data.csrf_token or "",
			PluginID = pluginID,
			MangaID = mangaID,
			Manga = manga,
			Chapters = chapters,
			Progress = progress,
			ChPage = chPage,
			ChTotalPages = chTotalPages,
		})

	-- Cover picker modal (shared partial): current cover + fetched candidates.
	body = body
		.. detailCoverModal({
			csrf_token = data.csrf_token or "",
			PluginID = pluginID,
			MangaID = mangaID,
			Manga = manga,
			AltCovers = altCovers,
			CoverVersions = coverVersions,
			CurrentTitle = currentTitle,
		})

	-- Human verification wizard modal (shared partial; renders "" when hidden).
	body = body
		.. verifyWizard({
			csrf_token = data.csrf_token or "",
			PluginID = pluginID,
			MangaID = mangaID,
			NeedsHumanVerify = needsHumanVerify,
			VerifyURL = verifyURL,
			VerifyCookies = verifyCookies,
			VerifyUserAgent = verifyUserAgent,
			Challenge = challenge,
		})

	return body
end
