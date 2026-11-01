package pluginmanager

import (
	"testing"
	"time"

	"modernc.org/quickjs"
)

// TestJSTimeoutInterrupt verifies that SetEvalTimeout aborts a spinning JS
// loop. quickjs re-arms the interrupt handler on every Eval/Call, so a
// single SetEvalTimeout before a long-running CallValue is sufficient: when
// the budget expires, the next JS checkpoint aborts with an error.
func TestJSTimeoutInterrupt(t *testing.T) {
	vm, err := quickjs.NewVM()
	if err != nil {
		t.Fatalf("NewVM: %v", err)
	}
	defer func() { _ = vm.Close() }()

	// Set a 1-second eval timeout.
	if err := vm.SetEvalTimeout(1 * time.Second); err != nil {
		t.Fatalf("SetEvalTimeout: %v", err)
	}

	// Busy-spin for about 5 seconds in JS. quickjs's interrupt fires at
	// the 1-second checkpoint and aborts the eval.
	start := time.Now()
	_, err = vm.Eval("while(true){}", quickjs.EvalGlobal)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected timeout error, got nil after %v", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout did not fire promptly: elapsed %v", elapsed)
	}
	t.Logf("timeout fired after %v: %v", elapsed, err)
}
