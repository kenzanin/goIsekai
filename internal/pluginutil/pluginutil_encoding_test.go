package pluginutil

import (
	"testing"
)

// Encoding helpers are the byte-level contract every JS/Lua crypto script
// depends on (key-mint, VRF proofs, mangafire XOR). A silent change here is a
// blank chapter, not a visible failure — so each helper is pinned by vector.
func TestBase64RoundTrip(t *testing.T) {
	const in = "hello\x00\xff binary \xe2\x98\x83"
	enc := Base64Encode(in)
	if got, err := Base64Decode(enc); err != nil || got != in {
		t.Fatalf("base64 round trip: got %q err %v", got, err)
	}
	if _, err := Base64Decode("!!!not-base64!!!"); err == nil {
		t.Fatal("Base64Decode must fail on invalid input")
	}
}

func TestBase64URLToleratesPadding(t *testing.T) {
	const in = "mangafire payload?" // encodes with padding chars
	enc := Base64URLEncode(in)
	if got, err := Base64URLDecode(enc); err != nil || got != in {
		t.Fatalf("base64url round trip: got %q err %v", got, err)
	}
	if got, err := Base64URLDecode(enc + "=="); err != nil || got != in {
		t.Fatalf("base64url must tolerate padded input: got %q err %v", got, err)
	}
}

func TestHexRoundTrip(t *testing.T) {
	const in = "deadBEEF bytes"
	enc := HexEncode(in)
	if enc != "6465616442454546206279746573" {
		t.Fatalf("HexEncode: got %q", enc)
	}
	if got, err := HexDecode(enc); err != nil || got != in {
		t.Fatalf("hex round trip: got %q err %v", got, err)
	}
	if got, err := HexDecode("DEADBEEF"); err != nil || got != "\xde\xad\xbe\xef" {
		t.Fatalf("HexDecode must accept uppercase: got %q err %v", got, err)
	}
	if _, err := HexDecode("zz"); err == nil {
		t.Fatal("HexDecode must fail on invalid input")
	}
}

func TestKnownDigests(t *testing.T) {
	if got := SHA256Hex(""); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("SHA256Hex(\"\"): got %q", got)
	}
	if got := MD5Hex(""); got != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Fatalf("MD5Hex(\"\"): got %q", got)
	}
	// RFC 4231 test case 2.
	if got := HMACSHA256Hex("Jefe", "what do ya want for nothing?"); got != "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843" {
		t.Fatalf("HMACSHA256Hex: got %q", got)
	}
}

func TestXORShortestWins(t *testing.T) {
	if got := XOR([]byte{0xff, 0x00, 0xaa}, []byte{0x0f}); string(got) != "\xf0\x00\x00" {
		t.Fatalf("XOR must XOR to the shorter length, zero-filling the tail: %x", got)
	}
	if got, err := XORHex("ff00", "0f0f"); err != nil || got != "f00f" {
		t.Fatalf("XORHex: got %q err %v", got, err)
	}
	if _, err := XORHex("zz", "00"); err == nil {
		t.Fatal("XORHex must fail on invalid hex")
	}
}

func TestHexB64CrossConversions(t *testing.T) {
	const b64 = "aGVsbG8=" // "hello"
	if got, err := B64DecodeHex(b64); err != nil || got != "68656c6c6f" {
		t.Fatalf("B64DecodeHex: got %q err %v", got, err)
	}
	if got, err := B64URLEncodeHex("68656c6c6f"); err != nil || got != "aGVsbG8" {
		t.Fatalf("B64URLEncodeHex: got %q err %v", got, err)
	}
	if got, err := B64URLDecodeHex("aGVsbG8"); err != nil || got != "68656c6c6f" {
		t.Fatalf("B64URLDecodeHex: got %q err %v", got, err)
	}
	if got := UTF8Hex("A\xe2\x82\xac"); got != "41e282ac" {
		t.Fatalf("UTF8Hex: got %q", got)
	}
}
