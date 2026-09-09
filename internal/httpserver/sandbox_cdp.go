package httpserver

import "net/http"

func (s *Sandbox) handleCDPStatus(w http.ResponseWriter, r *http.Request) {
	cfg := s.svc.CDPStatus()
	writeJSON(w, http.StatusOK, map[string]any{
		"engine":  cfg.Engine,
		"path":    cfg.Path,
		"timeout": cfg.Timeout.String(),
		"enabled": cfg.Engine != "" && cfg.Engine != "off",
	})
}

func (s *Sandbox) handleCDPTest(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		writeErr(w, http.StatusBadRequest, "missing url param")
		return
	}
	cookies, ua, err := s.svc.TestCDP(targetURL)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var cookieList []map[string]any
	for _, c := range cookies {
		cookieList = append(cookieList, map[string]any{
			"name":   c.Name,
			"value":  c.Value,
			"domain": c.Domain,
			"path":   c.Path,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"cookies": cookieList,
		"ua":      ua,
	})
}

func (s *Sandbox) handleCDPCookies(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		writeErr(w, http.StatusBadRequest, "missing domain param")
		return
	}
	cookies := s.svc.CDPCookies(domain)
	writeJSON(w, http.StatusOK, cookies)
}
