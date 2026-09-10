-- views/plugins.lua
-- Plugin manager page: list, install, toggle, TLS profiles, human verification.
-- Replaces views/plugins.jet.
-- Called as: plugins(data) -> string (body HTML only, layout wraps it)
-- data.Plugins: []PluginView (embedded database.Plugin + Loaded, VerifyURL, etc.)

local pluginCards = require("partials.plugin_cards")

return function(data)
	local plugins = data.Plugins or {}

	local cardsHTML = pluginCards({ Plugins = plugins })

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
		.. cardsHTML
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
