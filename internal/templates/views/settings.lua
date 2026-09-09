-- views/settings.lua
-- Settings page. Replaces views/settings.jet.
-- Called as: settings(data) -> string (body HTML only, layout wraps it)
-- data.Config: *config.Config, data.Path: string, data.CacheBytes: int64

return function(data)
	local cfg = data.Config
	local path = data.Path or ""
	local cacheBytes = data.CacheBytes or 0

	return [[<h1 class="text-xl font-semibold mb-1">Settings</h1>
<p class="text-xs text-neutral-500 mb-6">Config: <code class="text-xs bg-neutral-800 rounded px-1.5 py-0.5">]] .. h(
		path
	) .. [[</code></p>]] .. (cfg and [[<form method="post" action="/action/save-settings" class="space-y-4 max-w-2xl mx-auto">
    <div class="border border-neutral-800 rounded-lg p-4">
        <h2 class="text-sm font-medium text-neutral-300 mb-3">Server</h2>
        <div class="space-y-4">
            <div>
                <label for="title" class="block text-sm font-medium mb-1">Title</label>
                <input id="title" name="title" type="text" value="]] .. h(cfg.Title or "") .. [[" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">
            </div>
            <div class="grid grid-cols-2 gap-4">
                <div>
                    <label for="host" class="block text-sm font-medium mb-1">Host</label>
                    <input id="host" name="host" type="text" value="]] .. h(cfg.Host or "") .. [[" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">
                </div>
                <div>
                    <label for="port" class="block text-sm font-medium mb-1">Port</label>
                    <input id="port" name="port" type="number" value="]] .. h(tostring(cfg.Port or "")) .. [[" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">
                </div>
            </div>
            <div>
                <label for="log_level" class="block text-sm font-medium mb-1">Log Level</label>
                <select id="log_level" name="log_level" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">
                    <option value="debug"]] .. (cfg.LogLevel == "debug" and " selected" or "") .. [[>debug</option>
                    <option value="info"]] .. (cfg.LogLevel == "info" and " selected" or "") .. [[>info</option>
                    <option value="warning"]] .. (cfg.LogLevel == "warning" and " selected" or "") .. [[>warning</option>
                </select>
            </div>
        </div>
    </div>

    <div class="border border-neutral-800 rounded-lg p-4">
        <h2 class="text-sm font-medium text-neutral-300 mb-3">HTTP Client</h2>
        <div class="space-y-4">
            <div>
                <label for="user_agent" class="block text-sm font-medium mb-1">User Agent</label>
                <input id="user_agent" name="user_agent" type="text" value="]] .. h(cfg.UserAgent or "") .. [[" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">
            </div>
            <div>
                <label for="accept_language" class="block text-sm font-medium mb-1">Accept Language</label>
                <input id="accept_language" name="accept_language" type="text" value="]] .. h(
		cfg.AcceptLanguage or ""
	) .. [[" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">
            </div>
            <div>
                <label for="referer" class="block text-sm font-medium mb-1">Referer</label>
                <input id="referer" name="referer" type="text" value="]] .. h(cfg.Referer or "") .. [[" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">
            </div>
        </div>
    </div>

    <div class="border border-neutral-800 rounded-lg p-4">
        <h2 class="text-sm font-medium text-neutral-300 mb-3">Reader</h2>
        <div class="space-y-4">
            <div>
                <label for="read_ahead" class="block text-sm font-medium mb-1">Read Ahead (pages to prefetch)</label>
                <input id="read_ahead" name="read_ahead" type="number" min="0" max="10" value="3" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">
                <span class="text-xs text-neutral-500">(stored in this browser)</span>
            </div>
        </div>
    </div>

    <button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Save</button>
</form>

<div class="max-w-2xl mx-auto mt-10 p-4 border border-neutral-800 rounded-lg">
    <h2 class="text-sm font-semibold mb-1">Image Cache</h2>
    <p class="text-xs text-neutral-400 mb-3">Cached images: ]] .. h(formatBytes(cacheBytes)) .. [["
    <form method="post" action="/action/clear-cache-all" data-confirm="Delete ALL cached images?">
        <button type="submit" class="bg-neutral-700 hover:bg-red-600 text-white rounded-md px-4 py-2 text-sm font-medium">🗑 Clear all cached images</button>
    </form>
</div>

<script>
(function () {
  var ra = document.getElementById('read_ahead');
  ra.value = localStorage.getItem('gi_readAhead') || '3';
  ra.closest('form').addEventListener('submit', function () {
    localStorage.setItem('gi_readAhead', ra.value);
  });
})();
</script>]] or [[<div class="py-16 text-center text-neutral-500">Failed to load configuration</div>]])
end
