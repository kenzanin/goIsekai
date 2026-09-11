package pluginutil

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
)

// VRFStage is one XOR-table stage of the mangafire VRF signer.
// Key and Tbl are decoded bytes; IV is the chain initial value.
type VRFStage struct {
	IV  byte
	Key []byte
	Tbl []byte
}

// VRFStageB64 is a single stage in wire form (base64 key/tbl) so plugins
// can declare the current table set as data and rotate it without a host
// rebuild.
type VRFStageB64 struct {
	IV  byte
	Key string
	Tbl string
}

// VRFStageFromMap converts a {iv,key,tbl} map (from Lua table or JS object)
// to a wire-form stage. iv may be float64 (Lua) or int64 (goja).
func VRFStageFromMap(m map[string]any) (VRFStageB64, error) {
	var s VRFStageB64
	iv, ok := m["iv"]
	if !ok {
		return s, fmt.Errorf("vrf stage missing iv")
	}
	switch v := iv.(type) {
	case int64:
		s.IV = byte(v)
	case float64:
		s.IV = byte(v)
	default:
		return s, fmt.Errorf("vrf stage iv: unexpected type %T", iv)
	}
	key, _ := m["key"].(string)
	tbl, _ := m["tbl"].(string)
	if key == "" || tbl == "" {
		return s, fmt.Errorf("vrf stage missing key/tbl")
	}
	s.Key, s.Tbl = key, tbl
	return s, nil
}

// VRFStagesB64 converts wire-form stages to decoded bytes.
func VRFStagesB64(stages []VRFStageB64) ([]VRFStage, error) {
	out := make([]VRFStage, 0, len(stages))
	for i, s := range stages {
		key, err := base64.StdEncoding.DecodeString(s.Key)
		if err != nil {
			return nil, fmt.Errorf("vrf stage %d: key: %w", i, err)
		}
		tbl, err := base64.StdEncoding.DecodeString(s.Tbl)
		if err != nil {
			return nil, fmt.Errorf("vrf stage %d: tbl: %w", i, err)
		}
		out = append(out, VRFStage{IV: s.IV, Key: key, Tbl: tbl})
	}
	return out, nil
}

// VRFSign signs an API request the way mangafire requires (keiyoushi
// VrfSigner port, mirrored byte-for-byte from the JS plugin that runs live):
// signStr = apiPath minus a leading "/api" + "?" + sorted "k=v" raw values;
// then three chained XOR-table stages out[i] = tbl[(data[i]^key[i%len]^prev)]
// with prev seeded by each stage IV; result is base64url without padding.
func VRFSign(apiPath string, params map[string]string, stages []VRFStage) string {
	signStr := strings.TrimPrefix(apiPath, "/api")
	if len(params) > 0 {
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+"="+params[k])
		}
		signStr += "?" + strings.Join(parts, "&")
	}

	data := []byte(signStr)
	for _, st := range stages {
		data = vrfStage(data, st.IV, st.Key, st.Tbl)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// vrfStage: out[i] = tbl[(data[i]^key[i%len(key)]^prev) & 0xFF], prev=iv.
func vrfStage(data []byte, iv byte, key, tbl []byte) []byte {
	out := make([]byte, len(data))
	prev := iv
	for i, d := range data {
		x := int(d) ^ int(key[i%len(key)]) ^ int(prev)
		prev = tbl[x&0xFF]
		out[i] = prev
	}
	return out
}
