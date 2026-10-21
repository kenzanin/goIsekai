package pluginmanager

import (
	"context"
	"fmt"
)

// wasmInvokeResult finalises a wasm invocation. A challenge parked by an HTTP
// helper during the call wins over whatever the guest returned: the guest could
// only see a nil/error body and would otherwise report an empty result, hiding
// the fact that the site wants a human to verify. Returning the typed error
// here is the wasm counterpart of the Lua natives aborting the call.
func (m *Manager) wasmInvokeResult(id, out string) (string, error) {
	if err := m.takeChallenge(id); err != nil {
		return "", err
	}
	return out, nil
}

func (m *Manager) callWasm(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Challenges are per-invocation: drop anything an earlier call left behind
	// so a past block cannot fail this one.
	m.clearChallenge(p.id)

	if p.wasmMod == nil {
		return "", fmt.Errorf("wasm plugin %s: not loaded", p.id)
	}
	fn, ok := p.wasmFns[fnName]
	if !ok {
		return "", fmt.Errorf("wasm plugin %s: no export %s", p.id, fnName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.invokeDeadline())
	defer cancel()

	input := []byte(inputJSON)
	inPtr, ok := wasmAlloc(ctx, p.wasmMod, uint32(len(input)))
	if !ok {
		return "", fmt.Errorf("wasm plugin %s %s: malloc failed for input", p.id, fnName)
	}
	defer wasmFree(ctx, p.wasmMod, inPtr)
	if !p.wasmMod.Memory().Write(inPtr, input) {
		return "", fmt.Errorf("wasm plugin %s %s: write input out of range", p.id, fnName)
	}

	results, err := fn.Call(ctx, uint64(inPtr), uint64(len(input)))
	if err != nil {
		return "", fmt.Errorf("wasm plugin %s %s: %w", p.id, fnName, err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("wasm plugin %s %s: no result", p.id, fnName)
	}
	outPtr, outLen := unpack(results[0])
	if outPtr == 0 || outLen == 0 {
		return "", fmt.Errorf("wasm plugin %s %s: empty result", p.id, fnName)
	}
	defer wasmFree(ctx, p.wasmMod, outPtr)
	out, ok := p.wasmMod.Memory().Read(outPtr, outLen)
	if !ok {
		return "", fmt.Errorf("wasm plugin %s %s: result out of range", p.id, fnName)
	}
	return m.wasmInvokeResult(p.id, string(out))
}
