-- views/updates.lua
-- Updates page: flat list of fresh chapter updates + recently added library titles.
-- Called as: updates(data) -> string (body HTML only, layout wraps it)

local pagination = require("partials.pagination")

return function(data)
	local allItems = data.Items or {}
	local page = data.Page or 1
	local totalPages = data.TotalPages or 1

	if #allItems == 0 then
		return [[
<div class="py-16 text-center text-neutral-500">
    <p class="mb-2">No updates yet</p>
    <a href="/view/library" class="text-indigo-400 hover:text-indigo-300 text-sm">Go to library</a>
</div>]]
	end

	local rows = {}
	for _, entry in ipairs(allItems) do
		local title = entry.Title or ""
		local pluginID = entry.PluginID or ""
		local sourceMangaID = entry.MangaID or ""
		local coverURL = entry.CoverURL or ""
		local pluginIcon = entry.PluginIcon or ""
		local pluginName = entry.PluginName or ""
		local readChapters = entry.ReadChapters or 0
		local totalChapters = entry.TotalChapters or 0
		local badge = ""
		local tsAttr = ""
		local formattedDate = ""

		if entry.Type == "update" then
			badge = '<span class="shrink-0 text-[10px] px-1.5 py-0.5 rounded-full bg-red-500 text-white font-medium">New</span>'
			tsAttr = h(tostring(entry.Date or ""))
			formattedDate = h(formatDate(tostring(entry.Date or "")))
		else
			badge = '<span class="shrink-0 text-[10px] px-1.5 py-0.5 rounded-full bg-indigo-500/15 text-indigo-400">New title</span>'
			tsAttr = h(tostring(entry.Date or ""))
			formattedDate = h(formatDate(tostring(entry.Date or "")))
		end

		local coverHtml
		if coverURL ~= "" then
			coverHtml = string.format(
				'<img src="/image?pluginID=%s&amp;url=%s" alt="%s" class="w-16 aspect-[2/3] object-cover rounded shrink-0" loading="lazy" onerror="this.onerror=null;this.src=\'/static/img/placeholder.svg\';">',
				h(pluginID),
				h(coverURL),
				h(title)
			)
		else
			coverHtml = string.format(
				'<div class="w-16 aspect-[2/3] bg-neutral-800 rounded flex items-center justify-center text-neutral-500 text-sm font-semibold shrink-0">%s</div>',
				h(getInitials(title))
			)
		end

		local iconHtml = ""
		if pluginIcon ~= "" then
			iconHtml = string.format('<img src="%s" alt="" class="h-3.5 w-3.5 rounded-sm object-cover">', h(pluginIcon))
		end

		rows[#rows + 1] = string.format(
			[[
    <a href="/view/manga/%s/%s" class="flex items-center gap-4 py-4 -mx-2 px-2 rounded hover:bg-neutral-900 transition">
        %s
        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between">
            <div class="text-sm font-medium truncate">%s</div>
            <div class="text-xs text-neutral-500 shrink-0 hidden sm:block" data-ts="%s" title="%s">%s</div>
          </div>
          <div class="flex items-center gap-2 mt-1">
            <span class="text-xs text-neutral-400">%s/%s chapters</span>
            <span class="inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-500">%s%s</span>
            <div class="text-xs text-neutral-500 sm:hidden" data-ts="%s" title="%s">%s</div>
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
			iconHtml,
			h(pluginName),
			tsAttr,
			formattedDate,
			formattedDate
		)
	end

	local listHTML = string.format(
		[[
<h1 class="text-xl font-semibold mb-6">Updates</h1>
<div class="divide-y divide-neutral-800">
%s
</div>
<script>
function refreshRelativeTimes() {
  document.querySelectorAll('[data-ts]').forEach(el => {
    const d = new Date(el.dataset.ts);
    const diff = Date.now() - d.getTime();
    const mins = Math.floor(diff/60000);
    if (mins < 60) el.textContent = mins + 'm ago';
    else if (mins < 1440) el.textContent = Math.floor(mins/60) + 'h ago';
    else el.textContent = Math.floor(mins/1440) + 'd ago';
  });
}
refreshRelativeTimes();
setInterval(refreshRelativeTimes, 60000);
</script>]],
		table.concat(rows, "\n")
	)

	local pag = ""
	if totalPages > 1 then
		pag = pagination({
			Pagination = { Base = "/view/updates", Param = "page", Current = page, Total = totalPages },
		})
	end

	return listHTML .. pag
end
