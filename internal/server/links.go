package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ndmik-dev/sofar/internal/store"
)

// Known hosts get their own name; everything else is labelled by its domain.
// Nobody wants to type "Netflix" after pasting a netflix.com link.
var siteNames = map[string]string{
	"netflix.com":            "Netflix",
	"youtube.com":            "YouTube",
	"youtu.be":               "YouTube",
	"megogo.net":             "Megogo",
	"sweet.tv":               "SWEET.TV",
	"kinogo.uk":              "Кіно",
	"uaserials.pro":          "UASerials",
	"anitube.in.ua":          "AniTube",
	"crunchyroll.com":        "Crunchyroll",
	"goodreads.com":          "Goodreads",
	"yakaboo.ua":             "Yakaboo",
	"book-ye.com.ua":         "Книгарня Є",
	"store.steampowered.com": "Steam",
	"gog.com":                "GOG",
	"coursera.org":           "Coursera",
	"udemy.com":              "Udemy",
	"themoviedb.org":         "TMDB",
	"imdb.com":               "IMDb",
	"mangadex.org":           "MangaDex",
	"myanimelist.net":        "MyAnimeList",
}

// normalizeLink accepts what a person actually pastes. Only http and https are
// allowed: a link is opened in the browser, and javascript: in an href is the
// oldest trick there is.
func normalizeLink(raw string) (link, label string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	if !strings.Contains(raw, "//") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", "", false
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return "", "", false
	}

	host := strings.ToLower(u.Hostname())
	if name, known := siteNames[host]; known {
		return u.String(), name, true
	}
	trimmed := strings.TrimPrefix(host, "www.")
	if name, known := siteNames[trimmed]; known {
		return u.String(), name, true
	}
	return u.String(), trimmed, true
}

func (s *Server) handleAddLink(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	link, label, valid := normalizeLink(r.FormValue("url"))
	if !valid {
		http.Error(w, "Це не схоже на посилання", http.StatusBadRequest)
		return
	}
	if given := strings.TrimSpace(r.FormValue("label")); given != "" {
		label = given
	}

	c := s.now()
	if err := s.store.AddLink(r.Context(), id, link, label, c.Now); err != nil {
		s.fail(w, r, err)
		return
	}
	s.respondEntry(w, r, id, c, toastView{Text: label + " · посилання додано"})
}

func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	linkID, err := strconv.ParseInt(r.PathValue("link"), 10, 64)
	if err != nil {
		http.Error(w, "bad link id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteLink(r.Context(), id, linkID); err != nil {
		s.fail(w, r, err)
		return
	}
	c := s.now()
	s.respondEntry(w, r, id, c, toastView{Text: "Посилання прибрано"})
}

func (s *Server) respondEntry(w http.ResponseWriter, r *http.Request, id int64, c clock, toast toastView) {
	entry, err := s.store.GetEntry(r.Context(), id)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}
	s.respondMoveWith(w, r, store.Move{Entry: entry, Changed: true}, c, toast)
}
