-- partials/plugin_cards.lua
-- Plugin manager: one card per plugin (icon, TLS profile, badges, verification).
-- Called as: plugin_cards(data) -> string
-- data.Plugins: []PluginView

-- The runtime a plugin ships in, shown as a badge so a source's kind is visible
-- at a glance. The classes are ones the committed Tailwind build already
-- contains, so the badge renders without regenerating the stylesheet.

local pluginCard = require("partials.plugin_card")

local KIND_BADGE = {
	lua = { label = "Lua", class = "bg-indigo-500/15 text-indigo-400", title = "Lua plugin (main.lua)" },
	js = { label = "JS", class = "bg-amber-500/15 text-amber-400", title = "JavaScript plugin (main.js)" },
	go = { label = "Native", class = "bg-sky-500/15 text-sky-400", title = "Native plugin, shared library (.so/.dll) — C/C++, no cgo" },
	yaegi = { label = "Yaegi", class = "bg-emerald-500/15 text-emerald-400", title = "Yaegi-interpreted Go plugin" },
	wasm = { label = "Wasm", class = "bg-blue-500/15 text-sky-400", title = "WebAssembly plugin (main.wasm)" },
}

local pluginCard = require("partials.plugin_card")

return function(data)
	local plugins = data.Plugins or {}

	local pluginCards = ""
	if #plugins == 0 then
		pluginCards = '<div class="py-16 text-center text-neutral-500">No plugins installed</div>'
	else
		pluginCards = '<div class="space-y-3">'
		for _, p in ipairs(plugins) do
			local cardHTML = pluginCard({
				csrf_token = data.csrf_token or "",
				Plugin = p,
			})
			pluginCards = pluginCards .. cardHTML
		end
		pluginCards = pluginCards .. "</div>"
	end

	return pluginCards
end
