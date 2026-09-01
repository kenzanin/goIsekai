// Package vrf implements the MangaFire frontend VRF signer.
//
// MangaFire signs every /api request with a `vrf` query parameter: a 3-stage
// XOR-table transform over a sign-string, then base64url-Raw (no padding).
//
//	sign string = apiPath (without "/api") + "?" + "k=v" sorted by key,
//	              values NOT url-encoded (raw)
//	stage: out[i] = table[data[i] XOR key[i%len(key)] XOR prev]; prev=out[i-1]
//	IVs:   stage1=0x5A, stage2=0x35, stage3=0xBA
//
// Tables/keys rotate with the MangaFire frontend; the base64 constants below
// were extracted from its source and verified live (every /api endpoint
// returns HTTP 200 with these signatures).
package vrf

import (
	"encoding/base64"
	"sort"
	"strconv"
	"strings"
)

// vrfStages holds the decoded (256-byte tables, variable keys, stage IVs).
// Tables decode from base64 at init() and may contain 0x00 bytes, so they are
// kept as []byte and indexed by computed byte values (never as C-style strings).
var vrfStages = []struct {
	iv  byte
	key []byte
	tbl []byte
}{
	{0x5A, mustB64(vrfK1b64), mustB64(vrfT1b64)},
	{0x35, mustB64(vrfK2b64), mustB64(vrfT2b64)},
	{0xBA, mustB64(vrfK3b64), mustB64(vrfT3b64)},
}

func init() {
	for _, st := range vrfStages {
		if len(st.tbl) != 256 {
			panic("mangafire: VRF table len=" + strconv.Itoa(len(st.tbl)) + ", want 256")
		}
	}
}

func mustB64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic("mangafire: VRF decode: " + err.Error())
	}
	return b
}

// VRF table/key constants (base64). Rotate these when the frontend changes.
const (
	vrfK1b64 = "0Ec58JOY3uBzJK9m3zqIOpdlF7UFiax9DmA="
	vrfT1b64 = "yINlmUNho8VYJT+ibTIP+9ESiULpVEtMOoD6U6lRE0R/xwXo/Xp9NrUgC4cw/Lmo33vUyjUE40kUoEWIr/fxfNNcq2s79ShQ5NhNrFnJ4hXPwOu/SuXzIbuTQKGFvfm08E9jvCfqAtoDqvQq3dVWPQFmJjgvkISBeXY3BgANR+yVnjGbcxZ47d6kLNfZPIayTq3/YGySb1KuVZodWp/WGNAO5pfMcpaK53Hhs0allBszaMaxuouOwdxbwgxIw6YunSsXjI05Yi0j9j4eHKfSXR8Ifo/Od+8iamRfCXTyvm7NGRGYdcQ0ywcK/u6RXhrbcCm4t2eCtrDgQVecJGkQ+A=="
	vrfK2b64 = "AAdjb1iPY8CiDmq9H34tKTBF8a3oDQ=="
	vrfT2b64 = "IUFltCxD3Oc2cwCgkJffthaOg9cgPUb0LgW6H/VtfcF0kc5F25t+aWj6JH9VOhOaY0rAFdUxlDnl5BLNvwEJvQtP5qcw7vdb/K+chnbwnspSHT8mz5lqwz41TezG0hkO06FTjJZhsyNuFLDpD2ZZxQj/QIRcF90zpmQ7Byu483WsQqUE0C342HL+JXngRB6fRzxRyVTaKu83h7UYTJ0QMt6ixFh6S3F8gqkKwrGTL3jHNBsD45UnifK8+RGtishQV2K3rujLKEkiZxpr2dYcudFW4oFsDKhad3CLBvuyTqsCo4B7mL5IKQ1vXo/MOOvq1I1d8ar9X6Ttu5KF4fZgiA=="
	vrfK3b64 = "DELOJgPsVaCcblDtTGMdHzM="
	vrfT3b64 = "NQHlu1/wVO5EmkwQymF810qqY2xG1k2obcas4Z9mCsPEIFl9pRIjFxbJ7ybMHbBckT5Ton85E0FOeHezbh/mjlEYpmpnlXOS8dgrqeq2KfxImTh1YK9y0PeMNhzA1OQzSY9brYOJq/l2QnE/hwOeZIhPixVSKIUlDb5vLcH6RWKxkIEMuP0bDwIqQ71AJJaEaMJL7A6YtyIwoRT+L5v4aZzodN/0+3nOGsfblFjgxSfPzVDjNFeNl5P26+kEC/8AHgdrpAbt3hHz3HrRN1Y6e+JHgF7ncFWnoF0y3THL1S71WgWGCa6KtSzTCCG58n68nTyj2T3Sshk7utqCtMi/ZQ=="
)

// stage applies one transform: out[i] = table[data[i] XOR key[i%len(key)] XOR prev].
func stage(data []byte, iv byte, key, tbl []byte) []byte {
	out := make([]byte, len(data))
	prev := int(iv)
	kl := len(key)
	for i, b := range data {
		x := byte(int(b) ^ int(key[i%kl]) ^ prev)
		v := tbl[x] // 0-indexed into the 256-byte table
		out[i] = v
		prev = int(v)
	}
	return out
}

// Sign computes the VRF signature for an API path and query params.
// sign string = path (without /api) + "?" + "k=v" sorted by key (values NOT
// url-encoded). This matches the MangaFire frontend's canonical construction.
func Sign(apiPath string, params map[string]string) string {
	signStr := strings.TrimPrefix(apiPath, "/api")
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+"="+params[k])
		}
		signStr += "?" + strings.Join(parts, "&")
	}
	data := []byte(signStr)
	for _, st := range vrfStages {
		data = stage(data, st.iv, st.key, st.tbl)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
