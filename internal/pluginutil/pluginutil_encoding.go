package pluginutil

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// Base64Encode returns standard padded base64.
func Base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// Base64Decode decodes standard padded base64.
func Base64Decode(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Base64URLEncode returns unpadded base64url (JWT/VRF style).
func Base64URLEncode(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// Base64URLDecode decodes unpadded base64url, tolerating padded input.
func Base64URLDecode(s string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// HexEncode returns lowercase hex.
func HexEncode(s string) string {
	return hex.EncodeToString([]byte(s))
}

// HexDecode decodes lowercase or uppercase hex.
func HexDecode(s string) (string, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// SHA256Hex returns the lowercase hex SHA-256 digest.
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// MD5Hex returns the lowercase hex MD5 digest.
func MD5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// HMACSHA256Hex returns the lowercase hex HMAC-SHA256 of msg under key.
func HMACSHA256Hex(key, msg string) string {
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

// XOR returns the byte-wise XOR of a and b as a new byte slice.
// If lengths differ, XORs up to the shorter length.
// ponytail: could accept varargs for multi-source XOR, add when mangafire needs it.
func XOR(a, b []byte) []byte {
	out := make([]byte, len(a))
	n := min(len(b), len(a))
	for i := range n {
		out[i] = a[i] ^ b[i]
	}
	return out
}

// XORHex returns the byte-wise XOR of two hex-encoded strings as a new hex string.
// If lengths differ, XORs up to the shorter length.
// ponytail: could accept varargs for multi-source XOR, add when mangafire needs it.
func XORHex(a, b string) (string, error) {
	aa, err := hex.DecodeString(a)
	if err != nil {
		return "", err
	}
	bb, err := hex.DecodeString(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(XOR(aa, bb)), nil
}

// UTF8Hex encodes a string as its UTF-8 bytes, returned as lowercase hex.
func UTF8Hex(s string) string {
	return hex.EncodeToString([]byte(s))
}

// B64DecodeHex decodes standard padded base64 and returns the result as hex.
func B64DecodeHex(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// B64URLEncodeHex encodes a hex string as bytes and returns base64url (unpadded).
func B64URLEncodeHex(h string) (string, error) {
	b, err := hex.DecodeString(h)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// B64URLDecodeHex decodes unpadded base64url and returns the result as hex.
func B64URLDecodeHex(s string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
