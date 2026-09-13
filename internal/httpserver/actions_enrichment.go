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
	s.viewMangaDetail(w, r)
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
	s.viewMangaDetail(w, r)
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
	s.viewMangaDetail(w, r)
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
	s.viewMangaDetail(w, r)
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
	s.viewMangaDetail(w, r)
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
	s.viewMangaDetail(w, r)
}

// handleFetchEnrichment fetches categories and related manga from enrichment
// sources and stores them. When source is empty, all registered sources are used.
func (s *Server) handleFetchEnrichment(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if err2 := r.ParseForm(); err2 != nil {
			s.logger.Error("fetch enrichment: parse form", "error", err2)
			http.Error(w, err2.Error(), http.StatusBadRequest)
			return
		}
	}
	title := r.FormValue("manga_title")
	s.logger.Debug("fetch enrichment", "pluginID", pluginID, "mangaID", mangaID, "title", title)
	source := r.FormValue("source")

	var sources []string
	if source != "" {
		sources = []string{source}
	} else {
		sources = s.service.EnrichmentSources()
	}

	if err := s.service.FetchEnrichment(pluginID, mangaID, title, sources); err != nil {
		s.logger.Error("fetch enrichment", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.viewMangaDetail(w, r)
}

func (s *Server) handleRemoveGenre(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("remove genre: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	genre := r.FormValue("genre")
	if err := s.service.ToggleGenre(pluginID, mangaID, genre); err != nil {
		s.logger.Error("remove genre", "pluginID", pluginID, "mangaID", mangaID, "genre", genre, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.viewMangaDetail(w, r)
}

func (s *Server) handleRemoveRelated(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("remove related: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	if err := s.service.RemoveRelated(pluginID, mangaID, title); err != nil {
		s.logger.Error("remove related", "pluginID", pluginID, "mangaID", mangaID, "title", title, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.viewMangaDetail(w, r)
}

func (s *Server) handleRemoveCategory(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("remove category: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	category := r.FormValue("category")
	if err := s.service.RemoveCategory(pluginID, mangaID, category); err != nil {
		s.logger.Error("remove category", "pluginID", pluginID, "mangaID", mangaID, "category", category, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.service.ToggleGenre(pluginID, mangaID, category); err != nil {
		s.logger.Error("toggle genre from category", "pluginID", pluginID, "mangaID", mangaID, "category", category, "error", err)
	}
	s.viewMangaDetail(w, r)
}

// handleAddCategory adds a user-supplied category to a manga.
func (s *Server) handleAddCategory(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("add category: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	category := r.FormValue("category")
	if err := s.service.AddCategory(pluginID, mangaID, category); err != nil {
		s.logger.Error("add category", "pluginID", pluginID, "mangaID", mangaID, "category", category, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.service.ToggleGenre(pluginID, mangaID, category); err != nil {
		s.logger.Error("toggle genre from category", "pluginID", pluginID, "mangaID", mangaID, "category", category, "error", err)
	}
	s.viewMangaDetail(w, r)
}

// handleAddGenre adds a user-supplied genre to a manga.
func (s *Server) handleAddGenre(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("add genre: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	genre := r.FormValue("genre")
	if err := s.service.ToggleGenre(pluginID, mangaID, genre); err != nil {
		s.logger.Error("add genre", "pluginID", pluginID, "mangaID", mangaID, "genre", genre, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.viewMangaDetail(w, r)
}

// handleResetEnrichment deletes all enrichment overrides and restores
// title, synopsis, and genres to their original plugin values.
func (s *Server) handleResetEnrichment(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := s.service.ResetEnrichment(pluginID, mangaID); err != nil {
		s.logger.Error("reset enrichment", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.viewMangaDetail(w, r)
}
