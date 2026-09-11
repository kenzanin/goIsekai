package pluginutil

import "testing"

func TestURLEncode(t *testing.T) {
	tests := []struct{ in, want string }{
		{"a b/c?d=e&f", "a%20b%2Fc%3Fd%3De%26f"},
		{"café", "caf%C3%A9"},
		{"AZaz09-._~", "AZaz09-._~"},
		{"100%", "100%25"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := URLEncode(tt.in); got != tt.want {
			t.Errorf("URLEncode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestURLDecode(t *testing.T) {
	tests := []struct{ in, want string }{
		{"a%20b%2Fc", "a b/c"},
		{"caf%C3%A9", "café"},
		{"plain", "plain"},
		{"%00kept", "%00kept"}, // control byte stays encoded
		{"%zz", "%zz"},         // malformed stays literal
		{"100%", "100%"},
	}
	for _, tt := range tests {
		if got := URLDecode(tt.in); got != tt.want {
			t.Errorf("URLDecode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestURLRoundTrip(t *testing.T) {
	for _, s := range []string{"a b/c?d=e&f", "café — résumé", "日本語 title"} {
		if got := URLDecode(URLEncode(s)); got != s {
			t.Errorf("roundtrip %q -> %q", s, got)
		}
	}
}

func TestHTMLDecode(t *testing.T) {
	tests := []struct{ in, want string }{
		{"&amp;", "&"},
		{"&#039;", "'"},
		{"&quot;x&quot;", `"x"`},
		{"&lt;b&gt;", "<b>"},
		{"plain", "plain"},
	}
	for _, tt := range tests {
		if got := HTMLDecode(tt.in); got != tt.want {
			t.Errorf("HTMLDecode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct{ in, want string }{
		{"<p>a<br>b<br/>c<br />d</p>", "a\nb\nc\nd"},
		{"<div><span>hi</span></div>", "hi"},
		{"<p>&amp; &#039;</p>", "& '"},
		{"  <p>trim</p>  ", "trim"},
		{"no tags", "no tags"},
	}
	for _, tt := range tests {
		if got := StripHTML(tt.in); got != tt.want {
			t.Errorf("StripHTML(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTitlecase(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello world", "Hello world"},
		{"", ""},
		{"éclair", "Éclair"},
		{"Already", "Already"},
	}
	for _, tt := range tests {
		if got := Titlecase(tt.in); got != tt.want {
			t.Errorf("Titlecase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestBase64(t *testing.T) {
	if got := Base64Encode("hello"); got != "aGVsbG8=" {
		t.Errorf("Base64Encode = %q", got)
	}
	if got, err := Base64Decode("aGVsbG8="); err != nil || got != "hello" {
		t.Errorf("Base64Decode = %q, %v", got, err)
	}
	if _, err := Base64Decode("not base64!!"); err == nil {
		t.Error("Base64Decode should fail on invalid input")
	}

	if got := Base64URLEncode("a?b"); got != "YT9i" {
		t.Errorf("Base64URLEncode = %q", got)
	}
	if got, err := Base64URLDecode("YT9i"); err != nil || got != "a?b" {
		t.Errorf("Base64URLDecode = %q, %v", got, err)
	}
	if got, err := Base64URLDecode("YT9i="); err != nil || got != "a?b" {
		t.Errorf("Base64URLDecode padded = %q, %v", got, err)
	}
}

func TestHex(t *testing.T) {
	if got := HexEncode("abc"); got != "616263" {
		t.Errorf("HexEncode = %q", got)
	}
	if got, err := HexDecode("616263"); err != nil || got != "abc" {
		t.Errorf("HexDecode = %q, %v", got, err)
	}
	if _, err := HexDecode("zz"); err == nil {
		t.Error("HexDecode should fail on invalid input")
	}
}

func TestCryptoVectors(t *testing.T) {
	if got := SHA256Hex("abc"); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("SHA256Hex(abc) = %q", got)
	}
	if got := MD5Hex("abc"); got != "900150983cd24fb0d6963f7d28e17f72" {
		t.Errorf("MD5Hex(abc) = %q", got)
	}
	const want = "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"
	if got := HMACSHA256Hex("key", "The quick brown fox jumps over the lazy dog"); got != want {
		t.Errorf("HMACSHA256Hex = %q, want %q", got, want)
	}
}

func TestStripMarkdown(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"**bold** and *italic*", "bold and italic"},
		{"[x](http://y)", "x"},
		{"a [b](c) [d](e)", "a b d"},
		{"__u__ _e_", "u e"},
		{"<http://x>", "http://x"},
		{"# Heading\n\ntext", "Heading\n\ntext"},
		{"a\n\n\n\nb", "a\n\nb"},
		{"line  \ntrailing", "line\ntrailing"},
		{"***bi***", "bi"},
		{"---", ""},
		{"plain text", "plain text"},
	}
	for _, tc := range tests {
		if got := StripMarkdown(tc.in); got != tc.want {
			t.Errorf("StripMarkdown(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
