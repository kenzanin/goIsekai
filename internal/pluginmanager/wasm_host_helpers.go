package pluginmanager

import (
	"fmt"
	"strings"

	"github.com/goccy/go-json"

	"goisekai/internal/htmldoc"
	"goisekai/internal/logger"
	"goisekai/internal/pluginutil"
)

// wasmLog forwards a wasm plugin's log.<level>(msg) to the host logger, tagged
// with the plugin id exactly like the Lua and JS globals. Without this a wasm
// plugin had no way to emit a diagnostic at all — the reason a port that
// misbehaved was silent instead of explaining itself.
func wasmLog(id, fn, msg string) {
	switch fn {
	case "log.debug":
		logger.Debug(msg, "plugin", id)
	case "log.info":
		logger.Info(msg, "plugin", id)
	case "log.warn":
		logger.Warn(msg, "plugin", id)
	case "log.error":
		logger.Error(msg, "plugin", id)
	}
}

// wasmRegexRow shapes one match's captures the way the JS find/find_all do: a
// single capture is the value itself, several are the capture array, and no
// match is null.
func wasmRegexRow(row []string) any {
	switch len(row) {
	case 0:
		return nil
	case 1:
		return row[0]
	default:
		return row
	}
}

// wasmNormalizeStatus is host.text.normalize_status in its two call shapes:
// (raw) with the default vocabulary, or (mapJSON, raw) with a plugin-supplied
// map of lowercased keys to canonical values.
func wasmNormalizeStatus(args []string) (any, error) {
	if len(args) < 2 {
		return pluginutil.NormalizeStatus(nil, wasmArg(args, 0)), nil
	}
	m := map[string]string{}
	if err := json.Unmarshal([]byte(args[0]), &m); err != nil {
		return nil, fmt.Errorf("normalize_status: %w", err)
	}
	if len(m) == 0 {
		m = nil
	}
	return pluginutil.NormalizeStatus(m, args[1]), nil
}

// wasmHTML parses the markup and runs one lookup. It is the stateless
// equivalent of the Lua/JS document handle: the markup is the first argument on
// every call, since a wasm guest cannot hold a Go handle between calls.
func wasmHTML(args []string, lookup func(*htmldoc.Document) (any, error)) (any, error) {
	doc, err := htmldoc.Parse(wasmArg(args, 0))
	if err != nil {
		return nil, err
	}
	return lookup(doc)
}

// wasmVRFSign rebuilds host.crypto.vrf_sign(apiPath, params, stages) from JSON:
// params is a JSON object of string values, stages a JSON array of the
// {iv,key,tbl} base64 objects the signer consumes.
func wasmVRFSign(apiPath, paramsJSON, stagesJSON string) (any, error) {
	params := map[string]string{}
	if strings.TrimSpace(paramsJSON) != "" {
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			return nil, fmt.Errorf("vrf_sign params: %w", err)
		}
	}
	var rawStages []map[string]any
	if strings.TrimSpace(stagesJSON) != "" {
		if err := json.Unmarshal([]byte(stagesJSON), &rawStages); err != nil {
			return nil, fmt.Errorf("vrf_sign stages: %w", err)
		}
	}
	stages := make([]pluginutil.VRFStageB64, 0, len(rawStages))
	for i, raw := range rawStages {
		stage, err := pluginutil.VRFStageFromMap(raw)
		if err != nil {
			return nil, fmt.Errorf("vrf_sign stage %d: %w", i, err)
		}
		stages = append(stages, stage)
	}
	decoded, err := pluginutil.VRFStagesB64(stages)
	if err != nil {
		return nil, fmt.Errorf("vrf_sign stages: %w", err)
	}
	return pluginutil.VRFSign(apiPath, params, decoded), nil
}
