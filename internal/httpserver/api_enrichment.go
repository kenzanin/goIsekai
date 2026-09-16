package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// apiEnrichmentResponse is the JSON shape for enrichment data.
type apiEnrichmentResponse struct {
	AltTitles    []apiEnrichmentItem `json:"alt_titles"`
	AltSummaries []apiEnrichmentItem `json:"alt_summaries"`
	Categories   []apiEnrichmentItem `json:"categories"`
	Related      []apiEnrichmentItem `json:"related"`
}

// apiEnrichmentItem is one enrichment entry.
type apiEnrichmentItem struct {
	Value  string `json:"value"`
	URL    string `json:"url,omitempty"`
	Source string `json:"source"`
}

// apiEnrichment returns enrichment data for a manga.
func (s *Server) apiEnrichment(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")

	item, err := s.service.GetEnrichment(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api enrichment", "error", err)
		writeErr(w, http.StatusNotFound, "enrichment not found")
		return
	}

	altTitles := make([]apiEnrichmentItem, 0, len(item.AltTitles))
	for _, a := range item.AltTitles {
		altTitles = append(altTitles, apiEnrichmentItem{Value: a.Value, Source: a.Source})
	}
	altSummaries := make([]apiEnrichmentItem, 0, len(item.AltSummaries))
	for _, a := range item.AltSummaries {
		altSummaries = append(altSummaries, apiEnrichmentItem{Value: a.Value, Source: a.Source})
	}
	categories := make([]apiEnrichmentItem, 0, len(item.Categories))
	for _, c := range item.Categories {
		categories = append(categories, apiEnrichmentItem{Value: c.Value, Source: c.Source})
	}
	related := make([]apiEnrichmentItem, 0, len(item.Related))
	for _, r := range item.Related {
		related = append(related, apiEnrichmentItem{Value: r.Value, URL: r.URL, Source: r.Source})
	}

	writeJSON(w, http.StatusOK, apiEnrichmentResponse{
		AltTitles:    altTitles,
		AltSummaries: altSummaries,
		Categories:   categories,
		Related:      related,
	})
}

// apiCategories returns just the categories for a manga.
func (s *Server) apiCategories(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")

	cats, err := s.service.ListCategories(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api categories", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to load categories")
		return
	}

	items := make([]apiEnrichmentItem, 0, len(cats))
	for _, c := range cats {
		items = append(items, apiEnrichmentItem{Value: c.Value, Source: c.Source})
	}
	writeJSON(w, http.StatusOK, items)
}

// apiRelated returns related/recommended manga for a manga.
func (s *Server) apiRelated(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")

	rels, err := s.service.ListRelated(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api related", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to load related manga")
		return
	}

	items := make([]apiEnrichmentItem, 0, len(rels))
	for _, r := range rels {
		items = append(items, apiEnrichmentItem{Value: r.Value, URL: r.URL, Source: r.Source})
	}
	writeJSON(w, http.StatusOK, items)
}

// apiRemoveCategory deletes a category from a manga's enrichment data.
func (s *Server) apiRemoveCategory(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")
	q := r.URL.Query()
	category := q.Get("category")
	if category == "" {
		writeErr(w, http.StatusBadRequest, "category not specified")
		return
	}
	if err := s.service.RemoveCategory(pluginID, mangaID, category); err != nil {
		s.logger.Warn("remove category", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to remove category")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// apiRemoveRelated deletes a related/recommended manga from storage.
func (s *Server) apiRemoveRelated(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")
	q := r.URL.Query()
	title := q.Get("title")
	if title == "" {
		writeErr(w, http.StatusBadRequest, "title not specified")
		return
	}
	if err := s.service.RemoveRelated(pluginID, mangaID, title); err != nil {
		s.logger.Warn("remove related", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to remove related manga")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// apiFetchEnrich fetches enrichment from a source and returns the updated data
// as JSON. Called by the JS enrichment panel after selecting kind + source.
func (s *Server) apiFetchEnrich(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")
	q := r.URL.Query()
	source := q.Get("source")
	title := q.Get("title")

	if source == "" {
		writeErr(w, http.StatusBadRequest, "source not selected")
		return
	}

	if err := s.service.FetchEnrichment(pluginID, mangaID, title, []string{source}); err != nil {
		s.logger.Error("api fetch enrich", "pluginID", pluginID, "mangaID", mangaID, "source", source, "error", err)
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}

	cats, err := s.service.ListCategories(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api fetch enrich list categories", "error", err)
	}
	categories := make([]apiEnrichmentItem, 0, len(cats))
	for _, c := range cats {
		categories = append(categories, apiEnrichmentItem{Value: c.Value, Source: c.Source})
	}
	rels, err := s.service.ListRelated(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api fetch enrich list related", "error", err)
	}
	related := make([]apiEnrichmentItem, 0, len(rels))
	for _, r := range rels {
		related = append(related, apiEnrichmentItem{Value: r.Value, URL: r.URL, Source: r.Source})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
		"related":    related,
	})
}
