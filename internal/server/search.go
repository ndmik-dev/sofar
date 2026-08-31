package server

import (
	"database/sql"
	"fmt"
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
	"н": "course", "c": "course",
	"м": "manga", "манга": "manga", "manga": "manga",
}

var manualKinds = map[string]bool{"book": true, "game": true, "course": true, "manga": true}

// Kinds a catalog can answer for. Courses have no such thing anywhere, so a
// missing-key message would be a lie rather than a hint.
var catalogKinds = map[string]bool{"book": true, "game": true}

var unitForKind = map[string]string{
	"show": "episode", "anime": "episode",
	"book": "page", "game": "hour",
	"movie":  "minute",
	"course": "lesson", "manga": "chapter",
}

var unitLabels = map[string]string{
	"episode": "серій", "page": "сторінок", "hour": "годин",
	"chapter": "розділів", "lesson": "уроків", "minute": "хвилин", "none": "",
}

type searchResult struct {
	TMDBType  string
	TMDBID    int
	Source    string
	ExtID     string
	Kind      string
	KindLabel string
	Title     string
	Original  string
	Meta      string
	Total     int
	Cover     string
	NeedsForm bool
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
	Secondary bool
	Total     int
	Position  int
	Subtitle  string
	// The second line means something different per type, and a book with no
	// author is the one case where an empty field is worth offering anyway.
	SubtitleLabel       string
	SubtitlePlaceholder string
	Source              string
	ExtID               string
	Cover               string
}

type shelfResult struct {
	EntryID   int64
	Title     string
	Sub       string
	Kind      string
	KindLabel string
	Where     string
	Href      string
	Pos       string
}

type searchView struct {
	Query    string
	Kind     string
	Shelf    []shelfResult
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

// What you already have comes first: reaching a title you own is a different
// job from adding one, and the palette is the only search box there is.
func (s *Server) shelfMatches(r *http.Request, kind, q string) []shelfResult {
	if len([]rune(q)) < 2 {
		return nil
	}
	found, err := s.store.SearchShelf(r.Context(), defaultUserID, q, 5)
	if err != nil {
		s.log.Error("shelf search", "q", q, "err", err)
		return nil
	}

	var out []shelfResult
	for _, f := range found {
		if kind != "" && f.Kind != kind {
			continue
		}
		res := shelfResult{
			EntryID:   f.EntryID,
			Title:     f.Title,
			Sub:       f.Sub,
			Kind:      f.Kind,
			KindLabel: kindLabels[f.Kind],
			Where:     statusWords[f.Status],
			Href:      fmt.Sprintf("%s?focus=%d", statusPaths[f.Status], f.EntryID),
		}
		if f.HasEnd && f.Total > 1 {
			res.Pos = fmt.Sprintf("%d / %d", f.Pos, f.Total)
		}
		out = append(out, res)
	}
	return out
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
	f.SubtitleLabel, f.SubtitlePlaceholder = subtitleField(kind)
	for _, k := range kindNav {
		f.Kinds = append(f.Kinds, kindChoice{Kind: k.Kind, Label: kindLabels[k.Kind], On: k.Kind == kind})
	}
	return f
}

func subtitleField(kind string) (label, placeholder string) {
	switch kind {
	case "book":
		return "Автор", "можна пропустити"
	case "game":
		return "Платформа", "PC, PS5, Switch…"
	case "manga":
		return "Автор", "мангака"
	case "course":
		return "Платформа", "Coursera, YouTube, курси в компанії…"
	}
	return "Оригінал", "назва мовою оригіналу"
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	kind, q := parseQuery(r.URL.Query().Get("q"))
	view := searchView{Query: q, Kind: kind}
	view.Shelf = s.shelfMatches(r, kind, q)

	switch {
	case manualKinds[kind]:
		s.fillOwnCatalog(w, r, &view, kind, q)
		return
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
			// A plain query only ever reaches TMDB, so a book will always come
			// back empty here. Say where it lives instead of just "nothing".
			view.Message = "TMDB нічого не знає. Книги — к:, манґа — м:, ігри — і:, курси — н:"
			view.Manual = manualFormFor(q, "")
		} else {
			view.Results[0].First = true
		}
	}

	s.renderFragment(w, r, "search-results", view)
}

// Books and games each have their own catalog. Whatever it returns is
// a suggestion: the last row always offers the manual form, and a book carries
// its page count into an editable field rather than straight into the database.
func (s *Server) fillOwnCatalog(w http.ResponseWriter, r *http.Request, view *searchView, kind, q string) {
	defer func() { s.renderFragment(w, r, "search-results", view) }()

	if len([]rune(q)) < 2 {
		view.Manual = manualFormFor(q, kind)
		return
	}
	if !catalogKinds[kind] {
		view.Manual = manualFormFor(q, kind)
		return
	}
	if !s.catalog.HasProvider(kind) {
		view.Message = "Ключ каталогу не заданий — лишається свій запис"
		view.Manual = manualFormFor(q, kind)
		return
	}

	found, err := s.catalog.FindByKind(r.Context(), kind, q, 6)
	if err != nil {
		s.log.Error("catalog search", "kind", kind, "q", q, "err", err)
		view.Message = "Каталог не відповідає — можна завести вручну"
		view.Manual = manualFormFor(q, kind)
		return
	}

	for i, f := range found {
		view.Results = append(view.Results, searchResult{
			Source:    f.Source,
			ExtID:     f.ExtID,
			Kind:      f.Kind,
			KindLabel: kindLabels[f.Kind],
			Title:     f.Title,
			Original:  f.Subtitle,
			Meta:      yearLabel(f.Year),
			Total:     f.Total,
			Cover:     f.Cover,
			NeedsForm: kind == "book",
			First:     i == 0,
		})
	}
	if len(view.Results) == 0 {
		view.Message = "У каталозі нічого"
	}
	view.Manual = manualFormFor(q, kind)
	view.Manual.Secondary = len(view.Results) > 0
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

// handleManualForm prefills the form from a catalog hit, so a book arrives with
// its author and a page count you can correct before saving.
func (s *Server) handleManualForm(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := q.Get("kind")
	if _, known := unitForKind[kind]; !known {
		http.Error(w, "unknown kind", http.StatusBadRequest)
		return
	}
	form := manualFormFor(q.Get("title"), kind)
	form.Total, _ = strconv.Atoi(q.Get("total"))
	form.Subtitle = q.Get("subtitle")
	form.Source = q.Get("source")
	form.ExtID = q.Get("ext_id")
	form.Cover = q.Get("cover")
	s.renderFragment(w, r, "manual-form", form)
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
	in := manualMedia(kind, title, total)
	in.TitleOrig = strings.TrimSpace(r.FormValue("subtitle"))
	in.PosterPath = r.FormValue("cover")
	in.ExtID = r.FormValue("ext_id")
	if src := r.FormValue("source"); src != "" && in.ExtID != "" {
		in.Source = src
	}
	if y, _ := strconv.Atoi(r.FormValue("year")); y > 0 {
		in.Year = y
	}

	mediaID, err := s.store.UpsertMedia(r.Context(), in, nil, now)
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
	s.respondAdded(w, r, entry, today, toast, created, created)
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
