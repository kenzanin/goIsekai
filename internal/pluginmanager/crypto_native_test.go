package pluginmanager

import (
	"testing"

	"github.com/dop251/goja"

	"goisekai/internal/hostnet"
)

// A real 1manga capture. The plugin reaches this through host.crypto, so the
// test has to go through the Lua VM rather than calling pluginutil directly:
// registering the native in one runtime and forgetting the other is exactly the
// kind of half-change that only shows up as a blank chapter at read time.
const (
	aesKey = "fIaJHCLh6iIPDj4Vv54SRMzhLdMXiWa-HSuvfBrEctw"
	aesIV  = "FB2e0IyeIcoogV8N"
	aesTag = "Y2rGLtxcy4BHHL06ySajAw"
	aesCT  = "agYQmZoqr2ujaUC44QAy9LKoxXIrLg9L9Nda-QZAkasiIB0gkma3m53SleQYvIZcLxPoi5amKY-gNHyRFvh06_kO8ckMFK96EQWRzoCVqEZUoI21a4Glr-JayxLAHSV5EdO2oA8Ndlm6E17pCI3PLPElPXeUjzpEFBSDCocYWxQ3_1SMGesSV21h9hRyWh9htuTOpBTdPIUz4DlyXiBOUfy3ETVVmN0BLai8FqWhHf1lZOkLckF3XbbsWOGlBdcE9dCI2LMyuBOaklPIZOfMNmDebylBPwtU9RGbwtkDTVIX4QPHKyItKH2hLcY"
)

func TestLuaCryptoAESGCMDecrypt(t *testing.T) {
	chunk := `
		local plain = host.crypto.aes_gcm_decrypt("` + aesKey + `", "` + aesIV + `", "` + aesTag + `", "` + aesCT + `")
		assert(plain ~= nil, "decrypt returned nil")
		assert(string.find(plain, "1a.jpg", 1, true) ~= nil, "suffixed page name missing: " .. plain)
		assert(string.find(plain, "14a.jpg", 1, true) ~= nil, "later suffixed page missing")
	`
	if err := luaEval(t, chunk); err != nil {
		t.Fatalf("host.crypto.aes_gcm_decrypt (Lua): %v", err)
	}
}

func TestLuaCryptoAESGCMDecryptReportsErrors(t *testing.T) {
	// A native that returns nil with no message would leave the plugin logging
	// a nil, so the error half of the return must be a usable string.
	chunk := `
		local plain, err = host.crypto.aes_gcm_decrypt("c2hvcnQ", "` + aesIV + `", "` + aesTag + `", "` + aesCT + `")
		assert(plain == nil, "a bad key must not decrypt")
		assert(type(err) == "string" and #err > 0, "error must be a non-empty string")
		assert(string.find(err, "AES-256", 1, true) ~= nil, "error should say what was wrong: " .. tostring(err))
	`
	if err := luaEval(t, chunk); err != nil {
		t.Fatalf("aes_gcm_decrypt error path (Lua): %v", err)
	}
}

func TestJSCryptoAESGCMDecrypt(t *testing.T) {
	vm := goja.New()
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "aes"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}
	chunk := `
		function assert(c, msg) { if (!c) throw new Error(msg || "assertion failed"); }
		var plain = host.crypto.aes_gcm_decrypt("` + aesKey + `", "` + aesIV + `", "` + aesTag + `", "` + aesCT + `");
		assert(typeof plain === "string", "decrypt should return a string");
		assert(plain.indexOf("1a.jpg") !== -1, "suffixed page name missing");
		assert(plain.indexOf("14a.jpg") !== -1, "later suffixed page missing");
	`
	if _, err := vm.RunString(chunk); err != nil {
		t.Fatalf("host.crypto.aes_gcm_decrypt (JS): %v", err)
	}

	// goja reports a native error by throwing, matching jsStr2Err.
	if _, err := vm.RunString(`host.crypto.aes_gcm_decrypt("c2hvcnQ", "` + aesIV + `", "` + aesTag + `", "` + aesCT + `")`); err == nil {
		t.Error("a bad key must throw in the JS runtime")
	}
}
