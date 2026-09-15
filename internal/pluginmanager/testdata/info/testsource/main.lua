-- Fixture info script: declares an enrichment provider and serves canned
-- items so the host-side test stays offline. Deliberately implements NONE of
-- the source ABI (no search_manga / get_manga_detail / get_chapter_list /
-- get_page_list): an info script must load without them.

PLUGIN = {
    contract_version = 1,
    name = "Test Info",
    site_url = "https://example.test",
    enrichment_providers = {
        { id = "testsource", name = "Test Source", kinds = { "titles", "authors" } },
    },
}

function getEnrichment(arg)
    local req = host.json.decode(arg)
    if req.kind == "authors" then
        return host.json.encode({ { value = "Author A", url = "https://example.test/a" } })
    end
    if req.kind == "titles" then
        return host.json.encode({ { value = "Alt Title", url = "https://example.test/t" } })
    end
    return host.json.encode({})
end
