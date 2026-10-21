-- Lua fixture that always returns an empty page list, simulating a plugin
-- whose upstream is unreachable (offline). It declares the full source-plugin
-- ABI so it also loads as an ACTIVE plugin: an inactive one takes the offline
-- path and never reaches the plugin at all.
PLUGIN = {
    contract_version = 1,
    name = "LuaEmptyPages",
}

function search_manga(arg)
    return host.json.encode({})
end

function get_manga_detail(arg)
    return host.json.encode({id = host.json.decode(arg)})
end

function get_chapter_list(arg)
    return host.json.encode({})
end

function get_page_list(arg)
    return host.json.encode({})
end
