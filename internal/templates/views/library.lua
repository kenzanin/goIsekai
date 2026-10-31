-- views/library.lua
-- Manga library grid with stats sidebar and duplicate detection.
-- Replaces views/library.jet.
-- Called as: library(data) -> string (body HTML only, layout wraps it)
-- data.Mangas: []database.Manga, data.Q: string, data.Page/TotalPages: int
-- data.Stats: {TotalTitles, StatusLine, ReadLine, HasUpdates, ReadingTime, MostLine, HasFewest, FewestLine}
-- data.LibraryStats: map[string]map[string]any, data.Ratios: map[string]float64
-- data.DuplicateCount: int, data.DuplicateGroups: []DuplicateGroup
-- data.DupColors: map[mangaRowID]hex colour of the manga's duplicate group
-- data.PluginCounts: []PluginCount

local librarySidebar = require("partials.library_sidebar")
local libraryCards = require("partials.library_cards")
local pagination = require("partials.pagination")
local libraryToolbar = require("partials.library_toolbar")

return function(data)
	local mangas = data.Mangas or {}
	local q = data.Q or ""
	local page = data.Page or 1
	local totalPages = data.TotalPages or 1
	local stats = data.Stats or {}
	local libraryStats = data.LibraryStats or {}
	local ratios = data.Ratios or {}
	local duplicateCount = data.DuplicateCount or 0
	local duplicateGroups = data.DuplicateGroups or {}
	local pluginCounts = data.PluginCounts or {}

	local pluginID = data.PluginID or ""

	local sidebarHTML = librarySidebar({
		Mangas = mangas,
		Q = q,
		Stats = stats,
		DuplicateCount = duplicateCount,
		DuplicateGroups = duplicateGroups,
		PluginCounts = pluginCounts,
		PluginID = pluginID,
		Status = data.Status or "all",
		View = data.View or "",
		Tag = data.Tag or "",
	})

	local mangaCards = libraryCards({
		Mangas = mangas,
		LibraryStats = libraryStats,
		Ratios = ratios,
		DupColors = data.DupColors or {},
	})
	local pluginName = data.PluginName or pluginID
	local titleCount = data.ResultCount
	if not titleCount then
		titleCount = stats.TotalTitles or 0
	end
	local subtitle = tostring(titleCount)
		.. ((q ~= "" or pluginID ~= "") and " results · " or " titles · ")
		.. tostring(totalPages)
		.. " page"
		.. (totalPages > 1 and "s" or "")

	local sortKey = data.Sort or "updated"

	local statusChips, tagBar, sortCarried, searchCarried,
		topToolbar, bottomPagination = libraryToolbar({
		csrf_token = data.csrf_token or "",
		Q = q,
		PluginID = pluginID,
		Sort = data.Sort,
		Status = data.Status,
		Tag = data.Tag,
		View = data.View,
		CategoryCounts = data.CategoryCounts,
	})

	return '<div class="flex items-center gap-3 flex-wrap mb-6">'
		.. '<div class="shrink-0"><h1 class="text-xl font-semibold">Library</h1>'
		.. '<div class="text-xs text-neutral-500">'
		.. h(subtitle)
		.. "</div></div>"
		.. '<form method="get" action="/view/library" class="flex-1 min-w-[180px] max-w-md" role="search">'
		.. searchCarried
		.. '<div class="flex gap-2">'
		.. '<div class="relative flex-1">'
		.. '<svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-neutral-500 pointer-events-none" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path stroke-linecap="round" d="m21 21-4.35-4.35"/></svg>'
		.. '<input type="search" name="q" value="'
		.. h(q)
		.. '" placeholder="Search library…" class="w-full bg-neutral-900 border border-neutral-700 rounded-md pl-9 pr-3 py-1.5 text-sm placeholder-neutral-500 focus:outline-none focus:border-indigo-500">'
		.. "</div>"
		.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-3 py-1.5 text-sm font-medium">Search</button>'
		.. "</div></form>"
		.. '<div class="flex items-center gap-1.5 flex-wrap">'
		.. statusChips
		.. "</div>"
		.. '<form method="get" action="/view/library" class="flex items-center">'
		.. sortCarried
		.. '<select name="sort" onchange="this.form.submit()" class="bg-neutral-900 border border-neutral-700 rounded-md px-2 py-1.5 text-xs text-neutral-300">'
		.. '<option value="updated"'
		.. (sortKey == "updated" and " selected" or "")
		.. ">Recently updated</option>"
		.. '<option value="title"'
		.. (sortKey == "title" and " selected" or "")
		.. ">Title A-Z</option>"
		.. '<option value="read"'
		.. (sortKey == "read" and " selected" or "")
		.. ">Chapters read</option>"
		.. '<option value="progress"'
		.. (sortKey == "progress" and " selected" or "")
		.. ">Progress</option>"
		.. '<option value="recent_read"'
		.. (sortKey == "recent_read" and " selected" or "")
		.. ">Recently read</option>"
		.. '<option value="recent_added"'
		.. (sortKey == "recent_added" and " selected" or "")
		.. ">Recently added</option>"
		.. '<option value="duplicates"'
		.. (sortKey == "duplicates" and " selected" or "")
		.. ">Duplicates first</option>"
		.. "</select>"
		.. "</form>"
		.. tagBar
		.. "</div>"
		.. topToolbar
		.. sidebarHTML
		.. mangaCards
		.. (sidebarHTML ~= "" and "</div></div>" or "")
		.. bottomPagination
		.. [[
<style>
  .view-container[data-view-mode="list"] {
    display: flex;
    flex-direction: column;
  }
  .view-container[data-view-mode="list"] .view-item {
    flex-direction: row;
  }
  .view-container[data-view-mode="list"] .view-item > div:first-child {
    width: 80px;
    height: 110px;
    border-radius: 0.5rem 0 0 0.5rem;
    overflow: hidden;
  }
  .view-container[data-view-mode="list"] .view-item .relative {
    width: 80px;
    height: 110px;
  }
  .view-container[data-view-mode="list"] .view-item > .relative img,
  .view-container[data-view-mode="list"] .view-item > .relative > div {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .view-container[data-view-mode="list"] .view-item .lib-dim {
    border-radius: 0.5rem 0 0 0.5rem;
  }
  .view-container[data-view-mode="list"] .view-item .p-3 {
    flex: 1;
    padding: 0.75rem 1rem;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }
  .view-container[data-view-mode="list"] .view-item:hover {
    transform: none;
  }
  .view-container[data-view-mode="list"] .view-item .view-meta {
    display: flex;
  }
  .view-container[data-view-mode="list"] .view-item .absolute.top-2,
  .view-container[data-view-mode="list"] .view-item .absolute.bottom-2 {
    display: none;
  }
  .view-mode-toggle button[aria-pressed="true"] svg,
  .view-mode-toggle button.active svg {
    color: #818cf8;
  }
</style>]]
end
