package server

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ndmik-dev/sofar/internal/store"
)

// Prefixes narrow the search to one type. The manual-only kinds jump straight
// to the form: there is no catalog behind them, so searching would be theatre.
var kindPrefixes = map[string]string{
	"с": "show", "s": "show",
	"а": "anime", "a": "anime",
	"ф": "movie", "f": "movie", "m": "movie",
	"к": "book", "b": "book",
	"і": "game", "и": "game", "g": "game",
	"п": "podcast", "p": "podcast",
}

var manualKinds = map[string]bool{"book": true, "game": true, "podcast": true}

var unitForKind = map[string]string{
	"show": "episode", "anime": "episode",
	"book": "page", "game": "hour",
	"podcast": "none", "movie": "none",
}

var unitLabels = map[string]string{
	"episode": "серій", "page": "сторінок", "hour": "годин",
	"chapter": "розділів", "none": "",
}

type searchResult struct {
	TMDBType  string
	TMDBID    int
	Kind      string
	KindLabel string
	Title     string
	Original  string
	Meta      string
	First     bool
}

type kindChoice struct {
	Kind  string
	Label string
	On    bool
}

type manualForm struct {
	Title     string
	Kind      string
	KindLabel string
	Unit      string
	UnitLabel string
	Kinds     []kindChoice
	Open      bool
}

type searchView struct {
	Query    string
	Kind     string
	Results  []searchResult
	Manual   *manualForm
	Message  string
	Disabled bool
}

func parseQuery(raw string) (kind, rest string) {
	raw = strings.TrimSpace(raw)
	prefix, after, found := strings.Cut(raw, ":")
	if !found {
		return "", raw
	}
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	k, ok := kindPrefixes[prefix]
	if !ok {
		return "", raw
	}
	return k, strings.TrimSpace(after)
}

func manualFormFor(title, kind string) *manualForm {
	if kind == "" {
		kind = "book"
	}
	unit := unitForKind[kind]
	f := &manualForm{
		Title:     title,
		Kind:      kind,
		KindLabel: kindLabels[kind],
		Unit:      unit,
		UnitLabel: unitLabels[unit],
		Open:      unit == "none",
	}
	for _, k := range kindNav {
		f.Kinds = append(f.Kinds, kindChoice{Kind: k.Kind, Label: kindLabels[k.Kind], On: k.Kind == kind})
	}
	return f
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	kind, q := parseQuery(r.URL.Query().Get("q"))
	view := searchView{Query: q, Kind: kind}

	switch {
	case manualKinds[kind]:
		view.Manual = manualFormFor(q, kind)
	case len([]rune(q)) < 2:
		view.Message = "Введи щонайменше дві літери"
	case !s.catalog.Enabled():
		view.Disabled = true
		view.Message = "TMDB_TOKEN не заданий — лишається свій запис"
		view.Manual = manualFormFor(q, kind)
	default:
		results, err := s.catalog.Search(r.Context(), q, 8)
		if err != nil {
			s.log.Error("tmdb search", "q", q, "err", err)
			view.Message = "Каталог не відповідає. Спробуй ще раз."
			break
		}
		for _, res := range results {
			if kind != "" && res.Kind != kind {
				continue
			}
			view.Results = append(view.Results, searchResult{
				TMDBType:  res.TMDBType,
				TMDBID:    res.TMDBID,
				Kind:      res.Kind,
				KindLabel: kindLabels[res.Kind],
				Title:     res.Title,
				Original:  res.Original,
				Meta:      yearLabel(res.Year),
			})
		}
		if len(view.Results) == 0 {
			view.Message = "У каталозі нічого"
		} else {
			view.Results[0].First = true
		}
	}

	s.renderFragment(w, r, "search-results", view)
}

func yearLabel(y int) string {
	if y == 0 {
		return ""
	}
	return strconv.Itoa(y)
}

func (s *Server) handleAdd(w http.ResponseWriter, r *http.Request) {
	tmdbType := r.FormValue("tmdb_type")
	tmdbID, err := strconv.Atoi(r.FormValue("tmdb_id"))
	if err != nil || tmdbID <= 0 {
		http.Error(w, "bad tmdb id", http.StatusBadRequest)
		return
	}
	status, ok := validStatus(r.FormValue("status"))
	if !ok {
		http.Error(w, "unknown status", http.StatusBadRequest)
		return
	}

	now := time.Now().In(s.cfg.Loc)
	mediaID, err := s.catalog.Import(r.Context(), tmdbType, tmdbID, now)
	if err != nil {
		s.log.Error("tmdb import", "type", tmdbType, "id", tmdbID, "err", err)
		http.Error(w, "Каталог не відповідає. Спробуй ще раз.", http.StatusBadGateway)
		return
	}
	s.finishAdd(w, r, mediaID, status, 0, now)
}

func (s *Server) handleAddManual(w http.ResponseWriter, r *http.Request) {
	kind := r.FormValue("kind")
	if _, known := unitForKind[kind]; !known {
		http.Error(w, "unknown kind", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "Назва не може бути порожньою", http.StatusBadRequest)
		return
	}
	status, ok := validStatus(r.FormValue("status"))
	if !ok {
		http.Error(w, "unknown status", http.StatusBadRequest)
		return
	}

	total, _ := strconv.Atoi(r.FormValue("total"))
	position, _ := strconv.Atoi(r.FormValue("position"))
	if total > 0 && position > total {
		position = total
	}

	now := time.Now().In(s.cfg.Loc)
	mediaID, err := s.store.UpsertMedia(r.Context(), manualMedia(kind, title, total), nil, now)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.finishAdd(w, r, mediaID, status, position, now)
}

func (s *Server) finishAdd(w http.ResponseWriter, r *http.Request, mediaID int64, status string, position int, now time.Time) {
	entryID, created, err := s.store.AddEntry(r.Context(), defaultUserID, mediaID, status, position, now)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	entry, err := s.store.GetEntry(r.Context(), entryID)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	_, today, _ := s.now()
	toast := toastView{Text: entry.Media.Title + " · вже на полиці", EntryID: entryID}
	if created {
		toast = toastView{Text: entry.Media.Title + " · " + statusWords[status], EntryID: entryID, Undo: true}
	}
	s.respondAdded(w, r, entry, today, toast, created)
}

func validStatus(v string) (string, bool) {
	switch v {
	case "":
		return "active", true
	case "backlog", "active", "done", "dropped":
		return v, true
	}
	return "", false
}

func manualMedia(kind, title string, total int) store.MediaInput {
	in := store.MediaInput{
		Kind:   kind,
		Source: "manual",
		Title:  title,
		Unit:   unitForKind[kind],
	}
	if in.Unit == "none" {
		in.TotalUnits = sql.NullInt64{Int64: 1, Valid: true}
		return in
	}
	if total > 0 {
		in.TotalUnits = sql.NullInt64{Int64: int64(total), Valid: true}
	}
	return in
}
