package pluginutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// SubstituteCipherMaterial is the data a byte-wise substitution cipher needs.
// It is passed in by the plugin rather than compiled in here, because the
// primitive is generic: sites that obfuscate their API this way ship the
// tables as part of their own front-end bundle, and the plugin is the only
// layer that has read them.
//
// SBoxes must each be a permutation of 0..255, Keys are the per-round key
// bytes (repeating, so any length works), and Previous seeds the running
// feedback byte for each round.
type SubstituteCipherMaterial struct {
	SBoxes   [][]int `json:"sboxes"`
	Keys     [][]int `json:"keys"`
	Previous []int   `json:"previous"`
}

// Validate rejects material a cipher cannot run on. A malformed sbox would
// otherwise produce a wrong-but-plausible token, and the site's answer to a
// wrong token is a bare "invalid_token" with nothing pointing at the cause.
func (m *SubstituteCipherMaterial) Validate() error {
	if len(m.SBoxes) != len(m.Keys) {
		return fmt.Errorf("substitute_cipher: %d sboxes but %d keys", len(m.SBoxes), len(m.Keys))
	}
	if len(m.SBoxes) == 0 {
		return errors.New("substitute_cipher: material has no rounds")
	}
	if len(m.Previous) != len(m.SBoxes) {
		return fmt.Errorf("substitute_cipher: %d sboxes but %d previous values", len(m.SBoxes), len(m.Previous))
	}
	for r, box := range m.SBoxes {
		if len(box) != 256 {
			return fmt.Errorf("substitute_cipher: round %d sbox has %d entries, want 256", r, len(box))
		}
		var seen [256]bool
		for _, v := range box {
			if v < 0 || v > 255 {
				return fmt.Errorf("substitute_cipher: round %d sbox value %d out of range", r, v)
			}
			if seen[v] {
				return fmt.Errorf("substitute_cipher: round %d sbox repeats value %d, so it is not a permutation", r, v)
			}
			seen[v] = true
		}
	}
	for r, key := range m.Keys {
		if len(key) == 0 {
			return fmt.Errorf("substitute_cipher: round %d key is empty", r)
		}
		for _, v := range key {
			if v < 0 || v > 255 {
				return fmt.Errorf("substitute_cipher: round %d key value %d out of range", r, v)
			}
		}
	}
	for r, p := range m.Previous {
		if p < 0 || p > 255 {
			return fmt.Errorf("substitute_cipher: round %d previous value %d out of range", r, p)
		}
	}
	return nil
}

// SubstituteCipher runs data through the material's rounds.
//
// direction is "encrypt" to apply the sboxes forward and "decrypt" to invert
// them; the two are exact inverses, so decrypt(encrypt(x)) == x for any x the
// material accepts.
//
// The primitive exists because neither plugin runtime can express it: Lunar
// Lua has no bitwise operators at all (they fail at the parser), and the JS
// runtime's bitwise forms are not available to plugins. Any site that signs
// its API with a byte-wise cipher therefore forces this into the host.
func SubstituteCipher(data, materialJSON, direction string) (string, error) {
	var m SubstituteCipherMaterial
	if err := json.Unmarshal([]byte(materialJSON), &m); err != nil {
		return "", fmt.Errorf("substitute_cipher material: %w", err)
	}
	if err := m.Validate(); err != nil {
		return "", err
	}
	in := []byte(data)

	switch direction {
	case "encrypt":
		for r := range m.SBoxes {
			in = substitute(in, m.SBoxes[r], m.Keys[r], m.Previous[r])
		}
	case "decrypt":
		for r, v := range slices.Backward(m.SBoxes) {
			in = substituteInverse(in, v, m.Keys[r], m.Previous[r])
		}
	default:
		return "", fmt.Errorf("substitute_cipher: direction must be \"encrypt\" or \"decrypt\", got %q", direction)
	}
	return string(in), nil
}

// substitute maps every byte through box[data[i] ^ key[i % len(key)] ^ prev],
// where prev carries the already-substituted byte forward. That feedback is
// what makes the transform position-dependent.
func substitute(data []byte, box, key []int, prev int) []byte {
	out := make([]byte, len(data))
	for i, b := range data {
		v := box[int(b)^key[i%len(key)]^prev]
		out[i] = byte(v)
		prev = v
	}
	return out
}

// substituteInverse walks the same recurrence backwards. The inverse box is
// built per call rather than cached: the payload is a request line or one JSON
// body, so the cost is noise next to the HTTP round trip it precedes.
func substituteInverse(data []byte, box, key []int, prev int) []byte {
	var inverse [256]int
	for i, v := range box {
		inverse[v] = i
	}
	out := make([]byte, len(data))
	for i, b := range data {
		cur := int(b)
		out[i] = byte(inverse[cur] ^ key[i%len(key)] ^ prev)
		prev = cur
	}
	return out
}
