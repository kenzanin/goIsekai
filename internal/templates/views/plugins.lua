-- views/plugins.lua
-- Plugin manager page: list, install, toggle, TLS profiles, human verification.
-- Replaces views/plugins.jet.
-- Called as: plugins(data) -> string (body HTML only, layout wraps it)
-- data.Plugins: []PluginView (embedded database.Plugin + Loaded, VerifyURL, etc.)

return function(data)
    local plugins = data.Plugins or {}
    local parts = {}
    local function emit(s) parts[#parts + 1] = s end

    -- Header + install form
    emit('<div class="flex items-center justify-between mb-6">')
    emit('    <h1 class="text-xl font-semibold">Plugins</h1>')
    emit('    <form method="post" action="/action/install-plugin" enctype="multipart/form-data" class="flex items-center gap-2">')
    emit('        <label for="plugin-file" class="cursor-pointer border border-dashed border-neutral-700 rounded-md px-3 py-2 text-sm text-neutral-400 hover:border-indigo-500">Choose plugin…</label>')
    emit('        <input type="file" id="plugin-file" name="file" accept=".wasm,.js,.lua,.so" class="sr-only">')
    emit('        <button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Install</button>')
    emit('    </form>')
    emit('</div>')

    if #plugins == 0 then
        emit('<div class="py-16 text-center text-neutral-500">No plugins installed</div>')
    else
        emit('<div class="space-y-3">')
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

            -- Main plugin card (single card with all controls)
            emit('<div class="bg-neutral-900 rounded-lg p-4 border border-neutral-800">')

            -- Top row: icon + name + profile badge + right-side badges/toggle
            emit('  <div class="flex items-center gap-3">')
            if iconURL ~= "" then
                emit('    <img src="' .. h(iconURL) .. '" alt="" class="h-10 w-10 rounded-md object-cover bg-neutral-800">')
            else
                local firstChar = string.sub(name, 1, 1)
                emit('    <div class="h-10 w-10 rounded-md bg-neutral-800 flex items-center justify-center text-sm font-semibold text-neutral-400 shrink-0">' .. h(firstChar) .. '</div>')
            end
            emit('    <div class="min-w-0 flex-1">')
            emit('      <div class="font-medium text-sm flex items-center gap-2">')
            emit('        ' .. h(name))
            if pinnedProfile ~= "" then
                emit('        <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-sky-500/15 text-sky-400">' .. h(pinnedProfile) .. '</span>')
            end
            emit('      </div>')
            emit('      <div class="text-xs font-mono text-neutral-500 truncate">' .. h(id) .. '</div>')
            if siteURL ~= "" then
                emit('      <a href="' .. h(siteURL) .. '" target="_blank" rel="noopener" class="text-xs text-indigo-400 hover:text-indigo-300 truncate">' .. h(siteURL) .. '</a>')
            end
            emit('    </div>')

            -- Right side: profile select + badges + toggle
            emit('    <div class="flex items-center gap-2 shrink-0 flex-wrap">')
            emit('      <select data-plugin-profile="' .. h(id) .. '" class="bg-neutral-800 border border-neutral-700 rounded px-2 py-1 text-xs">')
            emit('        <option value="">auto</option>')
            for _, profileName in ipairs(availableProfiles) do
                local sel = profileName == pinnedProfile and " selected" or ""
                emit('        <option value="' .. h(profileName) .. '"' .. sel .. '>' .. h(profileName) .. '</option>')
            end
            emit('      </select>')
            emit('      <button type="button" onclick="testProfile(\'' .. h(id) .. '\')" class="border border-neutral-700 text-neutral-400 hover:text-neutral-200 rounded px-2 py-1 text-xs">Test</button>')
            if pinnedProfile ~= "" then
                emit('      <button type="button" onclick="resetProfile(\'' .. h(id) .. '\')" class="border border-red-800/50 text-red-400 hover:text-red-300 rounded px-2 py-1 text-xs">Reset</button>')
            end
            if version ~= "" then
                emit('      <span class="text-xs px-2 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">v' .. h(version) .. '</span>')
            end
            if isActive then
                emit('      <span class="text-xs px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-400">Active</span>')
            else
                emit('      <span class="text-xs px-2 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">Inactive</span>')
            end
            if loaded then
                emit('      <span class="text-xs px-2 py-0.5 rounded-full bg-sky-500/15 text-sky-400">Loaded</span>')
            else
                emit('      <span class="text-xs px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-400">Deferred</span>')
            end
            emit('      <form method="post" action="/action/toggle-plugin/' .. h(id) .. '">')
            emit('        <button type="submit" class="border border-neutral-700 text-neutral-400 hover:text-neutral-200 rounded-md px-3 py-1.5 text-sm">')
            if isActive then emit('Deactivate') else emit('Activate') end
            emit('        </button>')
            emit('      </form>')
            emit('    </div>')
            emit('  </div>')

            emit('</div>')

            -- Human verification section (separate card, only when needed)
            if needsHumanVerify then
                emit('<div class="bg-neutral-900 rounded-lg p-4 border border-neutral-800">')
                emit('  <div class="flex items-center justify-between gap-3 mb-3">')
                emit('    <div class="text-sm font-medium">Human Verification</div>')
                if #verifyCookies > 0 then
                    emit('    <span class="text-xs px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-400">Verified ✓</span>')
                else
                    emit('    <span class="text-xs px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-400">Not verified — site requires a browser challenge</span>')
                end
                emit('  </div>')
                if #verifyURL > 0 then
                    emit('  <a href="' .. h(verifyURL) .. '" target="_blank" rel="noopener" class="inline-block bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-3 py-1.5 text-sm font-medium mb-3">Open verification page</a>')
                end
                emit('  <form method="post" action="/action/save-verify/' .. h(id) .. '" class="space-y-2">')
                emit('    <textarea name="cookies" rows="3" placeholder="cf_clearance=... or the full cookie header" class="w-full bg-neutral-800 border border-neutral-700 rounded-md px-3 py-2 text-sm font-mono">' .. h(verifyCookies) .. '</textarea>')
                emit('    <input type="text" name="user_agent" value="' .. h(verifyUserAgent) .. '" placeholder="Browser User-Agent (optional — must match the browser that solved the challenge)" class="w-full bg-neutral-800 border border-neutral-700 rounded-md px-3 py-2 text-xs font-mono">')
                emit('    <button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-1.5 text-sm font-medium">Save</button>')
                emit('  </form>')
                emit('  <details class="mt-3">')
                emit('    <summary class="text-xs text-neutral-400 cursor-pointer hover:text-neutral-200">How to copy cookies</summary>')
                emit('    <ol class="list-decimal list-inside text-xs text-neutral-400 space-y-1 mt-2 pl-1">')
                emit('      <li>Open the verification page and complete the challenge.</li>')
                emit('      <li>Press F12 → Network tab → refresh the page.</li>')
                emit('      <li>Click the first request → Request Headers → find the <code>cookie:</code> line.</li>')
                emit('      <li>Right-click → Copy value.</li>')
                emit('      <li>Paste everything into the box above.</li>')
                emit('    </ol>')
                emit('  </details>')
                emit('</div>')
            end
        end
        emit('</div>')
    end

    -- JavaScript
    emit('<script>')
    emit('document.querySelectorAll(\'input[type=file]\').forEach(inp => {')
    emit('  inp.addEventListener(\'change\', function() {')
    emit('    var label = this.previousElementSibling;')
    emit('    if (label) label.textContent = this.files[0] ? this.files[0].name : \'Choose .wasm…\';')
    emit('  });')
    emit('});')
    emit('')
    emit('function testProfile(pluginID) {')
    emit('  var sel = document.querySelector(\'[data-plugin-profile="\' + pluginID + \'"]\');')
    emit('  var profile = sel.value;')
    emit('  if (!profile) { showToast(\'Select a profile first\', true); return; }')
    emit('  var btn = sel.nextElementSibling;')
    emit('  btn.disabled = true;')
    emit('  btn.textContent = \'Testing…\';')
    emit('  fetch(\'/action/test-profile/\' + pluginID, {')
    emit('    method: \'POST\',')
    emit('    headers: {\'Content-Type\': \'application/x-www-form-urlencoded\'},')
    emit('    body: \'profile=\' + encodeURIComponent(profile)')
    emit('  }).then(function(r) { return r.json(); })')
    emit('    .then(function(d) {')
    emit('      btn.disabled = false;')
    emit('      btn.textContent = \'Test\';')
    emit('      if (d.ok) {')
    emit('        showToast(\'OK \' + d.status + \' via \' + profile, false);')
    emit('      } else {')
    emit('        showToast(profile + \' → \' + d.status + \' blocked\', true);')
    emit('      }')
    emit('    })')
    emit('    .catch(function() {')
    emit('      btn.disabled = false;')
    emit('      btn.textContent = \'Test\';')
    emit('      showToast(\'Test request failed\', true);')
    emit('    });')
    emit('}')
    emit('')
    emit('function resetProfile(pluginID) {')
    emit('  fetch(\'/action/reset-profile/\' + pluginID, {method: \'POST\'})')
    emit('    .then(function(r) { return r.json(); })')
    emit('    .then(function() { showToast(\'Profile reset to auto\'); location.reload(); })')
    emit('    .catch(function() { showToast(\'Reset failed\', true); });')
    emit('}')
    emit('</script>')

    return table.concat(parts, '\n')
end
