package pluginutil

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// AESGCMDecryptB64 decrypts one AES-256-GCM message. All four arguments are
// base64url without padding, which is how WebCrypto-based sites hand them over
// and what the b64url_encode_hex / b64url_decode_hex helpers already speak.
//
// The auth tag is passed separately rather than appended to the ciphertext: the
// WebCrypto and node APIs both take it appended, so callers porting from either
// split it out themselves. That keeps this a plain cipher primitive instead of
// an envelope format.
//
// The tag is verified, so a wrong key or tampered ciphertext returns an error
// rather than garbage.
func AESGCMDecryptB64(keyB64, ivB64, tagB64, ctB64 string) (string, error) {
	key, err := decodeB64URL(keyB64)
	if err != nil {
		return "", fmt.Errorf("aes_gcm_decrypt key: %w", err)
	}
	if len(key) != 32 {
		// AES accepts 16/24/32-byte keys, but this helper is documented as
		// AES-256 and every caller so far wants exactly that. Being strict here
		// turns a truncated or mis-decoded key into a clear message.
		return "", fmt.Errorf("aes_gcm_decrypt key: want 32 bytes for AES-256, got %d", len(key))
	}
	iv, err := decodeB64URL(ivB64)
	if err != nil {
		return "", fmt.Errorf("aes_gcm_decrypt iv: %w", err)
	}
	if len(iv) != 12 {
		return "", fmt.Errorf("aes_gcm_decrypt iv: want 12 bytes for GCM, got %d", len(iv))
	}
	tag, err := decodeB64URL(tagB64)
	if err != nil {
		return "", fmt.Errorf("aes_gcm_decrypt tag: %w", err)
	}
	if len(tag) != 16 {
		return "", fmt.Errorf("aes_gcm_decrypt tag: want 16 bytes for GCM, got %d", len(tag))
	}
	ct, err := decodeB64URL(ctB64)
	if err != nil {
		return "", fmt.Errorf("aes_gcm_decrypt ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes_gcm_decrypt cipher: %w", err)
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		return "", fmt.Errorf("aes_gcm_decrypt gcm: %w", err)
	}
	plain, err := gcm.Open(nil, iv, append(ct, tag...), nil)
	if err != nil {
		// The overwhelmingly common cause is a key from a different session: the
		// key and the message are fetched separately and rotate independently.
		return "", errors.New("aes_gcm_decrypt: authentication failed (wrong key, or the ciphertext does not match its tag)")
	}
	return string(plain), nil
}

// decodeB64URL accepts both the padded and unpadded base64url alphabets. Sites
// in the wild send either, and padding is meaningless for this encoding.
func decodeB64URL(s string) ([]byte, error) {
	s = strings.TrimRight(s, "=")
	if strings.ContainsAny(s, "+/") {
		return base64.StdEncoding.DecodeString(s)
	}
	return base64.RawURLEncoding.DecodeString(s)
}
