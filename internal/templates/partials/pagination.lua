-- partials/pagination.lua
-- Numbered page navigation. Replaces partials/pagination.jet.
-- Called as: pagination(data) -> string
-- One order everywhere: [← Prev] 1 2 … 4 5 [Next →]
-- data.Pagination mirrors the Go Pagination struct:
--   { Base, Param, Current, Total, Extra, Inner, Compact }
-- Inner (optional): raw (already-escaped) HTML rendered at the LEFT of the
-- page row so external controls (e.g. the chapter-actions dropdown) share the
-- same line as the page numbers. Without Inner the numbers stay centered.
-- Compact (optional): no outer margins + tighter buttons for inline toolbar use.

local BTN_BASE = "border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-sm"
local BTN_NUM = "border border-neutral-700 hover:bg-neutral-800 text-neutral-300 rounded-md px-4 py-1.5 text-sm"
local BTN_CURRENT = "bg-indigo-600 text-white rounded-md px-4 py-1.5 text-sm"
local BTN_DISABLED = "border border-neutral-700 text-neutral-600 rounded-md px-3 py-1.5 text-sm opacity-50 cursor-not-allowed"
local ELLIPSIS = "px-1.5 text-neutral-500"

-- Tighter button classes for the compact (inline toolbar) variant.
local C_BASE = "border border-neutral-700 hover:bg-neutral-800 rounded px-2 py-1 text-xs"
local C_NUM = "border border-neutral-700 hover:bg-neutral-800 text-neutral-300 rounded px-2.5 py-1 text-xs"
local C_CURRENT = "bg-indigo-600 text-white rounded px-2.5 py-1 text-xs"
local C_DISABLED = "border border-neutral-700 text-neutral-600 rounded px-2 py-1 text-xs opacity-50 cursor-not-allowed"

local function cls(standard, small, compact)
	if compact then
		return small
	end
	return standard
end

local function pageURL(p, n)
	local url = p.Base
	local sep = string.find(url, "?") and "&" or "?"
	url = url .. sep .. p.Param .. "=" .. tostring(n)
	if p.Extra and #p.Extra > 0 then
		for i = 1, #p.Extra, 2 do
			local k, v = p.Extra[i], p.Extra[i+1]
			if v ~= "" then
				url = url .. "&" .. k .. "=" .. tostring(v)
			end
		end
	end
	return url
end

return function(data)
	local p = data.Pagination
	if not p or not p.Total or p.Total <= 1 then
		return ""
	end

	local compact = p.Compact == true
	local parts = {}
	local function emit(s)
		parts[#parts + 1] = s
	end

	local inner = p.Inner or ""
	if inner ~= "" then
		-- Two-zone toolbar: [inner controls] on the left, page numbers on the right.
		emit('<div class="flex items-center gap-3 mt-4 mb-4 flex-wrap">')
		emit('<div class="flex items-center gap-2 flex-wrap shrink-0">' .. inner .. "</div>")
		emit('<div class="ml-auto flex items-center gap-2 flex-wrap">')
	elseif compact then
		emit('<nav class="flex items-center gap-1" aria-label="Pagination">')
	else
		emit('<div class="flex items-center justify-center gap-2 mt-6">')
	end

	-- Prev button with label
	if p.Current > 1 then
		emit('  <a href="' .. h(pageURL(p, p.Current - 1)) .. '" class="' .. h(cls(BTN_BASE, C_BASE, compact)) .. '" aria-label="Previous page">← Prev</a>')
	else
		emit('  <span class="' .. h(cls(BTN_DISABLED, C_DISABLED, compact)) .. '" aria-label="Previous page">← Prev</span>')
	end

	-- Numbered pages with smart ellipsis
	local innerStart, innerEnd = 1, p.Total
	if p.Total > 7 then
		if p.Current <= 4 then
			innerStart, innerEnd = 1, 5
		elseif p.Current >= p.Total - 3 then
			innerStart, innerEnd = p.Total - 4, p.Total
		else
			innerStart, innerEnd = p.Current - 2, p.Current + 2
		end
	end

	if innerStart > 1 then
		emit('  <a href="' .. h(pageURL(p, 1)) .. '" class="' .. h(cls(BTN_NUM, C_NUM, compact)) .. '" title="Page 1">1</a>')
		if innerStart > 2 then
			emit('  <span class="' .. h(ELLIPSIS) .. '">…</span>')
		end
	end

	for n = innerStart, innerEnd do
		if n == p.Current then
			emit('  <span class="' .. h(cls(BTN_CURRENT, C_CURRENT, compact)) .. '" aria-current="page">' .. tostring(n) .. '</span>')
		else
			emit('  <a href="' .. h(pageURL(p, n)) .. '" class="' .. h(cls(BTN_NUM, C_NUM, compact)) .. '" title="Page ' .. tostring(n) .. '">' .. tostring(n) .. '</a>')
		end
	end

	if innerEnd < p.Total then
		if innerEnd < p.Total - 1 then
			emit('  <span class="' .. h(ELLIPSIS) .. '">…</span>')
		end
		emit('  <a href="' .. h(pageURL(p, p.Total)) .. '" class="' .. h(cls(BTN_NUM, C_NUM, compact)) .. '" title="Page ' .. tostring(p.Total) .. '">' .. tostring(p.Total) .. '</a>')
	end

	-- Next button with label, after the numbers: [← Prev] 1 2 … 4 5 [Next →]
	if p.Current < p.Total then
		emit('  <a href="' .. h(pageURL(p, p.Current + 1)) .. '" class="' .. h(cls(BTN_BASE, C_BASE, compact)) .. '" aria-label="Next page">Next →</a>')
	else
		emit('  <span class="' .. h(cls(BTN_DISABLED, C_DISABLED, compact)) .. '" aria-label="Next page">Next →</span>')
	end

	if compact then
		emit("</nav>")
	else
		emit("</div>")
	end
	if inner ~= "" then
		emit("</div>")
	end
	return table.concat(parts, "\n")
end
