-- KaliScan plugin for goIsekai
-- Site: https://kaliscan.io
-- ABI contract version: 1 (matches pkg/types ContractVersion)

PLUGIN = {
    contract_version = 1,
    name = "KaliScan",
    verify_url = "https://kaliscan.io",
    needs_human_verify = false,
    thumb_ratio = 0.703
}

local util = require("util")

-- search_manga(arg) — arg is a JSON object: {"query":"...","page":1}
-- Returns: array of {id, title, cover_url} + "total" field
function search_manga(arg)
    local args = json.decode(arg)
    local query = args.query or ""
    local page = args.page or 1
    local resp = util.http_get("https://kaliscan.io/search?q=" .. util.url_encode(query) .. "&page=" .. tostring(page))
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    -- ABI: plain JSON array of manga ({id,title,cover_url}) — pagination is
    -- host-driven (?page=N), total is not part of the contract.
    return json.encode(util.parse_search(resp.body).results)
end

-- get_manga_detail(arg) — arg is a JSON-encoded plain string (e.g. '"104-love-shuttle"')
-- Returns: {id, title, author, description, cover_url, genres, status}
function get_manga_detail(arg)
    local manga_id = json.decode(arg) -- yields a plain string
    local resp = util.http_get("https://kaliscan.io/manga/" .. manga_id)
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    return json.encode(util.parse_manga_detail(resp.body, manga_id))
end

-- get_chapter_list(arg) — arg is a JSON-encoded plain string (e.g. '"104-love-shuttle"')
-- Returns: array of {id, number, title, uploaded_at}
function get_chapter_list(arg)
    local manga_id = json.decode(arg) -- yields a plain string
    local resp = util.http_get("https://kaliscan.io/manga/" .. manga_id)
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    return json.encode(util.parse_chapter_list(resp.body, manga_id))
end

-- get_page_list(arg) — arg is a JSON-encoded plain string (e.g. '"104-love-shuttle/chapter-98"')
-- Returns: array of {url}
function get_page_list(arg)
    local chapter_path = json.decode(arg) -- e.g. "SLUG:chapter-98"
    chapter_path = chapter_path:gsub(":", "/") -- restore real path
    -- Step 1: fetch the chapter page to extract the numeric chapterId
    local resp = util.http_get("https://kaliscan.io/manga/" .. chapter_path)
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    local chapter_id = string.match(resp.body, "chapterId%s*=%s*(%d+)")
    if not chapter_id then
        return json.encode({})
    end
    -- Step 2: fetch page images from the chapter server (requires Referer)
    local api_url = "https://kaliscan.io/service/backend/chapterServer/?server_id=1&chapter_id=" .. chapter_id
    local img_resp = util.http_get(api_url, {
        ["Referer"] = "https://kaliscan.io/manga/" .. chapter_path,
        ["X-Requested-With"] = "XMLHttpRequest"
    })
    if not img_resp or img_resp.status ~= 200 then
        return json.encode({})
    end
    return json.encode(util.parse_page_list(img_resp.body, "https://kaliscan.io/manga/" .. chapter_path))
end
