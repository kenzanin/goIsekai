-- views/library.lua
-- Manga library grid with stats sidebar and duplicate detection.
-- Replaces views/library.jet.
-- Called as: library(data) -> string (body HTML only, layout wraps it)
-- data.Mangas: []database.Manga, data.Q: string, data.Page/TotalPages: int
-- data.Stats: {TotalTitles, StatusLine, ReadLine, HasUpdates, ReadingTime, MostLine, HasFewest, FewestLine}
-- data.LibraryStats: map[string]map[string]any, data.Ratios: map[string]float64
-- data.DuplicateCount: int, data.DuplicateGroups: []DuplicateGroup
-- data.PluginCounts: []PluginCount

local librarySidebar = require("partials.library_sidebar")
local libraryCards = require("partials.library_cards")
local pagination = require("partials.pagination")

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

	local sidebarHTML = librarySidebar({
		Mangas = mangas,
		Q = q,
		Stats = stats,
		DuplicateCount = duplicateCount,
		DuplicateGroups = duplicateGroups,
		PluginCounts = pluginCounts,
	})

	local mangaCards = libraryCards({
		Mangas = mangas,
		LibraryStats = libraryStats,
		Ratios = ratios,
	})

	local subtitle = tostring(stats.TotalTitles or 0)
		.. " titles · "
		.. tostring(totalPages)
		.. " page"
		.. (totalPages > 1 and "s" or "")

	local pagBase = "/view/library" .. (q ~= "" and "?q=" .. h(q) or "")
	local topPagination = pagination({
		Pagination = { Base = pagBase, Param = "page", Current = page, Total = totalPages, Compact = true },
	})

	return '<div class="flex items-center gap-3 flex-wrap mb-6">'
		.. '<div class="shrink-0"><h1 class="text-xl font-semibold">Library</h1>'
		.. '<div class="text-xs text-neutral-500">'
		.. h(subtitle)
		.. "</div></div>"
		.. '<form method="get" action="/view/library" class="flex-1 min-w-[180px] max-w-md" role="search">'
		.. '<div class="relative">'
		.. '<svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-neutral-500 pointer-events-none" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path stroke-linecap="round" d="m21 21-4.35-4.35"/></svg>'
		.. '<input type="search" name="q" value="'
		.. h(q)
		.. '" placeholder="Search library…" class="w-full bg-neutral-900 border border-neutral-700 rounded-md pl-9 pr-3 py-1.5 text-sm placeholder-neutral-500 focus:outline-none focus:border-indigo-500">'
		.. "</div></form>"
		.. topPagination
		.. '<div class="view-mode-toggle shrink-0 flex items-center gap-1" role="group" aria-label="View mode" data-view-mode="grid">'
		.. '<button type="button" id="view-grid-btn" aria-pressed="true" class="p-1.5 rounded-md hover:bg-neutral-800 transition" title="Grid view">'
		.. '<svg class="w-4 h-4 text-neutral-400" fill="currentColor" viewBox="0 0 16 16"><rect x="1" y="1" width="6" height="6" rx="1"/><rect x="9" y="1" width="6" height="6" rx="1"/><rect x="1" y="9" width="6" height="6" rx="1"/><rect x="9" y="9" width="6" height="6" rx="1"/></svg></button>'
		.. '<button type="button" id="view-list-btn" aria-pressed="false" class="p-1.5 rounded-md hover:bg-neutral-800 transition" title="List view">'
		.. '<svg class="w-4 h-4 text-neutral-400" fill="currentColor" viewBox="0 0 16 16"><rect x="1" y="1" width="14" height="3" rx="1"/><rect x="1" y="6.5" width="14" height="3" rx="1"/><rect x="1" y="12" width="14" height="3" rx="1"/></svg></button></div>'
		.. '<form method="post" action="/action/sync" class="ml-auto">'
		.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium inline-flex items-center gap-1.5">'
		.. '<svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 0 0 4.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 0 1-15.357-2m15.357 2H15"/></svg>Update</button>'
		.. "</form></div>"
		.. sidebarHTML
		.. mangaCards
		.. (sidebarHTML ~= "" and "</div></div>" or "")
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
