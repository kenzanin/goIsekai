package pluginutil

import (
	"encoding/json"
	"strings"
	"testing"
)

// comixMaterial builds a three-round material in the shape the real site ships.
// The values are arbitrary but the structure is not: three rounds, a full
// 256-byte sbox each, a 24-byte key each, and a seeded feedback byte.
func comixMaterial(t *testing.T) string {
	t.Helper()
	sbox := func(seed int) []int {
		box := make([]int, 256)
		for i := range box {
			box[i] = (i*167 + seed*31 + 7) % 256
		}
		// i*167 is coprime with 256, so the map is already a permutation; assert
		// it so a future edit that breaks that cannot silently pass.
		var seen [256]bool
		for _, v := range box {
			if seen[v] {
				t.Fatalf("test sbox seed %d is not a permutation", seed)
			}
			seen[v] = true
		}
		return box
	}
	key := func(seed int) []int {
		k := make([]int, 24)
		for i := range k {
			k[i] = (i*13 + seed*17 + 3) % 256
		}
		return k
	}
	m := SubstituteCipherMaterial{
		SBoxes:   [][]int{sbox(1), sbox(2), sbox(3)},
		Keys:     [][]int{key(1), key(2), key(3)},
		Previous: []int{189, 133, 32},
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal material: %v", err)
	}
	return string(b)
}

// TestSubstituteCipherRoundTrip is the contract that matters: decrypt undoes
// encrypt exactly. A site token is only useful if the plugin can also read the
// encrypted response body, and that is the same operation reversed.
func TestSubstituteCipherRoundTrip(t *testing.T) {
	material := comixMaterial(t)
	for _, plain := range []string{
		"/manga?keyword=isekai&page=1",
		"",
		"a",
		strings.Repeat("comix", 500),
		"\x00\x01\x02\xff binary-ish \x7f",
	} {
		enc, err := SubstituteCipher(plain, material, "encrypt")
		if err != nil {
			t.Fatalf("encrypt %q: %v", plain, err)
		}
		dec, err := SubstituteCipher(enc, material, "decrypt")
		if err != nil {
			t.Fatalf("decrypt %q: %v", enc, err)
		}
		if dec != plain {
			t.Errorf("round trip changed the payload\n in:  %q\n out: %q", plain, dec)
		}
	}
}

// TestSubstituteCipherFollowsTheDocumentedRecurrence pins the actual maths
// rather than only its invertibility, so a reimplementation that is self
// consistent but wrong still fails: out[i] = sbox[data[i] ^ key[i%len] ^ prev],
// with prev carrying the substituted byte forward.
func TestSubstituteCipherFollowsTheDocumentedRecurrence(t *testing.T) {
	m := SubstituteCipherMaterial{
		SBoxes:   [][]int{identity(256), identity(256)},
		Keys:     [][]int{{0x0f, 0x1f}, {0xf0, 0x0f}},
		Previous: []int{0x00, 0xaa},
	}
	material, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	data := []byte{0x01, 0x02, 0x03}
	got, err := SubstituteCipher(string(data), string(material), "encrypt")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Hand-computed with identity sboxes, so the only work left is the recurrence.
	want := make([]byte, len(data))
	prev := 0x00
	for i, b := range data {
		want[i] = byte(int(b) ^ []int{0x0f, 0x1f}[i%2] ^ prev)
		prev = int(want[i])
	}
	prev = 0xaa
	for i, b := range want {
		want[i] = byte(int(b) ^ []int{0xf0, 0x0f}[i%2] ^ prev)
		prev = int(want[i])
	}
	if got != string(want) {
		t.Errorf("recurrence mismatch\n got: %x\nwant: %x", got, want)
	}
}

// TestSubstituteCipherRejectsBadMaterial covers the trust boundary: a sbox that
// is not a permutation, or a round count that disagrees, must fail loudly. A
// silently accepted bad sbox yields a plausible-looking token and the site
// answers only "invalid_token", which points nowhere.
func TestSubstituteCipherRejectsBadMaterial(t *testing.T) {
	tests := []struct {
		name     string
		material SubstituteCipherMaterial
		wantErr  string
	}{
		{"sbox not a permutation", SubstituteCipherMaterial{
			SBoxes:   [][]int{repeat(7, 256)},
			Keys:     [][]int{{1}},
			Previous: []int{0},
		}, "not a permutation"},
		{"short sbox", SubstituteCipherMaterial{
			SBoxes:   [][]int{identity(16)},
			Keys:     [][]int{{1}},
			Previous: []int{0},
		}, "want 256"},
		{"sbox value out of range", SubstituteCipherMaterial{
			SBoxes:   [][]int{append(identity(255), 999)},
			Keys:     [][]int{{1}},
			Previous: []int{0},
		}, "out of range"},
		{"round count mismatch", SubstituteCipherMaterial{
			SBoxes:   [][]int{identity(256), identity(256)},
			Keys:     [][]int{{1}},
			Previous: []int{0, 1},
		}, "2 sboxes but 1 keys"},
		{"previous count mismatch", SubstituteCipherMaterial{
			SBoxes:   [][]int{identity(256)},
			Keys:     [][]int{{1}},
			Previous: []int{0, 1},
		}, "1 sboxes but 2 previous"},
		{"empty key", SubstituteCipherMaterial{
			SBoxes:   [][]int{identity(256)},
			Keys:     [][]int{{}},
			Previous: []int{0},
		}, "key is empty"},
		{"no rounds", SubstituteCipherMaterial{}, "no rounds"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.material)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			_, err = SubstituteCipher("data", string(b), "encrypt")
			if err == nil {
				t.Fatalf("want an error mentioning %q, got none", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not mention %q", err, tc.wantErr)
			}
		})
	}
}

// TestSubstituteCipherRejectsBadInput covers the other two failure modes: the
// material has to parse, and the direction has to be one we implement.
func TestSubstituteCipherRejectsBadInput(t *testing.T) {
	material := comixMaterial(t)

	if _, err := SubstituteCipher("data", "not json", "encrypt"); err == nil ||
		!strings.Contains(err.Error(), "material") {
		t.Errorf("unparseable material: want a material error, got %v", err)
	}
	for _, dir := range []string{"", "sign", "ENCRYPT", "both"} {
		if _, err := SubstituteCipher("data", material, dir); err == nil ||
			!strings.Contains(err.Error(), "direction") {
			t.Errorf("direction %q: want a direction error, got %v", dir, err)
		}
	}
}

// TestSubstituteCipherMaterialJSONShape locks the wire format, because a plugin
// stores this as a data file and a field rename would break every site using it
// with no compile-time signal.
func TestSubstituteCipherMaterialJSONShape(t *testing.T) {
	const material = `{"sboxes":[[0,1],[0,1]],"keys":[[2],[3]],"previous":[4,5]}`
	var m SubstituteCipherMaterial
	if err := json.Unmarshal([]byte(material), &m); err != nil {
		t.Fatalf("material does not parse: %v", err)
	}
	if len(m.SBoxes) != 2 || len(m.Keys) != 2 || len(m.Previous) != 2 {
		t.Fatalf("unexpected decode: %+v", m)
	}
	if m.SBoxes[1][1] != 1 || m.Keys[0][0] != 2 || m.Previous[1] != 5 {
		t.Errorf("fields decoded wrong: %+v", m)
	}
}

func identity(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func repeat(v, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = v
	}
	return out
}
