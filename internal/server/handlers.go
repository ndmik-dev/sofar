package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

var unitNames = map[string]string{
	"episode": "серій",
	"page":    "сторінок",
	"hour":    "год",
	"chapter": "розділів",
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
	Step      int
	Statuses  []statusOption
	Stale     bool
	Selected  bool
}

type summaryView struct {
	Waiting     int
	WaitingTime string
	Airing      int
	Active      int
	OOB         bool
}

type statusOption struct {
	Value string
	Label string
	On    bool
}

type toastView struct {
	Text    string
	EntryID int64
	Undo    bool
}

type listPage struct {
	Title   string
	Nav     []navItem
	Kinds   []kindItem
	Now     time.Time
	Rows    []row
	Stale   []row
	Summary summaryView
	Empty   bool
}

type moveResponse struct {
	Row     row
	Summary summaryView
	Toast   toastView
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

func (s *Server) now() (time.Time, string, int64) {
	now := time.Now().In(s.cfg.Loc)
	return now, s.cfg.Day(now).Format("2006-01-02"), now.Add(-staleAfter).Unix()
}

func (s *Server) handleActive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now, today, staleBefore := s.now()

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
	sum, err := s.store.Summary(ctx, defaultUserID, today, staleBefore)
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
		Now:     now,
		Summary: summaryFrom(sum),
	}
	for _, k := range kindNav {
		page.Kinds = append(page.Kinds, kindItem{Label: k.Label, Kind: k.Kind, Count: kindCounts[k.Kind]})
	}

	for i, e := range entries {
		rw := buildRow(e, today)
		rw.Selected = i == 0
		if e.UpdatedAt < staleBefore {
			rw.Stale = true
			page.Stale = append(page.Stale, rw)
			continue
		}
		page.Rows = append(page.Rows, rw)
	}
	page.Empty = len(page.Rows) == 0 && len(page.Stale) == 0

	s.render(w, r, "active.html", page)
}

func (s *Server) handleAdvance(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	now, today, _ := s.now()

	var abs *int
	if raw := r.FormValue("abs"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "abs must be a number", http.StatusBadRequest)
			return
		}
		abs = &v
	}
	delta, err := strconv.Atoi(orDefault(r.FormValue("n"), "1"))
	if err != nil {
		http.Error(w, "n must be a number", http.StatusBadRequest)
		return
	}

	move, err := s.store.Advance(r.Context(), id, delta, abs, now)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}
	s.respondMove(w, r, move, today)
}

func (s *Server) handleUndo(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	now, today, _ := s.now()

	move, err := s.store.Undo(r.Context(), id, now)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}
	s.respondMove(w, r, move, today)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	status := r.FormValue("status")
	switch status {
	case "backlog", "active", "done", "dropped":
	default:
		http.Error(w, "unknown status", http.StatusBadRequest)
		return
	}

	now, today, _ := s.now()
	entry, err := s.store.SetStatus(r.Context(), id, status, now)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}
	s.respondMove(w, r, store.Move{Entry: entry, Changed: true}, today)
}

func (s *Server) respondMove(w http.ResponseWriter, r *http.Request, move store.Move, today string) {
	_, _, staleBefore := s.now()
	sum, err := s.store.Summary(r.Context(), defaultUserID, today, staleBefore)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	view := summaryFrom(sum)
	view.OOB = true
	resp := moveResponse{
		Row:     buildRow(move.Entry, today),
		Summary: view,
		Toast:   toastFor(move),
	}
	s.renderFragment(w, r, "move-response", resp)
}

func toastFor(move store.Move) toastView {
	if !move.Changed {
		return toastView{}
	}
	unit := unitNames[move.Entry.Media.Unit]
	switch {
	case move.To < move.From:
		return toastView{
			Text:    fmt.Sprintf("%s · назад на %d", move.Entry.Media.Title, move.To),
			EntryID: move.Entry.ID,
			Undo:    true,
		}
	case move.Finished:
		return toastView{Text: move.Entry.Media.Title + " · завершено", EntryID: move.Entry.ID, Undo: true}
	case unit != "":
		return toastView{
			Text:    fmt.Sprintf("%s · %d %s", move.Entry.Media.Title, move.To, unit),
			EntryID: move.Entry.ID,
			Undo:    true,
		}
	default:
		return toastView{Text: move.Entry.Media.Title + " · оновлено", EntryID: move.Entry.ID}
	}
}

func (s *Server) entryID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad entry id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func (s *Server) failEntry(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Такого запису немає.", http.StatusNotFound)
		return
	}
	s.fail(w, r, err)
}

func summaryFrom(s store.Summary) summaryView {
	return summaryView{
		Waiting:     s.Waiting,
		WaitingTime: domain.HoursMins(s.WaitingMins),
		Airing:      s.Airing,
		Active:      s.Active,
	}
}

func buildRow(e store.Entry, today string) row {
	rw := row{
		EntryID:   e.ID,
		Title:     e.Media.Title,
		Kind:      e.Media.Kind,
		KindLabel: kindLabels[e.Media.Kind],
		Step:      max(e.Step, 1),
		Btn:       "+",
	}
	if rw.Step > 1 {
		rw.Btn = fmt.Sprintf("+%d", rw.Step)
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
		rw.Statuses = []statusOption{
			{"backlog", "хочу", e.Status == "backlog"},
			{"active", "у процесі", e.Status == "active"},
			{"done", "завершив", e.Status == "done"},
		}
	case !hasTotal:
		rw.Mode = "open"
		rw.Pos = strconv.Itoa(e.Position)
		rw.PosSub = "без межі"
		rw.Sub = e.Media.TitleOrig
	case len(e.Units) > 0 && total <= 60:
		rw.Mode = "cells"
		rw.Cells = domain.BuildTrack(e.Units, e.Position, today)
		rw.Pos = fmt.Sprintf("%d / %d", e.Position, total)
		rw.PosSub = remainingLabel(domain.RemainingFrom(e.Units, e.Position, today))
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

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
