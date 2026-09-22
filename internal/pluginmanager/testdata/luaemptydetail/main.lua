-- Lua plugin fixture that returns EMPTY detail and chapter list with nil
-- error — simulates transient upstream failure (rate limit / offline).
PLUGIN = {
    contract_version = 1,
    name = "LuaEmptyDetail",
    verify_url = "https://example.com",
    needs_human_verify = true,
    thumb_ratio = 0.7
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
