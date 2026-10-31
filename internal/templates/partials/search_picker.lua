--  partials/search_picker.lua
--  Plugin picker for the search page: the trigger button, the dropdown of
--  active plugins, and the inline script that syncs the hidden native select.
--  Returns the two HTML strings plus the selected plugin name, which the view
--  also stamps onto the hidden input as data-pname.
--  data.Plugins, data.PluginID
return function(data)
	local plugins = data.Plugins or {}
	local pluginID = data.PluginID or ""

	-- Plugin picker (button + hidden native select keeps form semantics)
	local selIcon, selName = "", pluginID
	for _, p in ipairs(plugins) do
		if p.ID == pluginID then
			selIcon = p.IconURL or ""
			selName = p.Name
			break
		end
	end
	local pickerBtn = '<button type="button" id="pluginPickerBtn" class="flex items-center gap-2 bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm min-w-[180px] text-left">'
		.. (selIcon ~= "" and ('<img src="' .. h(selIcon) .. '" alt="" class="w-5 h-5 rounded object-cover flex-shrink-0">') or "")
		.. '<span id="pluginPickerLabel" class="truncate">'
		.. h(selName)
		.. "</span></button>"
	local pickerDrop =
		'<div id="pluginPickerDrop" class="hidden absolute z-20 mt-1 w-56 bg-neutral-900 border border-neutral-700 rounded-md shadow-lg py-1 max-h-64 overflow-auto">'
	for _, p in ipairs(plugins) do
		if p.IsActive then
			local ic = p.IconURL or ""
			pickerDrop = pickerDrop
				.. '<button type="button" data-pick="'
				.. h(p.ID)
				.. '" data-pname="'
				.. h(p.Name)
				.. '" class="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-neutral-800">'
				.. (ic ~= "" and ('<img src="' .. h(ic) .. '" alt="" class="w-5 h-5 rounded object-cover flex-shrink-0">') or '<span class="w-5 h-5 rounded bg-neutral-700 flex-shrink-0"></span>')
				.. h(p.Name)
				.. "</button>"
		end
	end
	pickerDrop = pickerDrop .. "</div>"

	return pickerBtn, pickerDrop, selName
end
