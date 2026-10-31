--  partials/detail_actions.lua
--  Action row on the manga detail page: add/remove library, the far-right
--  Update button and the Migrate link. Returned as one HTML string.
--  data.PluginID, data.MangaID, data.InLibrary, data.Continue,
--  data.LastSynced, data.csrf_token
return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local inLibrary = data.InLibrary or false
	local continuePoint = data.Continue

	-- Action buttons; syncedHTML carries the far-right Update button
	local syncedHTML = ""
	local actionsHTML = '<div class="flex flex-wrap items-center gap-2 mt-4">'
		.. '<form method="post" action="/action/toggle-library/'
		.. h(pluginID)
		.. "/"
		.. h(mangaID)
		.. '">'
		.. csrfInput(data.csrf_token or "")
		.. (inLibrary and '<button type="submit" class="border border-emerald-600/50 text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20 rounded-md px-4 py-2 text-sm">✓ In Library</button>' or '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">+ Add to Library</button>')
		.. "</form>"
	if inLibrary then
		local synced = data.LastSynced
		syncedHTML = ""
		if type(synced) == "string" and #synced >= 16 then
			syncedHTML = '<form method="post" action="/action/sync-manga/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="ml-auto inline-flex items-center gap-2" data-job-action="sync-manga">'
				.. csrfInput(data.csrf_token or "")
				.. '<span class="text-xs text-neutral-500 cursor-default" title="Last refresh check - click Update to refresh now">updated '
				.. synced:sub(1, 10)
				.. " "
				.. synced:sub(12, 16)
				.. "</span>"
				.. '<button type="submit" title="Refresh this manga now" class="border border-neutral-700 text-neutral-300 hover:bg-neutral-800 rounded-md px-4 py-2 text-sm cursor-pointer">⟳ Update</button>'
				.. "</form>"
		else
			syncedHTML = '<form method="post" action="/action/sync-manga/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="ml-auto" data-job-action="sync-manga">'
				.. csrfInput(data.csrf_token or "")
				.. '<button type="submit" title="Refresh this manga now" class="border border-neutral-700 text-neutral-300 hover:bg-neutral-800 rounded-md px-4 py-2 text-sm cursor-pointer">⟳ Update</button>'
				.. "</form>"
		end
	end

	-- Migrate button (beside Update)
	local migrateHTML = ""
	if inLibrary then
		migrateHTML = '<a href="/view/migrate/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. '" class="ml-auto border border-amber-500/40 bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 rounded-md px-4 py-2 text-sm">⤴ Migrate</a>'
	end

	if continuePoint then
		local cp = continuePoint
		local label, titleAttr
		if cp.Page and cp.Page > 1 then
			label = "▶ Continue · Ch." .. formatChapterNum(cp.ChapterN)
			titleAttr = "Continue: Ch. " .. formatChapterNum(cp.ChapterN) .. ", page " .. tostring(cp.Page)
		elseif cp.Started then
			label = "▶ Continue · Ch." .. formatChapterNum(cp.ChapterN)
			titleAttr = "Continue: Ch. " .. formatChapterNum(cp.ChapterN)
		else
			label = "▶ Start Reading"
			titleAttr = "Start Reading"
		end
		actionsHTML = actionsHTML
			.. '<a href="/view/read/'
			.. h(ue(pluginID))
			.. "/"
			.. h(ue(mangaID))
			.. "/"
			.. h(ue(cp.ChapterID or ""))
			.. "?page="
			.. tostring(cp.Page or 1)
			.. '" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium" title="'
			.. h(titleAttr)
			.. '">'
			.. h(label)
			.. "</a>"
	end
	actionsHTML = actionsHTML .. migrateHTML .. (syncedHTML or "") .. "</div>"

	return actionsHTML
end
