package pluginmanager

import (
	"errors"

	"goisekai/internal/pluginutil"
	"modernc.org/quickjs"
)

// jsVrfSign wraps host.crypto.vrf_sign(apiPath, params, stages) — the VRF
// signer algorithm in Go, with the current table set supplied by the plugin
// (so rotations are a plugin-side constant edit, no host rebuild).
// stages: [{iv:int, key:<b64>, tbl:<b64>}...]; returns the base64url signature.
func jsVrfSign(vm *quickjs.VM) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) == 0 {
			return "", errors.New("vrf_sign: missing apiPath")
		}
		apiPath, _ := args[0].(string)

		params := map[string]string{}
		if len(args) > 1 {
			switch p := args[1].(type) {
			case *quickjs.Object:
				m, _ := objectToMap(p)
				for k, v := range m {
					if s, ok := v.(string); ok {
						params[k] = s
					}
				}
			}
		}

		var stages []pluginutil.VRFStageB64
		if len(args) > 2 {
			if arr, ok := args[2].(*quickjs.Object); ok {
				s, err := jsVRFStages(arr)
				if err != nil {
					return "", err
				}
				stages = s
			}
		}

		decoded, err := pluginutil.VRFStagesB64(stages)
		if err != nil {
			return "", err
		}
		return pluginutil.VRFSign(apiPath, params, decoded), nil
	}
}

// jsVRFStages converts a quickjs object (JS array) of stages to wire form.
func jsVRFStages(v *quickjs.Object) ([]pluginutil.VRFStageB64, error) {
	var stages []pluginutil.VRFStageB64
	items, err := arrayToSlice(v)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			s, err := pluginutil.VRFStageFromMap(m)
			if err != nil {
				return nil, err
			}
			stages = append(stages, s)
		}
	}
	return stages, nil
}

// jsNormalizeStatus creates the host.text.normalize_status(raw) callable.
// It returns the canonical status label, or "" when raw is not part of the
// host's canonical vocabulary. The host owns the vocabulary; a plugin scraping
// a site-specific word translates it itself.
func jsNormalizeStatus() quickjs.HostFunc {
	return func(args []any) (any, error) {
		var raw string
		if len(args) > 0 {
			raw, _ = args[0].(string)
		}
		return pluginutil.NormalizeStatus(raw), nil
	}
}
