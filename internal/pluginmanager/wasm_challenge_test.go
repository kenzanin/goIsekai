package pluginmanager

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"goisekai/internal/hostnet"
)

// The wasm guest cannot throw, so an anti-bot challenge is parked on the
// manager during the invoke and turned into the typed error afterwards. These
// tests pin that hand-off: getting it wrong is silent in the worst way, because
// the verify wizard would never re-open and the user would just see an empty
// result.

// TestWasmChallengeParkedBecomesTypedError covers the happy path: an HTTP
// helper parks a challenge, and the finished invoke reports it.
func TestWasmChallengeParkedBecomesTypedError(t *testing.T) {
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())

	cerr := &hostnet.ChallengeError{VerifyURL: "https://site.example/chapter/1"}
	mgr.parkChallenge("cf", cerr)

	out, err := mgr.wasmInvokeResult("cf", `[{"id":"x"}]`)
	if err == nil {
		t.Fatal("a parked challenge must fail the invoke, got a successful result")
	}
	if out != "" {
		t.Errorf("result = %q, want empty on challenge", out)
	}
	var ce *hostnet.ChallengeError
	if !errors.As(err, &ce) {
		t.Fatalf("error is not a *hostnet.ChallengeError: %v", err)
	}
	if ce.VerifyURL != cerr.VerifyURL {
		t.Errorf("VerifyURL = %q, want %q", ce.VerifyURL, cerr.VerifyURL)
	}
	if !errors.Is(err, hostnet.ErrChallenge) {
		t.Error("the typed error must still match hostnet.ErrChallenge")
	}
}

// TestWasmChallengeClearedAfterUse pins that the marker is consumed: a later
// successful invoke must not be poisoned by a challenge that already fired.
func TestWasmChallengeClearedAfterUse(t *testing.T) {
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	mgr.parkChallenge("cf", &hostnet.ChallengeError{VerifyURL: "https://site.example/"})

	if _, err := mgr.wasmInvokeResult("cf", ""); err == nil {
		t.Fatal("first finalise should report the challenge")
	}
	out, err := mgr.wasmInvokeResult("cf", `[]`)
	if err != nil {
		t.Fatalf("second finalise must be clean, got %v", err)
	}
	if out != `[]` {
		t.Errorf("result = %q, want []", out)
	}
}

// TestWasmChallengeIsPerPlugin pins that one plugin's challenge cannot fail
// another plugin's invoke.
func TestWasmChallengeIsPerPlugin(t *testing.T) {
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	mgr.parkChallenge("cf", &hostnet.ChallengeError{VerifyURL: "https://a.example/"})

	if _, err := mgr.wasmInvokeResult("other", `[]`); err != nil {
		t.Errorf("an unrelated plugin must not inherit the challenge: %v", err)
	}
	if _, err := mgr.wasmInvokeResult("cf", ""); !errors.Is(err, hostnet.ErrChallenge) {
		t.Errorf("the challenged plugin must still report it, got %v", err)
	}
}

// TestWasmParkChallengeIgnoresNonChallenges pins that only a challenge is
// parked. An ordinary transport failure already reaches the guest as a nil or
// status-0 body, and must not be upgraded into a verify prompt.
func TestWasmParkChallengeIgnoresNonChallenges(t *testing.T) {
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())

	mgr.parkChallenge("plain", nil)
	mgr.parkChallenge("plain", fmt.Errorf("hostnet: request failed: connection reset"))
	mgr.parkChallenge("plain", context.DeadlineExceeded)

	if _, err := mgr.wasmInvokeResult("plain", `[]`); err != nil {
		t.Errorf("a clean invoke must succeed, got %v", err)
	}
}

// TestWasmParkChallengeWrappedError pins that a wrapped ChallengeError is still
// recognised: the proxy wraps it in "hostnet: request failed: ...".
func TestWasmParkChallengeWrappedError(t *testing.T) {
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())

	cerr := &hostnet.ChallengeError{VerifyURL: "https://x.example/v"}
	mgr.parkChallenge("wrapped", fmt.Errorf("hostnet: request failed: %w", cerr))

	if _, err := mgr.wasmInvokeResult("wrapped", ""); !errors.Is(err, hostnet.ErrChallenge) {
		t.Errorf("finalise = %v, want it to match ErrChallenge", err)
	}
}

// TestWasmClearChallengeDropsStaleMarker pins the per-invoke reset: a challenge
// left from a previous call must not fail the next one.
func TestWasmClearChallengeDropsStaleMarker(t *testing.T) {
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	mgr.parkChallenge("cf", &hostnet.ChallengeError{VerifyURL: "https://stale.example/"})

	mgr.clearChallenge("cf")

	if _, err := mgr.wasmInvokeResult("cf", `[]`); err != nil {
		t.Errorf("a cleared challenge must not fail the invoke, got %v", err)
	}
}
