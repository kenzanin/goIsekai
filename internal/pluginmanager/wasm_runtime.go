package pluginmanager

import (
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"goisekai/pkg/types"
)

// ensureWasmRuntime lazily creates the shared wazero runtime once,
// instantiating WASI and the host module ("env" exporting host_http_request and
// the generic host_call that reaches every Lua/JS host helper).
//
// It takes runtimeMu, NOT mu: the callers (Install, LoadPlugin) already hold
// mu, and sync.RWMutex is not reentrant, so taking mu again here would
// self-deadlock the host on every wasm plugin install.
func (m *Manager) ensureWasmRuntime() (wazero.Runtime, error) {
	m.runtimeMu.Lock()
	defer m.runtimeMu.Unlock()
	if m.runtime != nil {
		return m.runtime, nil
	}
	rt := wazero.NewRuntimeWithConfig(m.ctx,
		wazero.NewRuntimeConfig().WithMemoryLimitPages(memoryLimitPages).WithCloseOnContextDone(true))
	if _, err := wasi_snapshot_preview1.Instantiate(m.ctx, rt); err != nil {
		_ = rt.Close(m.ctx)
		return nil, fmt.Errorf("instantiate wasi: %w", err)
	}
	if _, err := rt.NewHostModuleBuilder(hostModuleName).
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(m.hostHTTPRequest),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI64}).
		Export(types.HostHTTPRequestFunc).
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(m.hostCall),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI64}).
		Export(types.HostCallFunc).
		Instantiate(m.ctx); err != nil {
		_ = rt.Close(m.ctx)
		return nil, fmt.Errorf("instantiate host module: %w", err)
	}
	m.runtime = rt
	return rt, nil
}
