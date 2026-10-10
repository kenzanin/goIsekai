package httpserver

import (
	"io"
	"net/http"
)

// handleSaveVerify stores pasted verification cookies/UA for a plugin.
// If mangaID is provided in the path (e.g., /action/save-verify/{pluginID}/{mangaID}),
// redirect back to the manga detail page so the inline wizard closes on success.
// If only pluginID is present (e.g., the plugins/search pages), redirect back to
// the search page preserving the query params so the user stays on their search.
func (s *Server) handleSaveVerify(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := r.PathValue("mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("save verify: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.service.SavePluginVerify(pluginID, r.FormValue("cookies"), r.FormValue("user_agent")); err != nil {
		s.logger.Error("save verify", "pluginID", pluginID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Prefer the manga page (if called with mangaID) else the search page.
	if mangaID != "" {
		s.hxRedirect(w, r, "/view/manga/"+pluginID+"/"+mangaID)
	} else {
		// Preserve the original search query (q, pluginID, genre, page) so the
		// wizard closes and the user is dropped back on the same search results.
		query := r.URL.Query()
		urlStr := "/view/search"
		if len(query) > 0 {
			urlStr += "?" + query.Encode()
		}
		s.hxRedirect(w, r, urlStr)
	}
}

// bytesBuffer wraps a byte slice to satisfy io.ReaderAt.
type bytesBuffer struct {
	data []byte
}

func (b *bytesBuffer) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n = copy(p, b.data[off:])
	return n, nil
}
