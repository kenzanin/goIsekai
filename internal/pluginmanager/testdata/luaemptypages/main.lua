-- Lua fixture that always returns an empty page list, simulating a plugin
-- whose upstream is unreachable (offline). Also needs a search function so it
-- is a discoverable, valid plugin.
PLUGIN = {
    contract_version = 1,
    name = "LuaEmptyPages",
}

function search_manga(arg)
    return host.json.encode({})
end

function get_page_list(arg)
    return host.json.encode({})
end