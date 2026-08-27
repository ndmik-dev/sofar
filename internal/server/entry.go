package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ndmik-dev/sofar/internal/store"
)

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	now := time.Now().In(s.cfg.Loc)

	entry, err := s.store.SoftDelete(r.Context(), id, now)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}

	s.renderSidebands(w, r, removedRow{
		EntryID: id,
		Toast: toastView{
			Text:    entry.Media.Title + " · видалено",
			EntryID: id,
			Restore: true,
		},
	})
}

func (s *Server) handleRating(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	rating, err := strconv.Atoi(r.FormValue("rating"))
	if err != nil || rating < 0 || rating > 10 {
		http.Error(w, "rating must be 0-10", http.StatusBadRequest)
		return
	}

	now, today, _ := s.now()
	entry, err := s.store.SetRating(r.Context(), id, rating, now)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}

	text := entry.Media.Title + " · оцінку знято"
	if rating > 0 {
		text = fmt.Sprintf("%s · оцінка %d", entry.Media.Title, rating)
	}
	s.respondMoveWith(w, r, store.Move{Entry: entry, Changed: true}, today,
		toastView{Text: text, EntryID: entry.ID})
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	_, today, _ := s.now()

	entry, err := s.store.Restore(r.Context(), id)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}

	// Undo means "as it was": the row goes back to the exact spot the list
	// ordering gives it, not to the top.
	rowOOB := "beforeend:#rows"
	if succ, err := s.store.Successor(r.Context(), defaultUserID, entry.Status, entry.UpdatedAt, entry.ID); err == nil && succ > 0 {
		rowOOB = "beforebegin:#entry-" + strconv.FormatInt(succ, 10)
	}

	s.respondAddedAt(w, r, entry, today, toastView{
		Text:    entry.Media.Title + " · повернуто",
		EntryID: entry.ID,
	}, false, true, rowOOB)
}

func (s *Server) handleNote(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	now, today, _ := s.now()

	entry, err := s.store.SetNote(r.Context(), id, strings.TrimSpace(r.FormValue("note")), now)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}
	s.respondMoveWith(w, r, store.Move{Entry: entry, Changed: true}, today,
		toastView{Text: entry.Media.Title + " · нотатку збережено", EntryID: entry.ID})
}
