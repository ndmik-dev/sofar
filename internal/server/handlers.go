package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ndmik-dev/sofar/internal/domain"
	"github.com/ndmik-dev/sofar/internal/store"
)

const (
	defaultUserID = 1
	staleAfter    = 30 * 24 * time.Hour
)

var kindLabels = map[string]string{
	"show":    "серіал",
	"anime":   "аніме",
	"movie":   "фільм",
	"book":    "книга",
	"game":    "гра",
	"podcast": "подкаст",
}

var kindNav = []struct{ Kind, Label string }{
	{"show", "Серіали"},
	{"anime", "Аніме"},
	{"movie", "Фільми"},
	{"book", "Книги"},
	{"game", "Ігри"},
	{"podcast", "Подкасти"},
}

type navItem struct {
	Label  string
	Href   string
	Count  int
	Active bool
}

type kindItem struct {
	Label string
	Kind  string
	Count int
}

type row struct {
	EntryID   int64
	Title     string
	Sub       string
	Kind      string
	KindLabel string
	Mode      string
	Cells     []domain.Cell
	Percent   int
	Status    string
	Pos       string
	PosSub    string
	Btn       string
	Stale     bool
	Selected  bool
}

type summary struct {
	Waiting     int
	WaitingTime string
	Airing      int
}

type listPage struct {
	Title   string
	Nav     []navItem
	Kinds   []kindItem
	Now     time.Time
	Rows    []row
	Stale   []row
	Summary summary
	Empty   bool
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/active", http.StatusFound)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DB.PingContext(r.Context()); err != nil {
		s.log.Error("healthz", "err", err)
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleActive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now().In(s.cfg.Loc)
	today := s.cfg.Day(now).Format("2006-01-02")

	entries, err := s.store.ListEntries(ctx, defaultUserID, "active")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	statusCounts, err := s.store.CountsByStatus(ctx, defaultUserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	kindCounts, err := s.store.CountsByKind(ctx, defaultUserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	page := listPage{
		Title: "У процесі",
		Nav: []navItem{
			{Label: "У процесі", Href: "/active", Count: statusCounts["active"], Active: true},
			{Label: "Колись", Href: "/backlog", Count: statusCounts["backlog"]},
			{Label: "Завершено", Href: "/done", Count: statusCounts["done"]},
			{Label: "Кинуто", Href: "/dropped", Count: statusCounts["dropped"]},
		},
		Now: now,
	}
	for _, k := range kindNav {
		page.Kinds = append(page.Kinds, kindItem{Label: k.Label, Kind: k.Kind, Count: kindCounts[k.Kind]})
	}

	staleBefore := now.Add(-staleAfter).Unix()
	var waitingMins int
	for i, e := range entries {
		rw := buildRow(e, today)
		rw.Selected = i == 0
		if e.UpdatedAt < staleBefore {
			rw.Stale = true
			page.Stale = append(page.Stale, rw)
			continue
		}
		if rem := domain.RemainingFrom(e.Units, e.Position, today); rem.Waiting > 0 {
			page.Summary.Waiting += rem.Waiting
			waitingMins += rem.WaitingMins
		}
		if e.Media.Airing == "returning" {
			page.Summary.Airing++
		}
		page.Rows = append(page.Rows, rw)
	}
	page.Summary.WaitingTime = domain.HoursMins(waitingMins)
	page.Empty = len(page.Rows) == 0 && len(page.Stale) == 0

	s.render(w, r, "active.html", page)
}

func buildRow(e store.Entry, today string) row {
	rw := row{
		EntryID:   e.ID,
		Title:     e.Media.Title,
		Kind:      e.Media.Kind,
		KindLabel: kindLabels[e.Media.Kind],
		Btn:       "+",
	}
	if e.Step > 1 {
		rw.Btn = fmt.Sprintf("+%d", e.Step)
	}

	total := int(e.Media.TotalUnits.Int64)
	hasTotal := e.Media.TotalUnits.Valid

	switch {
	case e.Depth == "status":
		rw.Mode = "status"
		rw.Status = e.Status
		rw.Btn = "✓"
		rw.Pos = "—"
		rw.Sub = e.Media.TitleOrig
	case !hasTotal:
		rw.Mode = "open"
		rw.Pos = fmt.Sprintf("%d", e.Position)
		rw.PosSub = "без межі"
		rw.Sub = e.Media.TitleOrig
	case len(e.Units) > 0 && total <= 60:
		rw.Mode = "cells"
		rw.Cells = domain.BuildTrack(e.Units, e.Position, today)
		rw.Pos = fmt.Sprintf("%d / %d", e.Position, total)
		rem := domain.RemainingFrom(e.Units, e.Position, today)
		rw.PosSub = remainingLabel(rem)
		if next := domain.NextLabel(e.Units, e.Position); next != "" {
			rw.Sub = "далі " + next
		} else {
			rw.Sub = "усе переглянуто"
		}
	default:
		rw.Mode = "bar"
		if total > 0 {
			rw.Percent = e.Position * 100 / total
		}
		rw.Pos = fmt.Sprintf("%d / %d", e.Position, total)
		rw.PosSub = fmt.Sprintf("%d%%", rw.Percent)
		rw.Sub = e.Media.TitleOrig
	}

	return rw
}

func remainingLabel(rem domain.Remaining) string {
	if rem.Left == 0 {
		return "завершено"
	}
	if rem.Waiting > 0 && rem.Waiting < rem.Left {
		return fmt.Sprintf("%d чекають", rem.Waiting)
	}
	if t := domain.HoursMins(rem.LeftMins); t != "" {
		return fmt.Sprintf("%d · %s", rem.Left, t)
	}
	return fmt.Sprintf("лишилось %d", rem.Left)
}
