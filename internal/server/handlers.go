package server

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ndmik-dev/sofar/internal/domain"
	"github.com/ndmik-dev/sofar/internal/store"
)

const (
	defaultUserID = 1
	staleAfter    = 30 * 24 * time.Hour
)

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

// clock is one request's idea of the time, taken once so every helper down
// the line agrees on which day it is and what counts as stale.
type clock struct {
	Now         time.Time
	Today       string
	StaleBefore int64
}

func (s *Server) now() clock {
	now := time.Now().In(s.cfg.Loc)
	return clock{Now: now, Today: s.cfg.Day(now).Format("2006-01-02"), StaleBefore: now.Add(-staleAfter).Unix()}
}

func (s *Server) handleAdvance(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	c := s.now()

	var abs *int
	if raw := r.FormValue("abs"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "abs must be a number", http.StatusBadRequest)
			return
		}
		abs = &v
	}
	// Season and episode are the way a person remembers a series; the app
	// counts in one number, so the translation happens here.
	if raw := r.FormValue("season"); raw != "" {
		season, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "season must be a number", http.StatusBadRequest)
			return
		}
		episode, err := strconv.Atoi(orDefault(r.FormValue("episode"), "0"))
		if err != nil {
			http.Error(w, "episode must be a number", http.StatusBadRequest)
			return
		}
		entry, err := s.store.GetEntry(r.Context(), id)
		if err != nil {
			s.failEntry(w, r, err)
			return
		}
		v := domain.ResolveEpisode(entry.Units, season, episode)
		abs = &v
	}

	delta, err := strconv.Atoi(orDefault(r.FormValue("n"), "1"))
	if err != nil {
		http.Error(w, "n must be a number", http.StatusBadRequest)
		return
	}

	source := "manual"
	if r.FormValue("setup") != "" {
		source = "setup"
	}

	move, err := s.store.AdvanceFrom(r.Context(), id, delta, abs, c.Now, source)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}
	s.respondMove(w, r, move, c)
}

func (s *Server) handleUndo(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	c := s.now()

	move, err := s.store.Undo(r.Context(), id, c.Now)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}
	if !move.Changed {
		s.respondMoveWith(w, r, move, c, toastView{Text: "Нема чого скасовувати"})
		return
	}
	s.respondMove(w, r, move, c)
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

	c := s.now()
	before, err := s.store.GetEntry(r.Context(), id)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}
	entry, err := s.store.SetStatus(r.Context(), id, status, c.Now)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}

	// A row that no longer belongs on the open list has to leave it, the same
	// way a deleted one does — otherwise it lingers until a reload.
	if !viewingList(r, entry.Status) {
		s.renderSidebands(w, r, c, removedRow{
			EntryID: id,
			Toast: toastView{
				Text:       entry.Media.Title + " · " + statusWords[entry.Status],
				EntryID:    id,
				BackStatus: before.Status,
			},
		})
		return
	}
	s.respondMoveWith(w, r, store.Move{Entry: entry, Changed: true}, c,
		toastView{Text: entry.Media.Title + " · " + statusWords[entry.Status], EntryID: id})
}

func (s *Server) sidebands(r *http.Request, c clock) (summaryView, navView, error) {
	base := currentBase(r)
	sum, err := s.store.Summary(r.Context(), defaultUserID, c.Today, c.StaleBefore)
	if err != nil {
		return summaryView{}, navView{}, err
	}
	statusCounts, err := s.store.CountsByStatus(r.Context(), defaultUserID)
	if err != nil {
		return summaryView{}, navView{}, err
	}
	kindCounts, err := s.store.CountsByKind(r.Context(), defaultUserID, pathStatus[base])
	if err != nil {
		return summaryView{}, navView{}, err
	}

	view := summaryFrom(sum)
	view.Next = s.nextUp(r.Context(), c.Today, c.StaleBefore)
	view.OOB = true
	view.Render = base == "/active"
	view.Off = view.Off || currentKind(r) != ""
	return view, navViewFrom(statusCounts, kindCounts, base, true), nil
}

func (s *Server) renderSidebands(w http.ResponseWriter, r *http.Request, c clock, removed removedRow) {
	sum, nav, err := s.sidebands(r, c)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	removed.Summary, removed.Nav = sum, nav
	s.renderFragment(w, r, "removed-row", removed)
}

func (s *Server) respondAdded(w http.ResponseWriter, r *http.Request, entry store.Entry, c clock, toast toastView, created, showRow bool) {
	s.respondAddedAt(w, r, entry, c, toast, created, showRow, "afterbegin:#rows")
}

func (s *Server) respondAddedAt(w http.ResponseWriter, r *http.Request, entry store.Entry, c clock, toast toastView, created, showRow bool, rowOOB string) {
	sum, nav, err := s.sidebands(r, c)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	// The archive groups rows by year, so a bare prepend would land outside
	// the groups — rows only insert live on flat lists.
	resp := addedRow{
		Row:     buildRow(entry, c.Today),
		Toast:   toast,
		Summary: sum,
		Nav:     nav,
		Step:    positionStepFor(entry, created),
		ShowRow: showRow && entry.Status != "done" && viewingList(r, entry.Status),
		RowOOB:  rowOOB,
	}

	// Backfilling the archive is a rhythm, not a dialogue: anything that does
	// not land in "active" clears the field and waits for the next title.
	if created && entry.Status != "active" {
		resp.Flow = &flowItem{
			Title:  entry.Media.Title,
			Kind:   entry.Media.Kind,
			Status: entry.Status,
			Label:  statusWords[entry.Status],
		}
		w.Header().Set("HX-Trigger", "sofar:flow")
	}

	s.renderFragment(w, r, "added-row", resp)
}

func (s *Server) respondMove(w http.ResponseWriter, r *http.Request, move store.Move, c clock) {
	s.respondMoveWith(w, r, move, c, toastFor(move))
}

func (s *Server) respondMoveWith(w http.ResponseWriter, r *http.Request, move store.Move, c clock, toast toastView) {
	// The sidebar rides along: a status change moves a title between lists,
	// and counts that only refresh on the next click read as broken.
	sum, nav, err := s.sidebands(r, c)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	s.renderFragment(w, r, "move-response", moveResponse{
		Row:     buildRow(move.Entry, c.Today),
		Summary: sum,
		Nav:     nav,
		Toast:   toast,
	})
}

// currentBase is the list page the request was made from, so out-of-band nav
// updates highlight the page the user is looking at, not a hardcoded one.
func currentBase(r *http.Request) string {
	raw := r.Header.Get("HX-Current-URL")
	if raw == "" {
		return "/active"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "/active"
	}
	if _, ok := pathStatus[u.Path]; ok {
		return u.Path
	}
	return "/active"
}

// htmx sends the page the request came from, which is the only way the server
// can tell whether a freshly added row belongs on screen right now.
// A kind filter narrows the list but not the figures above it, so the bar
// hides rather than describe something else.
func currentKind(r *http.Request) string {
	u, err := url.Parse(r.Header.Get("HX-Current-URL"))
	if err != nil {
		return ""
	}
	return u.Query().Get("kind")
}

func viewingList(r *http.Request, status string) bool {
	raw := r.Header.Get("HX-Current-URL")
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Path == statusPaths[status]
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
