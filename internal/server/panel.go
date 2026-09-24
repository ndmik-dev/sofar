package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ndmik-dev/sofar/internal/domain"
	"github.com/ndmik-dev/sofar/internal/store"
)

type episodeRow struct {
	Label   string
	Title   string
	Date    string
	State   string
	Idx     int
	Current bool
}

type factRow struct {
	Label string
	Value string
}

type seasonTab struct {
	Number int
	On     bool
	Href   string
}

type panelView struct {
	EntryID   int64
	Percent   int
	HasBar    bool
	Title     string
	Original  string
	Kind      string
	KindLabel string
	Meta      string
	Overview  string
	Note      string
	Rating    int
	Dots      []bool

	Blocks    []domain.Block
	Dense     bool
	SeasonEnd int
	Position  string

	NextLabel string
	NextMeta  string
	Step      int

	SeasonTitle string
	SeasonTabs  []seasonTab
	Episodes    []episodeRow
	Facts       []factRow

	Links         []store.Link
	Statuses      []statusOption
	Watch         watchView
	CanWatch      bool
	Pos           int
	SubtitleLabel string
	Total         int
	TotalEditable bool
	UnitLabel     string
}

func (s *Server) handlePanel(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	c := s.now()

	entry, err := s.store.GetEntry(r.Context(), id)
	if err != nil {
		s.failEntry(w, r, err)
		return
	}
	pace, err := s.store.Pace(r.Context(), id, c.Now)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	wantSeason, _ := strconv.Atoi(r.URL.Query().Get("season"))

	links, err := s.store.LinksFor(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	watch, watching, err := s.store.WatchFor(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	view := buildPanel(entry, pace, c.Today, s.cfg.Loc, wantSeason)
	view.Links = links
	// Only a printed series has volumes to wait for.
	view.CanWatch = entry.Media.Kind == "manga"
	view.Watch = buildWatch(watch, watching, s.cfg.Loc)
	if view.Watch.Query == "" {
		view.Watch.Query = entry.Media.Title
	}
	s.renderFragment(w, r, "panel", view)
}

func buildPanel(e store.Entry, pace store.Pace, today string, loc *time.Location, wantSeason int) panelView {
	p := panelView{
		EntryID:   e.ID,
		Title:     e.Media.Title,
		Original:  e.Media.TitleOrig,
		Kind:      e.Media.Kind,
		KindLabel: kindLabels[e.Media.Kind],
		Overview:  e.Media.Overview,
		Note:      e.Note,
		Rating:    int(e.Rating.Int64),
		Step:      max(e.Step, 1),
		Dots:      make([]bool, 10),
	}
	for i := range p.Dots {
		p.Dots[i] = i < p.Rating
	}
	p.Meta = panelMeta(e)
	p.Statuses = panelStatuses(e.Status)
	p.SubtitleLabel, _ = subtitleField(e.Media.Kind)
	p.UnitLabel = domain.UnitMany(e.Media.Unit)
	p.Total = int(e.Media.TotalUnits.Int64)
	p.Pos = e.Position
	// A total can only be typed while nothing is drawn from real episodes.
	p.TotalEditable = len(e.Units) == 0 && e.Media.Unit != "none"

	total := int(e.Media.TotalUnits.Int64)
	if e.Media.TotalUnits.Valid {
		p.Position = fmt.Sprintf("%d / %d", e.Position, total)
	} else {
		p.Position = fmt.Sprintf("%d", e.Position)
	}

	switch {
	case e.Depth == "status":
		// Nothing to draw: the switch above is the whole of its progress.
	case volumeTrack(e):
		p.Blocks = domain.BuildVolumes(total, e.Position)
		p.SeasonEnd = total
	case len(e.Units) == 0 && e.Media.TotalUnits.Valid && total > 1:
		// Books have no named units, but their progress deserves more than a
		// number: the same bar the list shows.
		p.HasBar = true
		p.Percent = e.Position * 100 / total
	}

	if len(e.Units) > 0 {
		p.Blocks = domain.BuildTiered(e.Units, e.Position, today)
		p.SeasonEnd = domain.CurrentSeasonEnd(p.Blocks)
		p.Episodes, p.SeasonTitle = currentSeasonEpisodes(e, today, wantSeason)
		p.SeasonTabs = seasonTabs(e, p.SeasonTitle)

		if next, ok := domain.NextUnit(e.Units, e.Position); ok {
			p.NextLabel = domain.NextLabel(e.Units, e.Position)
			p.NextMeta = nextMeta(next, e, today)
		}
	}

	p.Facts = panelFacts(e, pace, loc)
	return p
}

func panelMeta(e store.Entry) string {
	var parts []string
	if e.Media.Year > 0 {
		parts = append(parts, fmt.Sprintf("%d", e.Media.Year))
	}
	if n := countSeasons(e.Units); n > 0 {
		parts = append(parts, domain.Count(n, "сезон", "сезони", "сезонів"))
	}
	if e.Media.TotalUnits.Valid && e.Media.Unit != "none" {
		total := int(e.Media.TotalUnits.Int64)
		parts = append(parts, domain.UnitCount(total, e.Media.Unit))
	}
	if e.Media.RuntimeMin > 0 && e.Media.Unit == "episode" {
		parts = append(parts, fmt.Sprintf("~%d хв", e.Media.RuntimeMin))
	}
	if e.Media.Airing == "returning" {
		parts = append(parts, "ще виходить")
	}
	return strings.Join(parts, " · ")
}

func countSeasons(units []domain.Unit) int {
	if len(units) == 0 {
		return 0
	}
	n, prev := 1, units[0].Season
	for _, u := range units {
		if u.Season != prev {
			n++
			prev = u.Season
		}
	}
	return n
}

func nextMeta(next domain.Unit, e store.Entry, today string) string {
	var parts []string
	if next.AirDate != "" {
		if next.AirDate > today {
			parts = append(parts, "вийде "+humanDate(next.AirDate))
		} else {
			parts = append(parts, "вийшла "+humanDate(next.AirDate))
		}
	}
	if next.Runtime > 0 {
		parts = append(parts, fmt.Sprintf("%d хв", next.Runtime))
	}
	if rem := domain.RemainingFrom(e.Units, e.Position, today); rem.Left > 0 {
		left := domain.Count(rem.Left, "серія", "серії", "серій")
		if t := domain.HoursMins(rem.LeftMins); t != "" {
			left += " ≈ " + t
		}
		parts = append(parts, "лишилось "+left)
	}
	return strings.Join(parts, " · ")
}

// Every entry can be moved between lists, not just the ones tracked by status.
// The row has no room for a fourth control, and this is where you already come
// to say what a title is to you.
func panelStatuses(current string) []statusOption {
	return []statusOption{
		{"active", "у процесі", current == "active"},
		{"backlog", "колись", current == "backlog"},
		{"done", "завершено", current == "done"},
		{"dropped", "кинуто", current == "dropped"},
	}
}

func seasonTabs(e store.Entry, showing string) []seasonTab {
	seasons := domain.Seasons(e.Units)
	if len(seasons) < 2 {
		return nil
	}
	out := make([]seasonTab, 0, len(seasons))
	for _, sn := range seasons {
		out = append(out, seasonTab{
			Number: sn.Number,
			On:     showing == fmt.Sprintf("Сезон %d", sn.Number),
			Href:   fmt.Sprintf("/entry/%d/panel?season=%d", e.ID, sn.Number),
		})
	}
	return out
}

// A season you have already passed collapses in the track, so the panel is the
// only place left to reach an episode inside it. want == 0 means "the one I am
// in", which is what every request but a tab click asks for.
func currentSeasonEpisodes(e store.Entry, today string, want int) ([]episodeRow, string) {
	season := 0
	if next, ok := domain.NextUnit(e.Units, e.Position); ok {
		season = next.Season
	} else {
		season = e.Units[len(e.Units)-1].Season
	}
	if want > 0 {
		season = want
	}

	multi := domain.MultiSeason(e.Units)
	var out []episodeRow
	for _, u := range e.Units {
		if u.Season != season {
			continue
		}
		state := domain.StateUnaired
		switch {
		case u.Idx <= e.Position:
			state = domain.StateWatched
		case u.Idx == e.Position+1:
			state = domain.StateNext
		case u.AirDate != "" && u.AirDate <= today:
			state = domain.StateAired
		}
		out = append(out, episodeRow{
			Label:   fmt.Sprintf("E%d", u.Number),
			Title:   u.Title,
			Date:    humanDate(u.AirDate),
			State:   state,
			Idx:     u.Idx,
			Current: u.Idx == e.Position+1,
		})
	}

	title := "Серії"
	if multi && season > 0 {
		title = fmt.Sprintf("Сезон %d", season)
	}
	return out, title
}

func panelFacts(e store.Entry, pace store.Pace, loc *time.Location) []factRow {
	var facts []factRow

	if pace.FirstAt > 0 {
		facts = append(facts, factRow{"почав", time.Unix(pace.FirstAt, 0).In(loc).Format("02.01.2006")})
	}
	if pace.Units > 0 && pace.Days > 0 {
		facts = append(facts, factRow{
			domain.Count(pace.Units, "одиниця за", "одиниці за", "одиниць за"),
			domain.Count(pace.Days, "день", "дні", "днів"),
		})
	}
	if pace.HasEnough && pace.PerWeek > 0 {
		facts = append(facts, factRow{"темп", fmt.Sprintf("%.1f / тиждень", pace.PerWeek)})
	}
	if e.Media.Unit == "episode" && e.Media.RuntimeMin > 0 {
		facts = append(facts, factRow{"переглянуто", domain.HoursMins(e.Position * e.Media.RuntimeMin)})
	}
	return facts
}

// humanDate turns an ISO date into the short form used everywhere else.
func humanDate(iso string) string {
	if len(iso) != 10 {
		return ""
	}
	return iso[8:10] + "." + iso[5:7] + "." + iso[0:4]
}
