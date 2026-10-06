package pluginutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

// The vectors are a real capture: 1manga's enc:v1 blob for chapter 59 and the
// key /api/chapter-crypto handed back in the same session. Decrypting them is
// what the plugin now depends on, so a regression here is a silent blank
// chapter rather than a visible failure.
const (
	capturedKey   = "fIaJHCLh6iIPDj4Vv54SRMzhLdMXiWa-HSuvfBrEctw"
	capturedIV    = "FB2e0IyeIcoogV8N"
	capturedTag   = "Y2rGLtxcy4BHHL06ySajAw"
	capturedCT    = "agYQmZoqr2ujaUC44QAy9LKoxXIrLg9L9Nda-QZAkasiIB0gkma3m53SleQYvIZcLxPoi5amKY-gNHyRFvh06_kO8ckMFK96EQWRzoCVqEZUoI21a4Glr-JayxLAHSV5EdO2oA8Ndlm6E17pCI3PLPElPXeUjzpEFBSDCocYWxQ3_1SMGesSV21h9hRyWh9htuTOpBTdPIUz4DlyXiBOUfy3ETVVmN0BLai8FqWhHf1lZOkLckF3XbbsWOGlBdcE9dCI2LMyuBOaklPIZOfMNmDebylBPwtU9RGbwtkDTVIX4QPHKyItKH2hLcY"
	capturedKeyID = "ecb0af300ae679c3"

	wantPlain = `{"p":"after-being-reborn-i-became-the-strongest-to-save-everyone/59/",`
)

func TestAESGCMDecryptB64DecryptsACapturedSiteMessage(t *testing.T) {
	got, err := AESGCMDecryptB64(capturedKey, capturedIV, capturedTag, capturedCT)
	if err != nil {
		t.Fatalf("decrypt the captured blob: %v", err)
	}
	if !strings.HasPrefix(got, wantPlain) {
		t.Errorf("plaintext prefix\n got: %.90s\nwant: %s", got, wantPlain)
	}
	// The point of the whole exercise: a non-contiguous page list survives.
	if !strings.Contains(got, `"1a.jpg"`) || !strings.Contains(got, `"14a.jpg"`) {
		t.Errorf("suffixed page names missing from the plaintext: %.140s", got)
	}
}

func TestAESGCMDecryptB64RoundTripsWithPaddedAndUnpaddedBase64(t *testing.T) {
	key := make([]byte, 32)
	iv := make([]byte, 12)
	plain := []byte("hello \x00\xff chapter pages")
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(iv); err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		t.Fatal(err)
	}
	sealed := gcm.Seal(nil, iv, plain, nil)
	ct, tag := sealed[:len(sealed)-16], sealed[len(sealed)-16:]

	enc := base64.RawURLEncoding
	got, err := AESGCMDecryptB64(
		enc.EncodeToString(key),
		enc.EncodeToString(iv),
		enc.EncodeToString(tag),
		enc.EncodeToString(ct),
	)
	if err != nil {
		t.Fatalf("unpadded: %v", err)
	}
	if got != string(plain) {
		t.Errorf("unpadded round trip = %q, want %q", got, plain)
	}

	// Padded input must be accepted: sites send both.
	pad := base64.URLEncoding
	if _, err := AESGCMDecryptB64(
		pad.EncodeToString(key),
		pad.EncodeToString(iv),
		pad.EncodeToString(tag),
		pad.EncodeToString(ct),
	); err != nil {
		t.Errorf("padded base64url rejected: %v", err)
	}
}

func TestAESGCMDecryptB64RejectsBadInput(t *testing.T) {
	const k32 = "fIaJHCLh6iIPDj4Vv54SRMzhLdMXiWa-HSuvfBrEctw"
	const iv12 = "FB2e0IyeIcoogV8N"
	const tag16 = "Y2rGLtxcy4BHHL06ySajAw"

	tests := []struct {
		name           string
		k, iv, tag, ct string
		wantSubstr     string
	}{
		{"short key", "c2hvcnQ", iv12, tag16, "YWJj", "AES-256"},
		{"short iv", k32, "c2hvcnQ", tag16, "YWJj", "12 bytes"},
		{"short tag", k32, iv12, "c2hvcnQ", "YWJj", "16 bytes"},
		{"bad base64", "!!!not base64!!!", iv12, tag16, "YWJj", "key"},
		{"wrong key", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", iv12, tag16,
			"agYQmZoqr2ujaUC44QAy9LKoxXIrLg9L9Nda-QZAkasiIB0gkma3m53SleQYvIZcLxPoi5amKY-gNHyRFvh06_kO8ckMFK96EQWRzoCVqEZUoI21a4Glr-JayxLAHSV5EdO2oA8Ndlm6E17pCI3PLPElPXeUjzpEFBSDCocYWxQ3_1SMGesSV21h9hRyWh9htuTOpBTdPIUz4DlyXiBOUfy3ETVVmN0BLai8FqWhHf1lZOkLckF3XbbsWOGlBdcE9dCI2LMyuBOaklPIZOfMNmDebylBPwtU9RGbwtkDTVIX4QPHKyItKH2hLcY", "authentication failed"},
		{"tampered ciphertext", k32, iv12, tag16,
			"aAYQmZoqr2ujaUC44QAy9LKoxX", "authentication failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AESGCMDecryptB64(tc.k, tc.iv, tc.tag, tc.ct)
			if err == nil {
				t.Fatalf("%s: expected an error", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not mention %q", err, tc.wantSubstr)
			}
		})
	}
}
