-- Fixture that reports failure with a host error code plus detail, the form the
-- ABI defines. The reader must see the host's wording, not this plugin's text.
PLUGIN = {
    contract_version = 1,
    name = "LuaCode",
    verify_url = "https://example.com",
}

function search_manga(arg)
    return host.json.encode({})
end

function get_manga_detail(arg)
    return host.json.encode({id = "C1", title = "Detail"})
end

function get_chapter_list(arg)
    return host.json.encode({})
end

function get_page_list(arg)
    return nil, "no_pages: chapter 7"
end
