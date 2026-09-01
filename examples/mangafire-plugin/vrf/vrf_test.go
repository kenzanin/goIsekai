package vrf

import "testing"

// TestSignVectors pins the signer against vectors verified live against
// mangafire.to (HTTP 200). Note the keyword value is the RAW query string:
// a space stays a literal space here; only the request URL encodes it as '+'.
func TestSignVectors(t *testing.T) {
	cases := []struct {
		path   string
		params map[string]string
		want   string
	}{
		{"/titles/dkw", nil, "8sK3xtqdFds7Xfo"},
		{"/titles", map[string]string{"keyword": "one piece", "limit": "50", "page": "1"}, "8sK3xtqdFZfetBhus6bRApNr5zMeEWBTZ95f9C_GdK1bchY3Fv5HBdo"},
	}
	for _, c := range cases {
		if got := Sign(c.path, c.params); got != c.want {
			t.Errorf("Sign(%q, %v) = %q, want %q", c.path, c.params, got, c.want)
		}
	}
}
