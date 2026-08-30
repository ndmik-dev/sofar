package server

import (
	"database/sql"
	"errors"
	"fmt"
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

var kindLabels = map[string]string{
	"show":    "серіал",
	"anime":   "аніме",
	"movie":   "фільм",
	"book":    "книга",
	"game":    "гра",
	"podcast": "подкаст",
	"course":  "курс",
}

var kindNav = []struct{ Kind, Label string }{
	{"show", "Серіали"},
	{"anime", "Аніме"},
	{"movie", "Фільми"},
	{"book", "Книги"},
	{"game", "Ігри"},
	{"podcast", "Подкасти"},
	{"course", "Курси"},
}

var statusWords = map[string]string{
	"active":  "у процесі",
	"backlog": "у «колись»",
	"done":    "у завершених",
	"dropped": "у кинутих",
}

var unitNames = map[string]string{
	"episode": "серій",
	"page":    "сторінок",
	"hour":    "год",
	"chapter": "розділів",
	"lesson":  "уроків",
	"minute":  "хв",
}

type navItem struct {
	Label  string
	Href   string
	Count  int
	Active bool
}

type kindFilter struct {
	Label string
	Kind  string
	Count int
	Clear string
}

type kindItem struct {
	Label string
	Kind  string
	Href  string
	Count int
	On    bool
}

type row struct {
	EntryID   int64
	Title     string
	Sub       string
	Kind      string
	KindLabel string
	Mode      string
	Blocks    []domain.Block
	Percent   int
	Status    string
	Pos       string
	PosSub    string
	Total     int
	Btn       string
	BtnWide   bool
	Rating    int
	Step      int
	SeasonEnd int
	Statuses  []statusOption
	Aired     string
	LinkURL   string
	Stale     bool
	Selected  bool
}

type summaryView struct {
	Waiting     int
	WaitingTime string
	Airing      int
	Active      int
	OOB         bool
	Render      bool
	Off         bool
}

type statusOption struct {
	Value string
	Label string
	On    bool
}

type toastView struct {
	Text       string
	EntryID    int64
	Undo       bool
	Restore    bool
	BackStatus string
}

type removedRow struct {
	EntryID int64
	Toast   toastView
	Summary summaryView
	Nav     navView
}

type navView struct {
	Nav   []navItem
	Kinds []kindItem
	Kind  string
	Base  string
	OOB   bool
}

type addedRow struct {
	Row     row
	Toast   toastView
	Summary summaryView
	Nav     navView
	Step    *positionStep
	Flow    *flowItem
	ShowRow bool
	RowOOB  string
}

type flowItem struct {
	Title  string
	Kind   string
	Status string
	Label  string
}

type positionStep struct {
	EntryID  int64
	Title    string
	Total    int
	Unit     string
	Hint     string
	BySeason bool
	Seasons  []domain.Season
	LastS    int
	LastE    int
}

type listPage struct {
	Title   string
	NavView navView
	Now     time.Time
	Rows    []row
	Fresh   []row
	Ongoing []row
	Stale   []row
	Years   []yearGroup
	Filters []filterChip
	Filter  *kindFilter
	Summary summaryView
	Empty   bool
}

type moveResponse struct {
	Row     row
	Summary summaryView
	Nav     navView
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
	kindCounts, err := s.store.CountsByKind(ctx, defaultUserID, "active")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	sum, err := s.store.Summary(ctx, defaultUserID, today, staleBefore)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	kind := r.URL.Query().Get("kind")
	page := listPage{
		Title:   "У процесі",
		NavView: navViewWithKind(statusCounts, kindCounts, "/active", kind, false),
		Now:     now,
		Summary: summaryFrom(sum),
	}

	since := s.cfg.Day(now).AddDate(0, 0, -domain.FreshDays).Format("2006-01-02")
	for i, e := range entries {
		if kind != "" && e.Media.Kind != kind {
			continue
		}
		rw := buildRow(e, today)
		rw.Selected = i == 0
		if ongoing(e) {
			page.Ongoing = append(page.Ongoing, rw)
			continue
		}
		if u, ok := domain.JustAired(e.Units, e.Position, since, today); ok {
			rw.Aired = domain.Label(u, domain.MultiSeason(e.Units))
			page.Fresh = append(page.Fresh, rw)
			continue
		}
		if e.UpdatedAt < staleBefore {
			rw.Stale = true
			page.Stale = append(page.Stale, rw)
			continue
		}
		page.Rows = append(page.Rows, rw)
	}
	page.Empty = len(page.Rows) == 0 && len(page.Stale) == 0 &&
		len(page.Fresh) == 0 && len(page.Ongoing) == 0
	// The element stays in the page even with nothing to show, so an
	// out-of-band swap has a target the moment the first row lands. Under a
	// kind filter the figures describe the whole list, not what is on screen,
	// so they stay hidden rather than contradict it.
	page.Summary.Off = page.Empty || kind != ""
	if kind != "" {
		page.Filter = &kindFilter{
			Label: kindLabels[kind],
			Kind:  kind,
			Count: len(page.Rows) + len(page.Stale) + len(page.Fresh) + len(page.Ongoing),
			Clear: "/active",
		}
	}

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

	move, err := s.store.AdvanceFrom(r.Context(), id, delta, abs, now, source)
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
	if !move.Changed {
		s.respondMoveWith(w, r, move, today, toastView{Text: "Нема чого скасовувати"})
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
	before, err := s.store.GetEntry(r.Context(), id)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}
	entry, err := s.store.SetStatus(r.Context(), id, status, now)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}

	// A row that no longer belongs on the open list has to leave it, the same
	// way a deleted one does — otherwise it lingers until a reload.
	if !viewingList(r, entry.Status) {
		s.renderSidebands(w, r, removedRow{
			EntryID: id,
			Toast: toastView{
				Text:       entry.Media.Title + " · " + statusWords[entry.Status],
				EntryID:    id,
				BackStatus: before.Status,
			},
		})
		return
	}
	s.respondMoveWith(w, r, store.Move{Entry: entry, Changed: true}, today,
		toastView{Text: entry.Media.Title + " · " + statusWords[entry.Status], EntryID: id})
}

func (s *Server) sidebands(r *http.Request, today string) (summaryView, navView, error) {
	_, _, staleBefore := s.now()
	base := currentBase(r)
	sum, err := s.store.Summary(r.Context(), defaultUserID, today, staleBefore)
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
	view.OOB = true
	view.Render = base == "/active"
	view.Off = view.Off || currentKind(r) != ""
	return view, navViewFrom(statusCounts, kindCounts, base, true), nil
}

func navViewFrom(statusCounts, kindCounts map[string]int, active string, oob bool) navView {
	return navViewWithKind(statusCounts, kindCounts, active, "", oob)
}

func navViewWithKind(statusCounts, kindCounts map[string]int, active, kind string, oob bool) navView {
	nv := navView{OOB: oob, Kind: kind, Base: active}
	kindBase := active
	if _, isList := pathStatus[active]; !isList {
		// Year and settings do not filter by kind, so their type links lead
		// back to the main list.
		kindBase = "/active"
	}
	for _, n := range []struct{ Label, Href, Key string }{
		{"У процесі", "/active", "active"},
		{"Колись", "/backlog", "backlog"},
		{"Завершено", "/done", "done"},
		{"Кинуто", "/dropped", "dropped"},
	} {
		nv.Nav = append(nv.Nav, navItem{
			Label: n.Label, Href: n.Href, Count: statusCounts[n.Key], Active: n.Href == active,
		})
	}
	for _, k := range kindNav {
		href := kindBase + "?kind=" + k.Kind
		if k.Kind == kind {
			href = kindBase
		}
		nv.Kinds = append(nv.Kinds, kindItem{
			Label: k.Label, Kind: k.Kind, Href: href,
			Count: kindCounts[k.Kind], On: k.Kind == kind,
		})
	}
	return nv
}

func (s *Server) renderSidebands(w http.ResponseWriter, r *http.Request, removed removedRow) {
	_, today, _ := s.now()
	sum, nav, err := s.sidebands(r, today)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	removed.Summary, removed.Nav = sum, nav
	s.renderFragment(w, r, "removed-row", removed)
}

func (s *Server) respondAdded(w http.ResponseWriter, r *http.Request, entry store.Entry, today string, toast toastView, created, showRow bool) {
	s.respondAddedAt(w, r, entry, today, toast, created, showRow, "afterbegin:#rows")
}

func (s *Server) respondAddedAt(w http.ResponseWriter, r *http.Request, entry store.Entry, today string, toast toastView, created, showRow bool, rowOOB string) {
	sum, nav, err := s.sidebands(r, today)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	// The archive groups rows by year, so a bare prepend would land outside
	// the groups — rows only insert live on flat lists.
	resp := addedRow{
		Row:     buildRow(entry, today),
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

// Asking "where did you stop" only makes sense for something you are actually
// watching, that has units to count, and that is still at zero.
func positionStepFor(e store.Entry, created bool) *positionStep {
	if !created || e.Status != "active" || e.Depth == "status" {
		return nil
	}
	if !e.Media.TotalUnits.Valid || e.Position > 0 {
		return nil
	}
	total := int(e.Media.TotalUnits.Int64)
	if total <= 1 {
		return nil
	}
	unit := unitNames[e.Media.Unit]
	hint := fmt.Sprintf("усього %d", total)
	if unit != "" {
		hint += " " + unit
	}
	step := &positionStep{
		EntryID: e.ID,
		Title:   e.Media.Title,
		Total:   total,
		Unit:    unit,
		Hint:    hint,
	}

	// Asking for an absolute episode number is asking someone to add up season
	// lengths in their head. With more than one season, ask the way people
	// actually remember it.
	if seasons := domain.Seasons(e.Units); len(seasons) > 1 {
		step.BySeason = true
		step.Seasons = seasons
		last := seasons[len(seasons)-1]
		step.LastS, step.LastE = last.Number, last.Episodes
	}
	return step
}

func (s *Server) respondMove(w http.ResponseWriter, r *http.Request, move store.Move, today string) {
	s.respondMoveWith(w, r, move, today, toastFor(move))
}

func (s *Server) respondMoveWith(w http.ResponseWriter, r *http.Request, move store.Move, today string, toast toastView) {
	// The sidebar rides along: a status change moves a title between lists,
	// and counts that only refresh on the next click read as broken.
	sum, nav, err := s.sidebands(r, today)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	s.renderFragment(w, r, "move-response", moveResponse{
		Row:     buildRow(move.Entry, today),
		Summary: sum,
		Nav:     nav,
		Toast:   toast,
	})
}

var statusPaths = map[string]string{
	"active": "/active", "backlog": "/backlog",
	"done": "/done", "dropped": "/dropped",
}

var pathStatus = map[string]string{
	"/active": "active", "/backlog": "backlog",
	"/done": "done", "/dropped": "dropped",
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

// unitNames is the genitive plural used after a fixed total ("усього 86
// серій"). Naming an arbitrary number needs all three forms, which is what
// unitForms holds.
func unitWords(e store.Entry) (one, few, many string) {
	// A podcast counts the same episodes by a different name.
	if e.Media.Kind == "podcast" {
		return "випуск", "випуски", "випусків"
	}
	f, ok := unitForms[e.Media.Unit]
	if !ok {
		return "", "", ""
	}
	return f[0], f[1], f[2]
}

func toastFor(move store.Move) toastView {
	if !move.Changed {
		return toastView{}
	}
	one, few, many := unitWords(move.Entry)
	switch {
	case move.To < move.From:
		return toastView{
			Text:    fmt.Sprintf("%s · назад на %d", move.Entry.Media.Title, move.To),
			EntryID: move.Entry.ID,
			Undo:    true,
		}
	case move.Finished:
		return toastView{Text: move.Entry.Media.Title + " · завершено", EntryID: move.Entry.ID, Undo: true}
	case one != "":
		return toastView{
			Text:    move.Entry.Media.Title + " · " + domain.Count(move.To, one, few, many),
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
		Render:      true,
		Off:         s.Active == 0,
		Waiting:     s.Waiting,
		WaitingTime: domain.HoursMins(s.WaitingMins),
		Airing:      s.Airing,
		Active:      s.Active,
	}
}

// ongoing marks what never finishes. A podcast is the only such thing today,
// but the question the main list asks — how far have you got — has no answer
// for any of them, so they get their own place rather than a blank position.
func ongoing(e store.Entry) bool {
	return e.Media.Kind == "podcast"
}

func buildRow(e store.Entry, today string) row {
	rw := row{
		EntryID:   e.ID,
		Title:     e.Media.Title,
		Rating:    int(e.Rating.Int64),
		Kind:      e.Media.Kind,
		KindLabel: kindLabels[e.Media.Kind],
		Step:      max(e.Step, 1),
		LinkURL:   e.LinkURL,
		Btn:       "+",
	}
	if rw.Step > 1 {
		rw.Btn = fmt.Sprintf("+%d", rw.Step)
		rw.BtnWide = true
	}

	total := int(e.Media.TotalUnits.Int64)
	hasTotal := e.Media.TotalUnits.Valid
	rw.Total = total

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
		if ongoing(e) {
			one, few, many := unitWords(e)
			rw.Pos = domain.Count(e.Position, one, few, many)
			rw.PosSub = ""
		}
		rw.Sub = e.Media.TitleOrig
	case len(e.Units) > 0:
		rw.Mode = "cells"
		rw.Blocks = domain.BuildTiered(e.Units, e.Position, today)
		rw.SeasonEnd = domain.CurrentSeasonEnd(rw.Blocks)
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
		return domain.Count(rem.Waiting, "чекає", "чекають", "чекають")
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
