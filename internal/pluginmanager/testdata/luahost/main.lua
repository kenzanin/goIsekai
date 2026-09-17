PLUGIN = { contract_version = 1, name = "Lua host natives" }

-- regex_payload reports host.regex's output in a fixed string so the
-- cross-runtime test compares Lua and JS literally. Patterns are written as
-- long strings, so a backslash reaches the engine unescaped.
local function regex_payload()
  local rows = {}
  for _, row in ipairs(host.regex.find_all(
      '<a href="/m/a/" title="A"><a href="/m/b/" title="B">',
      [[href="([^"]+)" title="([^"]+)"]]
    )) do
    rows[#rows + 1] = table.concat(row, ",")
  end
  return table.concat({
    table.concat({host.regex.find("chapterId = 42", [[chapterId\s*=\s*(\d+)]])}),
    table.concat({host.regex.find("<h1>Solo Leveling</h1>", [[<h1>([^<]+)</h1>]])}),
    tostring(host.regex.match("https://x/1.jpg", [[\.(?:jpg|png)$]])),
    tostring(host.regex.match("https://x/1.webp", [[\.(?:jpg|png)$]])),
    host.regex.replace("a  b", [[\s+]], " "),
    table.concat(rows, ";"),
  }, ",")
end

local function payload()
  return table.concat({
    host.text.url_encode("a b"),
    host.text.url_decode("a%20b"),
    host.text.html_decode("&amp;"),
    host.text.strip_html("<p>hi</p>"),
    host.text.strip_markdown("**bold** [x](y)"),
    host.text.titlecase("abc"),
    host.codecs.base64_encode("hi"),
    host.codecs.base64_decode("aGk="),
    host.codecs.base64url_encode("a?b"),
    host.codecs.hex_encode("hi"),
    host.codecs.hex_decode("6869"),
    host.crypto.sha256_hex("abc"),
    host.crypto.md5_hex("abc"),
    host.crypto.hmac_sha256_hex("key", "The quick brown fox jumps over the lazy dog"),
    host.crypto.xor("6162", "6364"),
    host.crypto.utf8_hex("hi"),
    host.codecs.b64_decode_hex("aGk="),
    host.codecs.b64url_encode_hex("6869"),
    host.codecs.b64url_decode_hex("aGk"),
    host.crypto.vrf_sign("/titles", {page = "2"}, {
        {iv = 0x5A, key = "aw==", tbl = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0+P0BBQkNERUZHSElKS0xNTk9QUVJTVFVWV1hZWltcXV5fYGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6e3x9fn+AgYKDhIWGh4iJiouMjY6PkJGSk5SVlpeYmZqbnJ2en6ChoqOkpaanqKmqq6ytrq+wsbKztLW2t7i5uru8vb6/wMHCw8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t/g4eLj5OXm5+jp6uvs7e7v8PHy8/T19vf4+fr7/P3+/w=="},
        {iv = 0x5A, key = "aw==", tbl = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0+P0BBQkNERUZHSElKS0xNTk9QUVJTVFVWV1hZWltcXV5fYGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6e3x9fn+AgYKDhIWGh4iJiouMjY6PkJGSk5SVlpeYmZqbnJ2en6ChoqOkpaanqKmqq6ytrq+wsbKztLW2t7i5uru8vb6/wMHCw8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t/g4eLj5OXm5+jp6uvs7e7v8PHy8/T19vf4+fr7/P3+/w=="},
        {iv = 0x5A, key = "aw==", tbl = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0+P0BBQkNERUZHSElKS0xNTk9QUVJTVFVWV1hZWltcXV5fYGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6e3x9fn+AgYKDhIWGh4iJiouMjY6PkJGSk5SVlpeYmZqbnJ2en6ChoqOkpaanqKmqq6ytrq+wsbKztLW2t7i5uru8vb6/wMHCw8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t/g4eLj5OXm5+jp6uvs7e7v8PHy8/T19vf4+fr7/P3+/w=="},
    }),
    host.text.chapter_num("Vol. 3 Ch. 12.5"),
    host.text.chapter_num(7.5),
    host.text.json_blob('<div>{"a":[1,2]}</div>', ""),
    host.text.date_to_iso("2026-01-02"),
    host.json.encode(host.json.decode('{"b":"x","a":1}')),
    host.json.encode(host.json.decode("[1,2,3]")),
    host.json.encode({a = 1, b = "x"}),
    regex_payload(),
    -- get_body reports every failure mode as nil, transport errors included, so
    -- a plugin can drop the per-call status guard.
    tostring(host.http.get_body("http://127.0.0.1:1/") == nil),
  }, "|")
end

-- json_errors joins the failure texts of the shared JSON natives so the
-- cross-runtime test can compare them literally against the JS runtime.
local function json_errors()
  local _, badjson = host.json.decode("{bad")
  local _, badarg = host.json.decode(7)
  local _, badencode = host.json.encode(function() end)
  return table.concat({
    badjson or "no error",
    badarg or "no error",
    badencode or "no error",
  }, "~")
end

function search_manga(a) return "[]" end

function get_manga_detail(a)
  return host.json.encode({
    id = "H1",
    title = json_errors(),
    description = payload(),
  })
end

function get_chapter_list(a) return "[]" end

function get_page_list(a) return "[]" end
