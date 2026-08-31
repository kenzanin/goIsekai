-- Minimal valid Lua plugin fixture for tests.
PLUGIN = {
    contract_version = 1,
    name = "LuaTest",
    verify_url = "https://example.com",
    needs_human_verify = true,
    thumb_ratio = 0.7
}

local sibling = require("sibling")

function search_manga(arg)
    local f = json.decode(arg)
    return json.encode({{id = "L1", title = "Lua " .. f.query, cover_url = sibling.cover()}})
end

function get_manga_detail(arg)
    local id = json.decode(arg)
    return json.encode({id = id, title = "Detail " .. id, cover_url = sibling.cover()})
end

function get_chapter_list(arg)
    local id = json.decode(arg)
    return json.encode({{id = id .. "/c1", manga_id = id, title = "Ch 1", chapter_num = 1.0}})
end

function get_page_list(arg)
    local cid = json.decode(arg)
    return json.encode({{index = 0, url = "https://example.com/img/1.png"}})
end
