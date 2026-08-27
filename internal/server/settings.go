package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ndmik-dev/sofar/internal/store"
)

type settingRow struct {
	Kind       string
	Label      string
	Depth      string
	Step       int
	UnitsLabel string
	Steps      []int
}

type settingsPage struct {
	Title    string
	NavView  navView
	Now      time.Time
	Settings []settingRow
}

var settingsMeta = map[string]struct {
	UnitsLabel string
	Steps      []int
}{
	"show":    {"серії", []int{1, 2}},
	"anime":   {"серії", []int{1, 2}},
	"movie":   {"галочка", []int{1}},
	"book":    {"сторінки", []int{1, 5, 10, 25}},
	"game":    {"години", []int{1, 2}},
	"podcast": {"епізоди", []int{1}},
}

func (s *Server) settingRows(r *http.Request) ([]settingRow, error) {
	saved, err := s.store.TypeSettings(r.Context(), defaultUserID)
	if err != nil {
		return nil, err
	}
	var out []settingRow
	for _, k := range kindNav {
		row := settingRow{
			Kind:       k.Kind,
			Label:      k.Label,
			Depth:      "units",
			Step:       1,
			UnitsLabel: settingsMeta[k.Kind].UnitsLabel,
			Steps:      settingsMeta[k.Kind].Steps,
		}
		if ts, ok := saved[k.Kind]; ok {
			row.Depth, row.Step = ts.Depth, ts.Step
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := s.settingRows(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	statusCounts, err := s.store.CountsByStatus(r.Context(), defaultUserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	kindCounts, err := s.store.CountsByKind(r.Context(), defaultUserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "settings.html", settingsPage{
		Title:    "Налаштування",
		NavView:  navViewFrom(statusCounts, kindCounts, "/settings", false),
		Now:      time.Now().In(s.cfg.Loc),
		Settings: rows,
	})
}

func (s *Server) handleSetSetting(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	meta, ok := settingsMeta[kind]
	if !ok {
		http.Error(w, "unknown kind", http.StatusBadRequest)
		return
	}
	depth := r.FormValue("depth")
	if depth != "status" && depth != "units" {
		http.Error(w, "unknown depth", http.StatusBadRequest)
		return
	}
	step, _ := strconv.Atoi(r.FormValue("step"))
	if !contains(meta.Steps, step) {
		step = meta.Steps[0]
	}

	if err := s.store.SetTypeSetting(r.Context(), defaultUserID, store.TypeSetting{
		Kind: kind, Depth: depth, Step: step,
	}); err != nil {
		s.fail(w, r, err)
		return
	}

	rows, err := s.settingRows(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	for _, row := range rows {
		if row.Kind == kind {
			s.renderFragment(w, r, "setting-row", row)
			return
		}
	}
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
