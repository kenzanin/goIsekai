-- Lua plugin fixture that reports failures the Lua way: nil plus a reason.
-- The reason is what lets a broken chapter be told apart from an empty one.
PLUGIN = {
    contract_version = 1,
    name = "LuaError",
    verify_url = "https://example.com",
}

function search_manga(arg)
    return host.json.encode({})
end

function get_manga_detail(arg)
    return host.json.encode({id = "E1", title = "Detail"})
end

function get_chapter_list(arg)
    return host.json.encode({})
end

-- Returns nil plus a reason instead of an empty list.
function get_page_list(arg)
    return nil, "no_pages: chapter 1"
end
