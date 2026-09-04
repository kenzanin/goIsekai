package pluginmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/extism/go-sdk"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// load compiles, instantiates, and version-checks a single WASM plugin using
// the Extism runtime. Each plugin gets its own extism.Plugin instance with
// a dedicated host_http_request function that routes through the hostnet proxy
// (keyed by plugin id for cookie isolation).
func (m *Manager) load(id, wasmPath string) (*loadedPlugin, error) {
	manifest := extism.Manifest{
		Wasm:    []extism.Wasm{extism.WasmFile{Path: wasmPath, Name: id}},
		Memory:  &extism.ManifestMemory{MaxPages: memoryLimitPages},
		Timeout: uint64(invokeTimeout.Milliseconds()),
	}

	hostFuncs := []extism.HostFunction{
		extism.NewHostFunctionWithStack(
			types.HostHTTPRequestFunc,
			m.makeHostHTTPRequest(id),
			[]extism.ValueType{extism.ValueTypePTR},
			[]extism.ValueType{extism.ValueTypePTR},
		),
	}

	plugin, err := extism.NewPlugin(m.ctx, manifest, extism.PluginConfig{
		EnableWasi: true,
	}, hostFuncs)
	if err != nil {
		return nil, fmt.Errorf("extism.NewPlugin %s: %w", id, err)
	}
	plugin.Timeout = invokeTimeout

	// --- contract_version --------------------------------------------------
	_, verOut, err := plugin.Call(types.ContractVersionFunc, nil)
	if err != nil {
		_ = plugin.Close(m.ctx)
		return nil, fmt.Errorf("contract_version: %w", err)
	}
	ver, err := strconv.Atoi(string(verOut))
	if err != nil {
		_ = plugin.Close(m.ctx)
		return nil, fmt.Errorf("contract_version: parse %q: %w", string(verOut), err)
	}
	if err := types.CheckVersion(int32(ver)); err != nil {
		_ = plugin.Close(m.ctx)
		return nil, err
	}
	logger.Debug("contract_version resolved", "id", id, "version", ver)

	p := &loadedPlugin{
		id:              id,
		wasmPath:        wasmPath,
		kind:            "wasm",
		loaded:          true,
		extismPlugin:    plugin,
		contractVersion: int32(ver),
	}
	p.meta = m.readInit(p)
	return p, nil
}

// makeHostHTTPRequest returns a HostFunctionStackCallback that routes plugin
// HTTP requests through the hostnet proxy keyed by pluginID for cookie
// isolation. The Extism calling convention passes a single pointer (offset
// into plugin memory) to a length-prefixed block; the callback reads the
// request JSON, delegates to the proxy, and writes the response back.
func (m *Manager) makeHostHTTPRequest(pluginID string) extism.HostFunctionStackCallback {
	return func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
		reqJSON, err := p.ReadString(stack[0])
		if err != nil {
			logger.Debug("host_http_request: read input", "plugin", pluginID, "error", err)
			stack[0] = 0
			return
		}
		respJSON, err := m.proxy.HandleRequest(pluginID, reqJSON)
		if err != nil {
			// Surface the failure as an empty response rather than
			// trapping; plugins can detect empty bodies.
			logger.Debug("host_http_request: proxy error", "plugin", pluginID, "error", err)
			stack[0] = 0
			return
		}
		stack[0], err = p.WriteString(respJSON)
		if err != nil {
			logger.Debug("host_http_request: write output", "plugin", pluginID, "error", err)
			stack[0] = 0
		}
	}
}

// readInit calls the plugin's Init export and parses the returned PluginMeta.
// A plugin without Init simply contributes zero metadata.
func (m *Manager) readInit(p *loadedPlugin) types.PluginMeta {
	_, initOut, err := p.extismPlugin.Call(types.InitFunc, nil)
	if err != nil {
		logger.Debug("plugin Init failed", "id", p.id, "error", err)
		return types.PluginMeta{}
	}
	if len(initOut) == 0 {
		return types.PluginMeta{}
	}
	var meta types.PluginMeta
	if err := json.Unmarshal(initOut, &meta); err != nil {
		logger.Debug("plugin Init returned invalid JSON", "id", p.id, "error", err)
		return types.PluginMeta{}
	}
	return meta
}
