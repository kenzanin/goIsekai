PLUGIN = { contract_version = 1, name = "Lua host natives" }

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
  }, "|")
end

function search_manga(a) return "[]" end

function get_manga_detail(a)
  return string.format('{"id":"H1","title":"payload","description":"%s"}', payload())
end

function get_chapter_list(a) return "[]" end

function get_page_list(a) return "[]" end
