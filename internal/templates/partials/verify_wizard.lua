-- partials/verify_wizard.lua
-- Human verification wizard modal: shown when a plugin is blocked by a
-- JS/cookie challenge. Shared by views/detail.lua and views/search.lua so the
-- two copies cannot drift apart again (they did: unescaped placeholder,
-- ignored UA fallback, and unquoted onclick IDs in the detail copy).
-- Called as: verify_wizard(data) -> string (fragment, "" when hidden)
-- data.PluginID, data.MangaID ("" on search; drives the save URL + query keep),
-- data.NeedsHumanVerify, data.VerifyURL, data.VerifyCookies,
-- data.VerifyUserAgent, data.Challenge, data.csrf_token

return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local needsHumanVerify = data.NeedsHumanVerify or false
	local verifyURL = data.VerifyURL or ""
	local verifyCookies = data.VerifyCookies or ""
	local verifyUserAgent = data.VerifyUserAgent or ""
	local challenge = data.Challenge or false

	-- Show only when actually blocked (challenge) or no cookies saved yet, so
	-- saved cookies suppress the modal and the page can load normally.
	if not (needsHumanVerify and verifyURL ~= "" and (challenge or verifyCookies == "")) then
		return ""
	end

	local placeholder = verifyCookies ~= "" and verifyCookies or "cf_clearance=...; session=..."
	local useragent = verifyUserAgent ~= "" and verifyUserAgent
		or "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

	-- Detail passes a manga ID (save goes to /save-verify/{pid}/{mid}); search
	-- passes "" (save keeps the query string). One function, one quoting style.
	local saveCall = "saveVerifyModal('" .. h(pluginID) .. "')"
	if mangaID ~= "" then
		saveCall = "saveVerifyModal('" .. h(pluginID) .. "', '" .. h(mangaID) .. "')"
	end

	return '<div id="verify-modal" class="fixed inset-0 z-50 flex items-center justify-center" style="display:none">'
		.. '<div class="absolute inset-0 bg-black/70" onclick="this.parentElement.classList.add(\'hidden\')"></div>'
		.. '<div class="relative bg-neutral-900 border border-neutral-700 rounded-xl p-4 max-w-lg max-h-[80vh] overflow-y-auto">'
		.. '<div class="space-y-4">'
		.. '<div class="bg-amber-500/10 border border-amber-600/50 rounded-lg p-3">'
		.. '<h3 class="text-sm font-semibold text-amber-200 mb-2">⚠️ Plugin blocked by bot verification</h3>'
		.. '<p class="text-xs text-amber-200/80 mb-2">This site requires manual verification. Open the link below in your browser, solve the challenge, then paste cookies below — either as name=value pairs or a Cookie-Editor JSON export.</p>'
		.. '<a href="'
		.. h(verifyURL)
		.. '" target="_blank" rel="noopener" class="text-xs text-amber-400 hover:text-amber-300 hover:underline break-all">'
		.. h(verifyURL)
		.. "</a>"
		.. "</div>"
		.. '<div class="space-y-2">'
		.. '<label class="block text-xs font-medium text-neutral-300">Saved cookies for this plugin</label>'
		.. '<textarea id="verify-cookies" rows="4" class="w-full bg-neutral-950 border border-neutral-700 rounded-lg p-2 text-xs text-neutral-200 font-mono" placeholder="'
		.. h(placeholder)
		.. '" autofocus onfocus="this.scrollTop=0">'
		.. h(verifyCookies)
		.. "</textarea>"
		.. '<label class="block text-xs font-medium text-neutral-300">Saved browser user-agent</label>'
		.. '<input type="text" id="verify-useragent" value="'
		.. h(useragent)
		.. '" class="w-full bg-neutral-950 border border-neutral-700 rounded-lg p-2 text-xs text-neutral-200">'
		.. "</div>"
		.. '<div class="flex gap-2">'
		.. '<button type="button" onclick="'
		.. saveCall
		.. '" class="bg-amber-600 hover:bg-amber-500 text-white rounded-md px-4 py-2 text-sm font-medium">Save & retry</button>'
		.. '<button type="button" onclick="this.closest(\'#verify-modal\').classList.add(\'hidden\')" class="border border-neutral-600 text-neutral-300 hover:bg-neutral-800 rounded-md px-4 py-2 text-sm">Cancel</button>'
		.. "</div>"
		.. "</div>"
		.. "</div></div>"
		.. "<script>"
		.. "document.getElementById('verify-modal').style.display='';"
		.. "function saveVerifyModal(pid, mid) {"
		.. "  var cookies = document.getElementById('verify-cookies').value;"
		.. "  var ua = document.getElementById('verify-useragent').value;"
		.. "  var form = new URLSearchParams();"
		.. "  form.append('cookies', cookies);"
		.. "  form.append('user_agent', ua);"
		.. "  var url = '/action/save-verify/' + encodeURIComponent(pid);"
		.. "  if (mid) { url += '/' + encodeURIComponent(mid); }"
		.. "  else { var qs = new URLSearchParams(window.location.search).toString(); if (qs) { url += '?' + qs; } }"
		.. "  fetch(url, {"
		.. "    method: 'POST',"
		.. "    headers: {"
		.. "      'Content-Type': 'application/x-www-form-urlencoded',"
		.. "      'X-CSRF-Token': '"
		.. h(data.csrf_token or "")
		.. "',"
		.. "    },"
		.. "    body: form.toString()"
		.. "  }).then(r => {"
		.. "    if (r.status === 303 || r.ok) {"
		.. "      document.getElementById('verify-modal').classList.add('hidden');"
		.. "      var cm = document.getElementById('cover-modal'); if (cm) { cm.classList.add('hidden'); }"
		.. "      location.reload();"
		.. "    } else {"
		.. "      alert('Save failed: ' + r.status);"
		.. "    }"
		.. "  }).catch(e => { alert('Save failed: ' + e.message); });"
		.. "}"
		.. "</script>"
end
