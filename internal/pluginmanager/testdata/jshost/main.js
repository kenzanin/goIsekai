var PLUGIN = { contract_version: 1, name: "JS host natives" };

function payload() {
  return [
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
    host.crypto.vrf_sign("/titles", {page: "2"}, [
        {iv: 0x5A, key: "aw==", tbl: "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0+P0BBQkNERUZHSElKS0xNTk9QUVJTVFVWV1hZWltcXV5fYGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6e3x9fn+AgYKDhIWGh4iJiouMjY6PkJGSk5SVlpeYmZqbnJ2en6ChoqOkpaanqKmqq6ytrq+wsbKztLW2t7i5uru8vb6/wMHCw8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t/g4eLj5OXm5+jp6uvs7e7v8PHy8/T19vf4+fr7/P3+/w=="},
        {iv: 0x5A, key: "aw==", tbl: "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0+P0BBQkNERUZHSElKS0xNTk9QUVJTVFVWV1hZWltcXV5fYGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6e3x9fn+AgYKDhIWGh4iJiouMjY6PkJGSk5SVlpeYmZqbnJ2en6ChoqOkpaanqKmqq6ytrq+wsbKztLW2t7i5uru8vb6/wMHCw8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t/g4eLj5OXm5+jp6uvs7e7v8PHy8/T19vf4+fr7/P3+/w=="},
        {iv: 0x5A, key: "aw==", tbl: "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0+P0BBQkNERUZHSElKS0xNTk9QUVJTVFVWV1hZWltcXV5fYGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6e3x9fn+AgYKDhIWGh4iJiouMjY6PkJGSk5SVlpeYmZqbnJ2en6ChoqOkpaanqKmqq6ytrq+wsbKztLW2t7i5uru8vb6/wMHCw8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t/g4eLj5OXm5+jp6uvs7e7v8PHy8/T19vf4+fr7/P3+/w=="},
    ])
  ].join("|");
}

function searchManga(a) { return "[]"; }

function getMangaDetail(a) {
  return JSON.stringify({ id: "H1", title: "payload", description: payload() });
}

function getChapterList(a) { return "[]"; }

function getPageList(a) { return "[]"; }
