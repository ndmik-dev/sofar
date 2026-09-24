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
	Backup   backupView
}

func (s *Server) settingRows(r *http.Request) ([]settingRow, error) {
	saved, err := s.store.TypeSettings(r.Context(), defaultUserID)
	if err != nil {
		return nil, err
	}
	var out []settingRow
	for _, k := range kinds {
		row := settingRow{
			Kind:       k.Kind,
			Label:      k.Nav,
			Depth:      "units",
			Step:       k.Steps[0],
			UnitsLabel: unitsSwitchLabel(k),
			Steps:      k.Steps,
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
	kindCounts, err := s.store.CountsByKind(r.Context(), defaultUserID, "active")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	backup, err := s.backupView()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "settings.html", settingsPage{
		Title:    "Налаштування",
		NavView:  navViewFrom(statusCounts, kindCounts, "/settings", false),
		Now:      time.Now().In(s.cfg.Loc),
		Settings: rows,
		Backup:   backup,
	})
}

func (s *Server) handleSetSetting(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	meta, ok := kindByName[kind]
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
