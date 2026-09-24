package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/ndmik-dev/sofar/internal/domain"
	"github.com/ndmik-dev/sofar/internal/store"
)

type timeBucket struct {
	Key   string
	Label string
	Max   int
}

// Buckets answer the only question a backlog gets asked: will this fit in the
// time I have tonight.
var timeBuckets = []timeBucket{
	{"40", "40 хв", 40},
	{"2h", "до 2 год", 120},
	{"evening", "вечір", 300},
	{"weekend", "вихідні", 900},
	{"long", "довге", 1 << 30},
}

type filterChip struct {
	Key   string
	Label string
	Href  string
	On    bool
}

type yearGroup struct {
	Year  int
	Count int
	Rows  []row
}

func (s *Server) handleActive(w http.ResponseWriter, r *http.Request) {
	s.renderList(w, r, listSpec{
		Status: "active",
		Title:  "У процесі",
		Href:   "/active",
		Page:   "active.html",
	})
}

func (s *Server) handleBacklog(w http.ResponseWriter, r *http.Request) {
	s.renderList(w, r, listSpec{
		Status: "backlog",
		Title:  "Колись",
		Href:   "/backlog",
		Page:   "backlog.html",
	})
}

func (s *Server) handleDone(w http.ResponseWriter, r *http.Request) {
	s.renderList(w, r, listSpec{
		Status: "done",
		Title:  "Завершено",
		Href:   "/done",
		Page:   "done.html",
	})
}

func (s *Server) handleDropped(w http.ResponseWriter, r *http.Request) {
	s.renderList(w, r, listSpec{
		Status: "dropped",
		Title:  "Кинуто",
		Href:   "/dropped",
		Page:   "done.html",
	})
}

type listSpec struct {
	Status string
	Title  string
	Href   string
	Page   string
}

func (s *Server) renderList(w http.ResponseWriter, r *http.Request, spec listSpec) {
	ctx := r.Context()
	c := s.now()

	entries, err := s.store.ListEntries(ctx, defaultUserID, spec.Status)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	statusCounts, err := s.store.CountsByStatus(ctx, defaultUserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	kindCounts, err := s.store.CountsByKind(ctx, defaultUserID, spec.Status)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	kind := r.URL.Query().Get("kind")
	if kind != "" {
		kept := entries[:0]
		for _, e := range entries {
			if e.Media.Kind == kind {
				kept = append(kept, e)
			}
		}
		entries = kept
	}

	page := listPage{
		Title:   spec.Title,
		NavView: navViewWithKind(statusCounts, kindCounts, spec.Href, kind, false),
		Now:     c.Now,
	}

	if kind != "" {
		page.Filter = &kindFilter{Label: kindLabels[kind], Kind: kind, Count: len(entries), Clear: spec.Href}
	}

	switch spec.Status {
	case "active":
		if err := s.fillActive(ctx, &page, entries, c, kind); err != nil {
			s.fail(w, r, err)
			return
		}
	case "backlog":
		s.fillBacklog(&page, entries, r.URL.Query().Get("t"), kind, c.Today)
	default:
		fillArchive(&page, entries, c.Today)
	}
	// The first row on screen, not the first in the query: under a kind
	// filter those differ, and the keyboard used to start on nothing.
	switch {
	case len(page.Fresh) > 0:
		page.Fresh[0].Selected = true
	case len(page.Rows) > 0:
		page.Rows[0].Selected = true
	case len(page.Stale) > 0:
		page.Stale[0].Selected = true
	case len(page.Years) > 0 && len(page.Years[0].Rows) > 0:
		page.Years[0].Rows[0].Selected = true
	}

	s.render(w, r, spec.Page, page)
}

// fillActive splits the list three ways — new since you last looked, moving,
// stale — and hangs the figures over it.
func (s *Server) fillActive(ctx context.Context, page *listPage, entries []store.Entry, c clock, kind string) error {
	sum, err := s.store.Summary(ctx, defaultUserID, c.Today, c.StaleBefore)
	if err != nil {
		return err
	}
	page.Summary = summaryFrom(sum)
	page.Summary.Next = s.nextUp(ctx, c.Today, c.StaleBefore)

	since := s.cfg.Day(c.Now).AddDate(0, 0, -domain.FreshDays).Format("2006-01-02")
	for _, e := range entries {
		rw := buildRow(e, c.Today)
		if label, ok := freshVolume(e, c.Now); ok {
			rw.Aired = label
			page.Fresh = append(page.Fresh, rw)
			continue
		}
		if u, ok := domain.JustAired(e.Units, e.Position, since, c.Today); ok {
			rw.Aired = "вийшла " + domain.Label(u, domain.MultiSeason(e.Units))
			page.Fresh = append(page.Fresh, rw)
			continue
		}
		if e.UpdatedAt < c.StaleBefore {
			rw.Stale = true
			page.Stale = append(page.Stale, rw)
			continue
		}
		page.Rows = append(page.Rows, rw)
	}
	page.Empty = len(page.Rows) == 0 && len(page.Stale) == 0 && len(page.Fresh) == 0
	// The element stays in the page even with nothing to show, so an
	// out-of-band swap has a target the moment the first row lands. Under a
	// kind filter the figures describe the whole list, not what is on screen,
	// so they stay hidden rather than contradict it.
	page.Summary.Off = page.Empty || kind != ""
	return nil
}

func (s *Server) fillBacklog(page *listPage, entries []store.Entry, filter, kind, today string) {
	if filter == "" {
		filter = "long"
	}
	page.Bare = len(entries) == 0
	limit := 1 << 30
	for _, b := range timeBuckets {
		href := "/backlog?t=" + b.Key
		// Both filters compose; switching one must not silently drop the other.
		if kind != "" {
			href += "&kind=" + kind
		}
		page.Filters = append(page.Filters, filterChip{Key: b.Key, Label: b.Label, Href: href, On: b.Key == filter})
		if b.Key == filter {
			limit = b.Max
		}
	}

	type sized struct {
		row  row
		mins int
	}
	var all []sized
	var totalMins int

	for _, e := range entries {
		mins := totalMinutes(e)
		totalMins += mins
		rw := buildRow(e, today)
		rw.Pos, rw.PosSub = durationLabels(e, mins)
		// Nothing has started here, so a progress track would be a grey slab.
		// The three-state control is the only useful thing to click.
		rw.Mode = "status"
		rw.Statuses = backlogStatuses(e.Status)
		rw.Btn = ""
		all = append(all, sized{rw, mins})
	}

	sort.SliceStable(all, func(i, j int) bool { return all[i].mins < all[j].mins })

	for _, it := range all {
		if it.mins > 0 && it.mins > limit {
			continue
		}
		page.Rows = append(page.Rows, it.row)
	}

	page.Summary = summaryView{
		Waiting:     len(page.Rows),
		WaitingTime: domain.HoursMins(totalMins),
		Active:      len(entries),
	}
}

func fillArchive(page *listPage, entries []store.Entry, today string) {
	byYear := map[int][]row{}
	var years []int
	for _, e := range entries {
		rw := buildRow(e, today)
		rw.Btn = ""
		y := 0
		if e.FinishedAt > 0 {
			y = time.Unix(e.FinishedAt, 0).Year()
		}
		if _, seen := byYear[y]; !seen {
			years = append(years, y)
		}
		byYear[y] = append(byYear[y], rw)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))
	for _, y := range years {
		page.Years = append(page.Years, yearGroup{Year: y, Count: len(byYear[y]), Rows: byYear[y]})
	}
	page.Summary = summaryView{Active: len(entries)}
}

// Only screen time is knowable: episodes and films carry runtimes, a book's
// page count says nothing about how long it takes to read.
func totalMinutes(e store.Entry) int {
	if len(e.Units) > 0 {
		var sum int
		for _, u := range e.Units {
			sum += u.Runtime
		}
		if sum > 0 {
			return sum
		}
	}
	if e.Media.RuntimeMin > 0 && e.Media.TotalUnits.Valid {
		return e.Media.RuntimeMin * int(e.Media.TotalUnits.Int64)
	}
	return 0
}

func durationLabels(e store.Entry, mins int) (main, sub string) {
	if mins > 0 {
		main = domain.HoursMins(mins)
		if len(e.Units) > 1 {
			sub = domain.Count(len(e.Units), "серія", "серії", "серій")
			if evenings := (mins + 119) / 120; evenings > 1 {
				sub += fmt.Sprintf(" · ≈ %d веч.", evenings)
			}
		}
		return main, sub
	}
	if e.Media.TotalUnits.Valid && e.Media.Unit != "none" {
		total := int(e.Media.TotalUnits.Int64)
		return strconv.Itoa(total), domain.UnitMany(e.Media.Unit)
	}
	return "—", ""
}

func backlogStatuses(current string) []statusOption {
	return []statusOption{
		{"backlog", "хочу", current == "backlog"},
		{"active", "у процесі", current == "active"},
		{"done", "завершив", current == "done"},
	}
}
