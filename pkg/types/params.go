package types

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// PluginParams holds the values a plugin declares in the "params" object of
// its plugin.json.
//
// Every value is a string, because that is what a plugin splices into a URL
// or a matcher. A number or a bool in the JSON is stringified rather than
// rejected: one unquoted value must not cost the plugin its whole metadata
// block, since a failed metadata read silently drops the name and the logo.
type PluginParams map[string]string

// UnmarshalJSON reads the params object, stringifying every value.
func (p *PluginParams) UnmarshalJSON(b []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(PluginParams, len(raw))
	for k, v := range raw {
		out[k] = paramString(v)
	}
	*p = out
	return nil
}

// paramString renders one params value as the string a plugin receives.
func paramString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}
