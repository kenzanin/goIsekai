package httpserver

import "net/http"

// handleFetchAltTitles resolves alternative titles via the provider plugin
// and redirects back to the manga detail page. The server is taken from the
// form (browser flow).
func (s *Server) handleFetchAltTitles(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("fetch alt titles: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	server := r.FormValue("server")
	if _, err := s.service.FetchAltTitles(pluginID, mangaID, server); err != nil {
		s.logger.Error("fetch alt titles", "pluginID", pluginID, "mangaID", mangaID, "server", server, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleSetTitle promotes the submitted title to be the manga's main title.
func (s *Server) handleSetTitle(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("set title: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	if err := s.service.SetMainTitle(pluginID, mangaID, title); err != nil {
		s.logger.Error("set title", "pluginID", pluginID, "mangaID", mangaID, "title", title, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleRemoveAltTitle removes the submitted alternative title.
func (s *Server) handleRemoveAltTitle(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("remove alt title: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	if err := s.service.RemoveAltTitle(pluginID, mangaID, title); err != nil {
		s.logger.Error("remove alt title", "pluginID", pluginID, "mangaID", mangaID, "title", title, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleFetchAltSummaries resolves alternative summaries via the provider
// plugin and redirects back to the manga detail page.
func (s *Server) handleFetchAltSummaries(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("fetch alt summaries: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	server := r.FormValue("server")
	if _, err := s.service.FetchAltSummaries(pluginID, mangaID, server); err != nil {
		s.logger.Error("fetch alt summaries", "pluginID", pluginID, "mangaID", mangaID, "server", server, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleRemoveAltSummary removes the submitted alternative description.
func (s *Server) handleRemoveAltSummary(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("remove alt summary: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	description := r.FormValue("description")
	if err := s.service.RemoveAltSummary(pluginID, mangaID, description); err != nil {
		s.logger.Error("remove alt summary", "pluginID", pluginID, "mangaID", mangaID, "description", description, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleSetSummary promotes the submitted alternative description to be
// the manga's main description.
func (s *Server) handleSetSummary(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("set summary: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	description := r.FormValue("description")
	if err := s.service.SetMainSummary(pluginID, mangaID, description); err != nil {
		s.logger.Error("set summary", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}
