package pluginmanager

import (
	"encoding/json"
	"testing"

	"github.com/dop251/goja"

	"goisekai/internal/hostnet"
	"goisekai/internal/pluginutil"
)

// subCipherMaterialJSON is a three-round material shaped like the ones sites
// ship inside their front-end bundle. Values are arbitrary; the structure is
// what matters, and it matches the shape the comix extension documents.
const subCipherMaterialJSON = `{"sboxes":[[0,167,78,245],[0,167,78,245],[0,167,78,245]],` +
	`"keys":[[13,30,47,64,81,98,115,132,149,166,183,200,217,234,251,12,29,46,63,80,97,114,131,148],` +
	`[30,47,64,81,98,115,132,149,166,183,200,217,234,251,12,29,46,63,80,97,114,131,148,165],` +
	`[47,64,81,98,115,132,149,166,183,200,217,234,251,12,29,46,63,80,97,114,131,148,165,182]],` +
	`"previous":[189,133,32]}`

// sboxPayload is sbox truncated to a real 256-entry permutation, built in Go so
// the test material is valid rather than illustrative.
func sboxPayload(t *testing.T) string {
	t.Helper()
	var m pluginutil.SubstituteCipherMaterial
	if err := json.Unmarshal([]byte(subCipherMaterialJSON), &m); err != nil {
		t.Fatalf("material does not parse: %v", err)
	}
	for r := range m.SBoxes {
		full := make([]int, 256)
		for i := range full {
			full[i] = (i*167 + r*31 + 7) % 256
		}
		m.SBoxes[r] = full
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal material: %v", err)
	}
	return string(b)
}

// TestLuaSubstituteCipher exercises the native through the Lua VM rather than
// calling pluginutil directly, because the failure this guards is registering
// the native in one runtime and forgetting the other: that only surfaces as a
// plugin calling a nil field at read time, with no compile-time signal.
func TestLuaSubstituteCipher(t *testing.T) {
	material := sboxPayload(t)
	chunk := `
		local material = ` + quoteLua(material) + `
		local enc = host.crypto.substitute_cipher("/manga?keyword=x", material, "encrypt")
		assert(type(enc) == "string" and #enc > 0, "encrypt must return a string")
		assert(enc ~= "/manga?keyword=x", "encrypt must change the payload")
		local dec = host.crypto.substitute_cipher(enc, material, "decrypt")
		assert(dec == "/manga?keyword=x", "round trip failed, got: " .. tostring(dec))
	`
	if err := luaEval(t, chunk); err != nil {
		t.Fatalf("host.crypto.substitute_cipher (Lua): %v", err)
	}
}

func TestLuaSubstituteCipherReportsErrors(t *testing.T) {
	chunk := `
		local out, err = host.crypto.substitute_cipher("data", "not json", "encrypt")
		assert(out == nil, "unparseable material must not produce output")
		assert(type(err) == "string" and string.find(err, "material", 1, true) ~= nil,
			"error should name the material: " .. tostring(err))

		local out2, err2 = host.crypto.substitute_cipher("data", ` + quoteLua(sboxPayload(t)) + `, "sideways")
		assert(out2 == nil, "an unknown direction must not produce output")
		assert(type(err2) == "string" and string.find(err2, "direction", 1, true) ~= nil,
			"error should name the direction: " .. tostring(err2))
	`
	if err := luaEval(t, chunk); err != nil {
		t.Fatalf("substitute_cipher error path (Lua): %v", err)
	}
}

func TestJSSubstituteCipher(t *testing.T) {
	vm := goja.New()
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "jsplug"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}
	material := sboxPayload(t)
	script := `
		const material = ` + quoteJSON(material) + `;
		const enc = host.crypto.substitute_cipher("/manga?keyword=x", material, "encrypt");
		if (typeof enc !== "string" || enc.length === 0) throw new Error("encrypt must return a string");
		if (enc === "/manga?keyword=x") throw new Error("encrypt must change the payload");
		const dec = host.crypto.substitute_cipher(enc, material, "decrypt");
		if (dec !== "/manga?keyword=x") throw new Error("round trip failed, got " + dec);
	`
	if _, err := vm.RunString(script); err != nil {
		t.Fatalf("host.crypto.substitute_cipher (JS): %v", err)
	}
}

func TestJSSubstituteCipherThrows(t *testing.T) {
	vm := goja.New()
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "jsplug"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}
	if _, err := vm.RunString(`host.crypto.substitute_cipher("data", "not json", "encrypt")`); err == nil {
		t.Fatal("JS: unparseable material must throw, not return silently")
	}
	if _, err := vm.RunString(`host.crypto.substitute_cipher("data", ` + quoteJSON(sboxPayload(t)) + `, "sideways")`); err == nil {
		t.Fatal("JS: an unknown direction must throw")
	}
}

// quoteLua renders a Go string as a Lua long-bracket literal, so JSON containing
// quotes needs no escaping. The payload starts with "{" and "[{" would parse as
// a table constructor, so the level-2 form with leading and trailing newlines is
// required: Lua only reads a long string when a newline follows the opener.
func quoteLua(s string) string { return "[==[\n" + s + "\n]==]" }

// quoteJSON renders a Go string as a JS single-quoted literal.
func quoteJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
