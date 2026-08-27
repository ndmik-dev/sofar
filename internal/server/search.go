package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type searchResult struct {
	TMDBType  string
	TMDBID    int
	Kind      string
	KindLabel string
	Title     string
	Original  string
	Year      int
	Meta      string
	First     bool
}

type searchView struct {
	Query    string
	Results  []searchResult
	Message  string
	Disabled bool
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	view := searchView{Query: q}

	switch {
	case !s.catalog.Enabled():
		view.Disabled = true
		view.Message = "TMDB_TOKEN не заданий — пошук по каталогу вимкнено"
	case len([]rune(q)) < 2:
		view.Message = "Введи щонайменше дві літери"
	default:
		results, err := s.catalog.Search(r.Context(), q, 8)
		if err != nil {
			s.log.Error("tmdb search", "q", q, "err", err)
			view.Message = "Каталог не відповідає. Спробуй ще раз."
			break
		}
		if len(results) == 0 {
			view.Message = "Нічого не знайшлось"
		}
		for i, res := range results {
			view.Results = append(view.Results, searchResult{
				TMDBType:  res.TMDBType,
				TMDBID:    res.TMDBID,
				Kind:      res.Kind,
				KindLabel: kindLabels[res.Kind],
				Title:     res.Title,
				Original:  res.Original,
				Year:      res.Year,
				Meta:      strconv.Itoa(res.Year),
				First:     i == 0,
			})
		}
	}

	s.renderFragment(w, r, "search-results", view)
}

func (s *Server) handleAdd(w http.ResponseWriter, r *http.Request) {
	tmdbType := r.FormValue("tmdb_type")
	tmdbID, err := strconv.Atoi(r.FormValue("tmdb_id"))
	if err != nil || tmdbID <= 0 {
		http.Error(w, "bad tmdb id", http.StatusBadRequest)
		return
	}
	status := orDefault(r.FormValue("status"), "active")
	switch status {
	case "backlog", "active", "done", "dropped":
	default:
		http.Error(w, "unknown status", http.StatusBadRequest)
		return
	}
	position, _ := strconv.Atoi(r.FormValue("position"))

	now := time.Now().In(s.cfg.Loc)
	mediaID, err := s.catalog.Import(r.Context(), tmdbType, tmdbID, now)
	if err != nil {
		s.log.Error("tmdb import", "type", tmdbType, "id", tmdbID, "err", err)
		http.Error(w, "Каталог не відповідає. Спробуй ще раз.", http.StatusBadGateway)
		return
	}

	if _, _, err := s.store.AddEntry(r.Context(), defaultUserID, mediaID, status, position, now); err != nil {
		s.fail(w, r, err)
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusNoContent)
}
