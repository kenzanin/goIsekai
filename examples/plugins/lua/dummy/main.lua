-- Dummy plugin for goIsekai
-- Reference Lua plugin with a hardcoded catalog. Use as a starting point
-- for real source plugins: replace http_request calls with live site data.

PLUGIN = {
    contract_version = 1,
    name = "Dummy",
    verify_url = "https://example.com",
    needs_human_verify = false,
    thumb_ratio = 0.70,
    search_page_size = 24
}

-- catalog: three dummy manga entries.
local catalog = {
    {
        id = "dummy-solo",
        title = "Solo Leveling Clone",
        cover_url = "https://picsum.photos/seed/dummy-solo/400/560",
        author = "Dummy Author",
        description = "A dummy isekai action series used as a plugin reference.",
        status = "ongoing",
        genres = {"action", "fantasy", "isekai"}
    },
    {
        id = "dummy-romance",
        title = "My Dummy Girlfriend",
        cover_url = "https://picsum.photos/seed/dummy-romance/400/560",
        author = "Dummy Author",
        description = "A dummy slice-of-life romance series.",
        status = "completed",
        genres = {"romance", "slice of life"}
    },
    {
        id = "dummy-horror",
        title = "The Dummy Below",
        cover_url = "https://picsum.photos/seed/dummy-horror/400/560",
        author = "Dummy Author",
        description = "A dummy horror/mystery series.",
        status = "ongoing",
        genres = {"horror", "mystery"}
    }
}

local function find_manga(id)
    for _, m in ipairs(catalog) do
        if m.id == id then return m end
    end
    return nil
end

local function chapters_for(manga_id)
    local chapters = {}
    for i = 1, 3 do
        table.insert(chapters, {
            id = manga_id .. "-ch" .. tostring(i),
            manga_id = manga_id,
            title = "Chapter " .. tostring(i),
            number = i,
            released_at = "2026-01-01T00:00:00Z",
            url = "https://example.com/" .. manga_id .. "/" .. tostring(i)
        })
    end
    return chapters
end

local function pages_for(chapter_id, n)
    local pages = {}
    for i = 0, n - 1 do
        table.insert(pages, {
            index = i,
            url = "https://picsum.photos/seed/" .. chapter_id .. "-" .. tostring(i) .. "/800/1200"
        })
    end
    return pages
end

-- search_manga(arg) — arg is a JSON object: {"query":"...","page":1}
function search_manga(arg)
    local args = json.decode(arg)
    local query = (args.query or ""):lower():gsub("^%s+", ""):gsub("%s+$", "")

    local results = {}
    for _, m in ipairs(catalog) do
        if query == "" or m.title:lower():find(query, 1, true) then
            table.insert(results, {
                id = m.id,
                title = m.title,
                cover_url = m.cover_url
            })
        end
    end
    return json.encode(results)
end

-- get_manga_detail(arg) — arg is a JSON-encoded plain string (e.g. '"dummy-solo"')
function get_manga_detail(arg)
    local manga_id = json.decode(arg)
    local m = find_manga(manga_id)
    if not m then return json.encode({}) end
    return json.encode(m)
end

-- get_chapter_list(arg) — arg is a JSON-encoded plain string (e.g. '"dummy-solo"')
function get_chapter_list(arg)
    local manga_id = json.decode(arg)
    if not find_manga(manga_id) then return json.encode({}) end
    return json.encode(chapters_for(manga_id))
end

-- get_page_list(arg) — arg is a JSON-encoded plain string (e.g. '"dummy-solo-ch1"')
function get_page_list(arg)
    local chapter_id = json.decode(arg)
    return json.encode(pages_for(chapter_id, 8))
end
