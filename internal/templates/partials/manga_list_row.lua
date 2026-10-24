-- partials/manga_list_row.lua
-- Shared row + section-header renderer for views/history.lua and
-- views/updates.lua (the 42-line jscpd clone between the two).
--
-- One entry point (house convention: every partial returns a single
-- function(data)); data.Which selects the block:
--
-- Which = "row"        data.Entry: normalized entry map, data.Opts: {onerrorCover}
-- Which = "section"    data.Label, data.Class (extra colour class), data.Inner (raw HTML prefix)
--
-- Entry shape (caller fills):
--   PluginID, SourceMangaID, Title, CoverURL, Badge (HTML fragment)
--   ReadChapters, TotalChapters, TsAttr (escaped raw), FormattedDate (escaped)

return function(data)
	local which = data.Which or "row"

	if which == "section" then
		local label = data.Label or ""
		-- Color class replaces the neutral default (e.g. red for "New chapters"),
		-- not appends to it: two text-color classes would conflict.
		local cls = data.Class or "text-neutral-300"
		local inner = (data.Inner or "") .. h(label)
		return string.format(
			'<div class="sticky top-0 z-10 -mx-2 py-2 px-2 bg-neutral-950/95 backdrop-blur text-sm font-semibold %s border-b border-neutral-800">%s</div>',
			h(cls),
			inner
		)
	end

	local entry = data.Entry or {}
	local opts = data.Opts or {}
	local title = entry.Title or ""
	local pluginID = entry.PluginID or ""
	local sourceMangaID = entry.SourceMangaID or ""
	local coverURL = entry.CoverURL or ""
	local readChapters = entry.ReadChapters or 0
	local totalChapters = entry.TotalChapters or 0
	local tsAttr = entry.TsAttr or ""
	local formattedDate = entry.FormattedDate or ""
	local badge = entry.Badge or ""

	local coverHtml
	if coverURL ~= "" then
		local onerror = ""
		if opts.onerrorCover then
			onerror = " onerror=\"this.onerror=null;this.src='/static/img/placeholder.svg';\""
		end
		coverHtml = string.format(
			'<img src="/image?pluginID=%s&amp;url=%s" alt="%s" class="w-24 aspect-[2/3] object-cover rounded shrink-0" loading="lazy"%s>',
			h(pluginID),
			h(coverURL),
			h(title),
			onerror
		)
	else
		coverHtml = string.format(
			'<div class="w-24 aspect-[2/3] bg-neutral-800 rounded flex items-center justify-center text-neutral-500 text-base font-semibold shrink-0">%s</div>',
			h(getInitials(title))
		)
	end

	return string.format(
		[[
    <a href="/view/manga/%s/%s" class="flex items-center gap-4 py-4 -mx-2 px-2 rounded hover:bg-neutral-900 transition">
        %s
        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between">
            <div class="text-base font-medium truncate">%s</div>
            <div class="text-sm text-neutral-500 shrink-0 hidden sm:block" data-ts="%s" title="%s">%s</div>
          </div>
          <div class="flex items-center gap-2 mt-1">
            <span class="text-sm text-neutral-400">%s/%s chapters</span>
            <span class="inline-flex items-center gap-1.5 text-sm px-2.5 py-1 rounded-full bg-neutral-800 text-neutral-500">%s</span>
            <div class="text-sm text-neutral-500 sm:hidden" data-ts="%s" title="%s">%s</div>
          </div>
        </div>
    </a>]],
		h(pluginID),
		h(sourceMangaID),
		coverHtml,
		h(title),
		tsAttr,
		formattedDate,
		formattedDate,
		h(tostring(readChapters)),
		h(tostring(totalChapters)),
		badge,
		tsAttr,
		formattedDate,
		formattedDate
	)
end
