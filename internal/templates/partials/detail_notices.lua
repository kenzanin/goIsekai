--  partials/detail_notices.lua
--  Top of the manga detail page: the Back button plus the challenge and
--  cached-data banners. Returned as one HTML string.
--  data.Challenge, data.CachedData
return function(data)
	local challenge = data.Challenge or false
	local cachedData = data.CachedData or false

	-- Back button
	local body = [[<div class="mb-4">
    <button onclick="window.backToList ? backToList() : (window.location.href = '/view/library')" class="inline-flex items-center gap-1.5 text-sm text-neutral-400 hover:text-neutral-200 transition" aria-label="Back">
        <svg xmlns="http://www.w3.org/2000/svg" class="size-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m15 18-6-6 6-6"/></svg>
        Back
    </button></div>
]]

	-- Challenge warning
	if challenge then
		body = body
			.. '<div class="bg-amber-500/15 border border-amber-500/40 text-amber-300 rounded-lg p-4 mb-6">This site needs human verification — go to <a href="/view/plugins" class="underline">Plugins → Human Verification</a> to paste your session cookies.</div>'
	end

	-- Cached data offline notice
	if cachedData then
		body = body
			.. '<div class="bg-blue-500/15 border border-blue-500/40 text-blue-300 rounded-lg p-4 mb-6">Showing cached data — offline or plugin unavailable.</div>'
	end

	return body
end
