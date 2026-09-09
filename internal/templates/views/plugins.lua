-- views/plugins.lua
-- Plugin manager page: list, install, toggle, TLS profiles, human verification.
-- Replaces views/plugins.jet.
-- Called as: plugins(data) -> string (body HTML only, layout wraps it)
-- data.Plugins: []PluginView (embedded database.Plugin + Loaded, VerifyURL, etc.)

return function(data)
	local plugins = data.Plugins or {}

	local pluginCards = ""
	if #plugins == 0 then
		pluginCards = '<div class="py-16 text-center text-neutral-500">No plugins installed</div>'
	else
		pluginCards = '<div class="space-y-3">'
		for _, p in ipairs(plugins) do
			local id = p.ID or ""
			local name = p.Name or ""
			local iconURL = p.IconURL or ""
			local siteURL = p.SiteURL or ""
			local version = p.Version or ""
			local isActive = p.IsActive or false
			local loaded = p.Loaded or false
			local needsHumanVerify = p.NeedsHumanVerify or false
			local verifyCookies = p.VerifyCookies or ""
			local verifyUserAgent = p.VerifyUserAgent or ""
			local verifyURL = p.VerifyURL or ""
			local pinnedProfile = p.PinnedProfile or ""
			local availableProfiles = p.AvailableProfiles or {}

			local cardHTML = '<div class="bg-neutral-900 rounded-lg p-4 border border-neutral-800">'

			-- Icon + name + profile badge
			local iconHTML
			if iconURL ~= "" then
				iconHTML = '<img src="'
					.. h(iconURL)
					.. '" alt="" class="h-10 w-10 rounded-md object-cover bg-neutral-800">'
			else
				local initials = getInitials(name)
				iconHTML = '<div class="h-10 w-10 rounded-md bg-indigo-500/15 text-indigo-400 flex items-center justify-center text-sm font-semibold shrink-0">'
					.. h(initials)
					.. "</div>"
			end

			local profileBadge = pinnedProfile ~= ""
					and ('<span class="text-[10px] px-1.5 py-0.5 rounded-full bg-sky-500/15 text-sky-400">' .. h(
						pinnedProfile
					) .. "</span>")
				or ""

			cardHTML = cardHTML
				.. '<div class="flex items-center gap-3">'
				.. iconHTML
				.. '<div class="min-w-0 flex-1">'
				.. '<div class="font-medium text-sm flex items-center gap-2">'
				.. h(name)
				.. profileBadge
				.. "</div>"
				.. '<div class="text-xs font-mono text-neutral-500 truncate">'
				.. h(id)
				.. "</div>"
				.. (siteURL ~= "" and ('<a href="' .. h(siteURL) .. '" target="_blank" rel="noopener" class="text-xs text-indigo-400 hover:text-indigo-300 truncate">' .. h(
					siteURL
				) .. "</a>") or "")
				.. "</div>"
				.. '<div class="flex items-center gap-2 shrink-0 flex-wrap">'

			-- Profile select
			cardHTML = cardHTML
				.. '<select data-plugin-profile="'
				.. h(id)
				.. '" class="bg-neutral-800 border border-neutral-700 rounded px-2 py-1 text-xs">'
				.. '<option value="">auto</option>'
			for _, profileName in ipairs(availableProfiles) do
				local sel = profileName == pinnedProfile and " selected" or ""
				cardHTML = cardHTML
					.. '<option value="'
					.. h(profileName)
					.. '"'
					.. sel
					.. ">"
					.. h(profileName)
					.. "</option>"
			end
			cardHTML = cardHTML .. "</select>"

			-- Test button
			cardHTML = cardHTML
				.. '<button type="button" onclick="testProfile(\''
				.. h(id)
				.. '\')" class="border border-neutral-700 text-neutral-400 hover:text-neutral-200 rounded px-2 py-1 text-xs">Test</button>'

			-- Reset button
			if pinnedProfile ~= "" then
				cardHTML = cardHTML
					.. '<button type="button" onclick="resetProfile(\''
					.. h(id)
					.. '\')" class="border border-red-800/50 text-red-400 hover:text-red-300 rounded px-2 py-1 text-xs">Reset</button>'
			end

			-- Version badge
			if version ~= "" then
				cardHTML = cardHTML
					.. '<span class="text-xs px-2 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">v'
					.. h(version)
					.. "</span>"
			end

			-- Active/Inactive
			cardHTML = cardHTML
				.. (
					isActive
						and '<span class="text-xs px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-400">Active</span>'
					or '<span class="text-xs px-2 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">Inactive</span>'
				)

			-- Loaded/Deferred (only show when deferred — loaded is the normal case, not noise)
			if not loaded then
				cardHTML = cardHTML
					.. '<span class="text-xs px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-400">Deferred</span>'
			end

			-- Toggle button
			cardHTML = cardHTML
				.. '<form method="post" action="/action/toggle-plugin/'
				.. h(id)
				.. '" data-confirm="'
				.. (isActive and "Deactivate" or "Activate")
				.. ' this plugin?">'
				.. '<button type="submit" class="border border-neutral-700 text-neutral-400 hover:text-neutral-200 rounded-md px-3 py-1.5 text-sm">'
				.. (isActive and "Deactivate" or "Activate")
				.. "</button></form></div></div>"

			-- Human verification section
			if needsHumanVerify then
				cardHTML = cardHTML
					.. '<div class="bg-neutral-800/50 border border-neutral-700 rounded-md mt-3 p-3">'
					.. '<div class="flex items-center justify-between gap-3 mb-2">'
					.. '<div class="text-sm font-medium text-neutral-300">Human Verification</div>'
					.. (#verifyCookies > 0 and '<span class="text-xs px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-400">Verified ✓</span>' or '<span class="text-xs px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-400">Not verified — site requires a browser challenge</span>')
					.. "</div>"
					.. (verifyURL ~= "" and ('<a href="' .. h(verifyURL) .. '" target="_blank" rel="noopener" class="inline-block bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-3 py-1.5 text-sm font-medium mb-3">Open verification page</a>') or "")
					.. '<form method="post" action="/action/save-verify/'
					.. h(id)
					.. '" class="space-y-2">'
					.. '<textarea name="cookies" rows="3" placeholder="cf_clearance=... or the full cookie header" class="w-full bg-neutral-800 border border-neutral-700 rounded-md px-3 py-2 text-sm font-mono">'
					.. h(verifyCookies)
					.. "</textarea>"
					.. '<input type="text" name="user_agent" value="'
					.. h(verifyUserAgent)
					.. '" placeholder="Browser User-Agent (optional — must match the browser that solved the challenge)" class="w-full bg-neutral-800 border border-neutral-700 rounded-md px-3 py-2 text-xs font-mono">'
					.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-1.5 text-sm font-medium">Save</button>'
					.. "</form>"
					.. '<details class="mt-3">'
					.. '<summary class="text-xs text-neutral-400 cursor-pointer hover:text-neutral-200">How to copy cookies</summary>'
					.. '<ol class="list-decimal list-inside text-xs text-neutral-400 space-y-1 mt-2 pl-1">'
					.. "<li>Open the verification page and complete the challenge.</li>"
					.. "<li>Press F12 → Network tab → refresh the page.</li>"
					.. "<li>Click the first request → Request Headers → find the <code>cookie:</code> line.</li>"
					.. "<li>Right-click → Copy value.</li>"
					.. "<li>Paste everything into the box above.</li>"
					.. '</div></details></div>'
			end

			cardHTML = cardHTML .. "</div>"
			pluginCards = pluginCards .. cardHTML
		end
		pluginCards = pluginCards .. "</div>"
	end

	local activeCount = 0
	for _, p in ipairs(plugins) do
		if p.IsActive then
				activeCount = activeCount + 1
			end
	end

	return '<div class="flex items-center gap-3 flex-wrap mb-6">'
		.. '<div class="shrink-0"><h1 class="text-xl font-semibold">Plugins</h1>'
		.. '<div class="text-xs text-neutral-500">'
		.. h(tostring(#plugins))
		.. " plugins · "
		.. h(tostring(activeCount))
		.. " active</div></div>"
		.. '<div class="flex-1 min-w-[180px] max-w-sm">'
		.. '<div class="relative">'
		.. '<svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-neutral-500 pointer-events-none" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path stroke-linecap="round" d="m21 21-4.35-4.35"/></svg>'
		.. '<input type="search" id="plugin-filter" placeholder="Filter plugins…" class="w-full bg-neutral-900 border border-neutral-700 rounded-md pl-9 pr-3 py-1.5 text-sm placeholder-neutral-500 focus:outline-none focus:border-indigo-500">'
		.. "</div></div>"
		.. '<form method="post" action="/action/install-plugin" enctype="multipart/form-data" class="flex items-center gap-2">'
		.. '<label for="plugin-file" class="cursor-pointer inline-flex items-center gap-1.5 border border-solid border-neutral-700 hover:border-indigo-500 hover:bg-neutral-800 rounded-md px-3 py-2 text-sm text-neutral-400 transition">'
		.. '<svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2M12 4v12m0-12-4 4m4-4 4 4"/></svg>Choose plugin…</label>'
		.. '<input type="file" id="plugin-file" name="file" accept=".wasm,.js,.lua,.so" class="sr-only">'
		.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Install</button>'
		.. "</form></div>"
		.. pluginCards
		.. [[<script>
document.querySelectorAll('input[type=file]').forEach(inp => {
  inp.addEventListener('change', function() {
    var label = this.previousElementSibling;
    if (label) label.textContent = this.files[0] ? this.files[0].name : 'Choose plugin…';
  });
});

document.querySelectorAll('form[data-confirm]').forEach(function(f) {
  f.addEventListener('submit', function(e) {
    if (!window.confirm(f.getAttribute('data-confirm'))) e.preventDefault();
  });
});

var pluginFilter = document.getElementById('plugin-filter');
if (pluginFilter) {
  pluginFilter.addEventListener('input', function() {
    var q = this.value.toLowerCase();
    document.querySelectorAll('.space-y-3 > div').forEach(function(card) {
      var text = card.textContent.toLowerCase();
      card.style.display = text.indexOf(q) !== -1 ? '' : 'none';
    });
  });
}

function testProfile(pluginID) {
  var sel = document.querySelector('[data-plugin-profile="' + pluginID + '"]');
  var profile = sel.value;
  if (!profile) { showToast('Select a profile first', true); return; }
  var btn = sel.nextElementSibling;
  setBtnState(btn, 'busy');
  fetch('/action/test-profile/' + pluginID, {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: 'profile=' + encodeURIComponent(profile)
  }).then(function(r) { return r.json(); })
    .then(function(d) {
      setBtnState(btn, d.ok ? 'ok' : 'fail');
      if (d.ok) {
        showToast('OK ' + d.status + ' via ' + profile, false);
      } else {
        showToast(profile + ' → ' + d.status + ' blocked', true);
      }
    })
    .catch(function() {
      setBtnState(btn, 'fail');
      showToast('Test request failed', true);
    });
}

function setBtnState(btn, state) {
  btn.disabled = (state === 'busy');
  btn.className = 'rounded px-2 py-1 text-xs border transition-colors duration-150';
  if (state === 'busy') {
    btn.className += ' bg-amber-600 border-amber-600 text-white';
    btn.textContent = 'Testing…';
  } else if (state === 'ok') {
    btn.className += ' bg-emerald-600 border-emerald-600 text-white';
    btn.textContent = 'OK ✓';
  } else {
    btn.className += ' bg-red-600 border-red-600 text-white';
    btn.textContent = 'Blocked ✗';
  }
}

function resetProfile(pluginID) {
  fetch('/action/reset-profile/' + pluginID, {method: 'POST'})
    .then(function(r) { return r.json(); })
    .then(function() { showToast('Profile reset to auto'); location.reload(); })
    .catch(function() { showToast('Reset failed', true); });
}
</script>
]]
end
