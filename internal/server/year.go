package server

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ndmik-dev/sofar/internal/domain"
)

type heatCell struct {
	Kind  string
	Level int
	Title string
}

type heatWeek struct{ Cells []heatCell }

type monthBar struct {
	Label    string
	Segments []monthSegment
	Total    string
}

type monthSegment struct {
	Kind    string
	Percent float64
}

type yearPage struct {
	Title    string
	NavView  navView
	Now      time.Time
	Year     int
	Years    []int
	Hours    string
	HoursSub string
	Episodes int
	Podcasts int
	Films    int
	Books    int
	Pages    int
	Streak   domain.StreakInfo
	Weeks    []heatWeek
	Months   []monthBar
	Copy     string
}

var monthNames = [12]string{"січ", "лют", "бер", "кві", "тра", "чер", "лип", "сер", "вер", "жов", "лис", "гру"}

func (s *Server) handleYear(w http.ResponseWriter, r *http.Request) {
	now := time.Now().In(s.cfg.Loc)
	year := now.Year()
	if raw := r.PathValue("y"); raw != "" {
		y, err := strconv.Atoi(raw)
		if err != nil || y < 2000 || y > now.Year() {
			http.Redirect(w, r, "/year", http.StatusFound)
			return
		}
		year = y
	}

	ctx := r.Context()
	from := time.Date(year, 1, 1, s.cfg.DayStart, 0, 0, 0, s.cfg.Loc).Unix()
	to := time.Date(year+1, 1, 1, s.cfg.DayStart, 0, 0, 0, s.cfg.Loc).Unix()

	rows, err := s.store.ProgressRows(ctx, defaultUserID, from, to)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	books, err := s.store.FinishedInYear(ctx, defaultUserID, "book", from, to)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// The streak spans all history, not one year.
	allRows, err := s.store.ProgressRows(ctx, defaultUserID, 0, now.Unix()+1)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	statusCounts, err := s.store.CountsByStatus(ctx, defaultUserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	kindCounts, err := s.store.CountsByKind(ctx, defaultUserID, "")
	if err != nil {
		s.fail(w, r, err)
		return
	}

	stats := domain.BuildYear(rows, func(ts int64) time.Time { return s.cfg.Day(time.Unix(ts, 0)) })

	days := map[string]bool{}
	for _, row := range allRows {
		days[s.cfg.Day(time.Unix(row.At, 0)).Format("2006-01-02")] = true
	}
	streak := domain.Streak(days, s.cfg.Day(now))

	page := yearPage{
		Title:    fmt.Sprintf("Рік %d", year),
		NavView:  navViewFrom(statusCounts, kindCounts, "/year", false),
		Now:      now,
		Year:     year,
		Years:    []int{now.Year(), now.Year() - 1, now.Year() - 2},
		Hours:    domain.HoursMins(stats.Mins),
		Episodes: stats.Episodes,
		Podcasts: stats.Podcasts,
		Films:    stats.Films,
		Books:    books,
		Pages:    stats.Pages,
		Streak:   streak,
		Weeks:    buildHeat(stats, year, now, s.cfg.Loc),
		Months:   buildMonths(stats),
	}
	if stats.Mins >= 24*60 {
		page.HoursSub = fmt.Sprintf("≈ %d діб поспіль", stats.Mins/(24*60))
	}
	page.Copy = buildCopyText(page)

	s.render(w, r, "year.html", page)
}

func buildHeat(stats domain.YearStats, year int, now time.Time, loc *time.Location) []heatWeek {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, loc)
	// Back up to Monday so columns are calendar weeks.
	for start.Weekday() != time.Monday {
		start = start.AddDate(0, 0, -1)
	}
	end := time.Date(year, 12, 31, 0, 0, 0, 0, loc)
	if now.Year() == year && now.Before(end) {
		end = now
	}

	var weeks []heatWeek
	for t := start; !t.After(end); {
		var week heatWeek
		for d := 0; d < 7; d++ {
			cell := heatCell{}
			if t.Year() == year && !t.After(end) {
				key := t.Format("2006-01-02")
				if dc, ok := stats.Days[key]; ok {
					cell.Kind = dc.Kind
					cell.Level = level(dc.Mins)
					cell.Title = key + " · " + domain.HoursMins(dc.Mins)
				} else {
					cell.Level = 0
					cell.Title = key
				}
			} else {
				cell.Level = -1
			}
			week.Cells = append(week.Cells, cell)
			t = t.AddDate(0, 0, 1)
		}
		weeks = append(weeks, week)
	}
	return weeks
}

func level(mins int) int {
	switch {
	case mins <= 0:
		return 0
	case mins < 45:
		return 1
	case mins < 120:
		return 2
	default:
		return 3
	}
}

func buildMonths(stats domain.YearStats) []monthBar {
	if stats.MaxMonth == 0 {
		return nil
	}
	var out []monthBar
	for i, kinds := range stats.Months {
		bar := monthBar{Label: monthNames[i]}
		var total int
		for _, kind := range []string{"show", "anime", "movie", "book", "game", "podcast"} {
			w := kinds[kind]
			if w == 0 {
				continue
			}
			total += w
			bar.Segments = append(bar.Segments, monthSegment{
				Kind:    kind,
				Percent: float64(w) * 100 / float64(stats.MaxMonth),
			})
		}
		if total > 0 {
			bar.Total = domain.HoursMins(total)
		}
		out = append(out, bar)
	}
	return out
}

// buildCopyText renders the year as the plain text ⌘C puts on the clipboard.
func podcastLine(n int) string {
	if n == 0 {
		return ""
	}
	return " · " + domain.Count(n, "випуск", "випуски", "випусків")
}

func buildCopyText(p yearPage) string {
	out := fmt.Sprintf("Sofar · %d\n\n", p.Year)
	out += fmt.Sprintf("%s годин", p.Hours)
	if p.HoursSub != "" {
		out += " (" + p.HoursSub + ")"
	}
	out += fmt.Sprintf("\n%s · %s%s\n%s",
		domain.Count(p.Episodes, "серія", "серії", "серій"),
		domain.Count(p.Films, "фільм", "фільми", "фільмів"),
		podcastLine(p.Podcasts),
		domain.Count(p.Books, "книга", "книги", "книг"))
	if p.Pages > 0 {
		out += fmt.Sprintf(" · %s", domain.Count(p.Pages, "сторінка", "сторінки", "сторінок"))
	}
	out += fmt.Sprintf("\nстрік: %d зараз · %d рекорд\n\n", p.Streak.Current, p.Streak.Best)
	for _, m := range p.Months {
		if m.Total != "" {
			out += fmt.Sprintf("%s %s\n", m.Label, m.Total)
		}
	}
	return out
}
