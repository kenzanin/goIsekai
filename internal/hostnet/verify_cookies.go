package hostnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	http "github.com/bogdanfinn/fhttp"
)

// parseVerifyCookies tolerantly parses cookieHeader into fhttp cookies scoped
// to domain. Accepted forms: a full "Cookie" header ("a=1; b=2"), a single
// "name=value" pair, a bare value (treated as cf_clearance=<value>), a
// Cookie-Editor style JSON export: [{"name":"...","value":"..."}, ...], or
// a cookies.txt (Netscape) export pasted verbatim ("# Netscape HTTP Cookie
// File" header, tab-separated domain/flag/path/secure/expiry/name/value
// lines; comment lines are skipped).
func parseVerifyCookies(domain, cookieHeader string) (verifySeed, error) {
	domain = normalizeDomain(domain)
	if domain == "" {
		return verifySeed{}, errors.New("hostnet: empty verify domain")
	}
	h := strings.TrimSpace(cookieHeader)
	if h == "" {
		return verifySeed{}, errors.New("hostnet: empty verify cookie")
	}
	if strings.HasPrefix(h, "[") {
		var err error
		h, err = cookieJSONToHeader(h)
		if err != nil {
			return verifySeed{}, err
		}
	} else if isNetscapeExport(h) {
		var err error
		h, err = cookieNetscapeToHeader(h)
		if err != nil {
			return verifySeed{}, err
		}
	}
	if !strings.ContainsAny(h, "=;") {
		h = "cf_clearance=" + h
	}
	cookies := http.ReadCookies(http.Header{"Cookie": []string{h}}, "")
	if len(cookies) == 0 {
		return verifySeed{}, fmt.Errorf("hostnet: no cookies parsed from %q", cookieHeader)
	}
	return verifySeed{domain: domain, cookies: cookies}, nil
}

// cookieJSONToHeader converts a Cookie-Editor JSON export into a Cookie header
// ("n1=v1; n2=v2"). Values are URL-unescaped when escaped (exporters encode
// trailing "=" as %3D); entries with an empty name are skipped.
func cookieJSONToHeader(raw string) (string, error) {
	var entries []cookieJSONEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return "", fmt.Errorf("hostnet: invalid cookie JSON: %w", err)
	}
	pairs := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		v := e.Value
		if u, err := url.QueryUnescape(v); err == nil {
			v = u
		}
		pairs = append(pairs, e.Name+"="+v)
	}
	if len(pairs) == 0 {
		return "", fmt.Errorf("hostnet: no cookies parsed from %q", raw)
	}
	return strings.Join(pairs, "; "), nil
}

// isNetscapeExport reports whether raw looks like a cookies.txt (Netscape)
// export: either the "# Netscape HTTP Cookie File" header comment or any
// tab-separated 7-field line.
func isNetscapeExport(raw string) bool {
	for line := range strings.Lines(raw) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Comment lines only count when they name the file format.
		if strings.HasPrefix(line, "#") {
			if strings.Contains(line, "Netscape HTTP Cookie File") {
				return true
			}
			continue
		}
		if len(strings.Split(line, "\t")) == 7 {
			return true
		}
		// First non-comment line decides: tab-less means not Netscape.
		return false
	}
	return false
}

// cookieNetscapeToHeader converts a cookies.txt (Netscape) export pasted
// verbatim into a Cookie header ("n1=v1; n2=v2"). Lines are tab-separated
// domain/flag/path/secure/expiry/name/value; comment lines ("#...") and
// malformed lines are skipped, name-less entries are dropped. Values are
// URL-unescaped when escaped (exporters encode trailing "=" as %3D).
func cookieNetscapeToHeader(raw string) (string, error) {
	pairs := make([]string, 0, 8)
	for line := range strings.Lines(raw) {
		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 7 {
			continue
		}
		name := strings.TrimSpace(fields[5])
		if name == "" {
			continue
		}
		v := strings.TrimSpace(fields[6])
		if u, err := url.QueryUnescape(v); err == nil {
			v = u
		}
		pairs = append(pairs, name+"="+v)
	}
	if len(pairs) == 0 {
		return "", fmt.Errorf("hostnet: no cookies parsed from %q", raw)
	}
	return strings.Join(pairs, "; "), nil
}

// normalizeDomain strips scheme and path from a caller-supplied domain, so
// "https://example.com/verify" and "example.com" both yield "example.com".
func normalizeDomain(domain string) string {
	d := strings.TrimSpace(domain)
	if i := strings.Index(d, "://"); i >= 0 {
		d = d[i+3:]
	}
	if i := strings.IndexByte(d, '/'); i >= 0 {
		d = d[:i]
	}
	return d
}
