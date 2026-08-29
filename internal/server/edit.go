package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ndmik-dev/sofar/internal/store"
)

func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "Назва не може бути порожньою", http.StatusBadRequest)
		return
	}

	ed := store.Edit{
		Title:    title,
		Subtitle: strings.TrimSpace(r.FormValue("subtitle")),
	}
	// An absent field means "not editable here", an empty one means "no limit".
	if raw, present := r.Form["total"]; present {
		n, err := strconv.Atoi(strings.TrimSpace(raw[0]))
		if err != nil && strings.TrimSpace(raw[0]) != "" {
			http.Error(w, "Всього має бути числом", http.StatusBadRequest)
			return
		}
		ed.SetTotal, ed.Total = true, n
	}

	now, today, _ := s.now()
	entry, err := s.store.UpdateDetails(r.Context(), id, ed, now)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}
	s.respondMoveWith(w, r, store.Move{Entry: entry, Changed: true}, today,
		toastView{Text: entry.Media.Title + " · збережено", EntryID: entry.ID})
}
