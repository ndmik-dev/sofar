package server

import (
	"net/http"
	"time"
)

type navItem struct {
	Label  string
	Href   string
	Count  int
	Active bool
}

type kindItem struct {
	Label string
	Color string
	Count int
}

type listPage struct {
	Title  string
	Nav    []navItem
	Kinds  []kindItem
	Now    time.Time
	Streak int
	Empty  bool
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
	s.render(w, r, "active.html", listPage{
		Title: "У процесі",
		Nav: []navItem{
			{Label: "У процесі", Href: "/active", Active: true},
			{Label: "Колись", Href: "/backlog"},
			{Label: "Завершено", Href: "/done"},
			{Label: "Кинуто", Href: "/dropped"},
		},
		Kinds: []kindItem{
			{Label: "Серіали", Color: "show"},
			{Label: "Аніме", Color: "anime"},
			{Label: "Фільми", Color: "movie"},
			{Label: "Книги", Color: "book"},
			{Label: "Ігри", Color: "game"},
			{Label: "Подкасти", Color: "podcast"},
		},
		Now:   time.Now().In(s.cfg.Loc),
		Empty: true,
	})
}
