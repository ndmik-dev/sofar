package server

import (
	"net/http"
	"time"
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

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	now, today, _ := s.now()

	entry, err := s.store.Restore(r.Context(), id, now)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}

	s.respondAdded(w, r, entry, today, toastView{
		Text:    entry.Media.Title + " · повернуто",
		EntryID: entry.ID,
	}, false, true)
}
