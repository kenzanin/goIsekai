-- Sibling module: require("sibling") must resolve inside the plugin folder.
local sibling = {}
function sibling.cover() return "https://example.com/cover.jpg" end
return sibling
