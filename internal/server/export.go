package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ndmik-dev/sofar/internal/store"
)

// This is a dump, not an API. It answers once, promises no compatibility and
// has no client — which is exactly what keeps the no-JSON-API rule intact
// while still letting the data leave.

type exportEntry struct {
	Title    string       `json:"title"`
	Original string       `json:"original,omitempty"`
	Kind     string       `json:"kind"`
	Status   string       `json:"status"`
	Position int          `json:"position"`
	Total    int          `json:"total,omitempty"`
	Unit     string       `json:"unit"`
	Rating   int          `json:"rating,omitempty"`
	Note     string       `json:"note,omitempty"`
	Year     int          `json:"year,omitempty"`
	TMDBID   int          `json:"tmdb_id,omitempty"`
	Links    []store.Link `json:"links,omitempty"`
}

type exportFile struct {
	App        string        `json:"app"`
	ExportedAt string        `json:"exported_at"`
	Entries    []exportEntry `json:"entries"`
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	c := s.now()
	out := exportFile{App: "sofar", ExportedAt: c.Now.Format(time.RFC3339)}

	for _, status := range []string{"active", "backlog", "done", "dropped"} {
		entries, err := s.store.ListEntries(r.Context(), defaultUserID, status)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		for _, e := range entries {
			links, err := s.store.LinksFor(r.Context(), e.ID)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			out.Entries = append(out.Entries, exportEntry{
				Title:    e.Media.Title,
				Original: e.Media.TitleOrig,
				Kind:     e.Media.Kind,
				Status:   e.Status,
				Position: e.Position,
				Total:    int(e.Media.TotalUnits.Int64),
				Unit:     e.Media.Unit,
				Rating:   int(e.Rating.Int64),
				Note:     e.Note,
				Year:     e.Media.Year,
				Links:    links,
			})
		}
	}

	body, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	name := fmt.Sprintf("sofar-%s.json", c.Now.Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
}
