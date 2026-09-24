package server

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ndmik-dev/sofar/internal/tmdb"
)

// One submission is one blocking round of TMDB searches, so the cap is what a
// person will wait for rather than what the API would tolerate.
const importLimit = 60

type importLine struct {
	Raw      string
	Query    string
	Year     int
	Position int
}

type importResult struct {
	Raw     string
	Title   string
	Kind    string
	Meta    string
	Status  string
	Matched bool
	Already bool
}

type importPage struct {
	Title   string
	NavView navView
	Now     time.Time

	Results  []importResult
	Matched  int
	Missed   int
	Overflow int
	Status   string
	Disabled bool
}

var yearSuffix = regexp.MustCompile(`\s*\((\d{4})\)\s*$`)

// parseImport takes one title per line and tolerates what people actually
// paste: a CSV first column, a trailing year, and "title | 12" for a position.
func parseImport(raw string) ([]importLine, int) {
	var out []importLine
	var overflow int
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), `"`))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		l := importLine{Raw: line}
		if title, pos, found := strings.Cut(line, "|"); found {
			l.Raw = strings.TrimSpace(title)
			line = l.Raw
			l.Position, _ = strconv.Atoi(strings.TrimSpace(pos))
		} else if title, _, found := strings.Cut(line, ","); found {
			line = strings.TrimSpace(strings.Trim(strings.TrimSpace(title), `"`))
			l.Raw = line
		}
		if m := yearSuffix.FindStringSubmatch(line); m != nil {
			l.Year, _ = strconv.Atoi(m[1])
			line = yearSuffix.ReplaceAllString(line, "")
		}
		l.Query = strings.TrimSpace(line)
		if l.Query == "" {
			continue
		}
		if len(out) >= importLimit {
			overflow++
			continue
		}
		out = append(out, l)
	}
	return out, overflow
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	s.renderImport(w, r, importPage{Status: "done"})
}

func (s *Server) renderImport(w http.ResponseWriter, r *http.Request, page importPage) {
	c := s.now()
	statusCounts, err := s.store.CountsByStatus(r.Context(), defaultUserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	kindCounts, err := s.store.CountsByKind(r.Context(), defaultUserID, "active")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	page.Title = "Імпорт"
	page.Now = c.Now
	page.NavView = navViewFrom(statusCounts, kindCounts, "/import", false)
	page.Disabled = !s.catalog.Enabled()
	s.render(w, r, "import.html", page)
}

func (s *Server) handleImportRun(w http.ResponseWriter, r *http.Request) {
	status, ok := validStatus(r.FormValue("status"))
	if !ok {
		http.Error(w, "unknown status", http.StatusBadRequest)
		return
	}
	lines, overflow := parseImport(r.FormValue("list"))
	page := importPage{Status: status, Overflow: overflow}

	now := time.Now().In(s.cfg.Loc)
	for _, l := range lines {
		res := s.importOne(r.Context(), l, status, now)
		page.Results = append(page.Results, res)
		if res.Matched {
			page.Matched++
		} else {
			page.Missed++
		}
	}
	s.renderFragment(w, r, "import-results", page)
}

// A list is not a search box: the first hit is often a documentary about the
// thing you meant. An exact title wins over popularity, and a year given in
// brackets wins over both.
func pickMatch(found []tmdb.Result, l importLine) tmdb.Result {
	want := strings.ToLower(l.Query)
	exact := func(f tmdb.Result) bool {
		return strings.ToLower(f.Title) == want || strings.ToLower(f.Original) == want
	}

	if l.Year > 0 {
		for _, f := range found {
			if f.Year == l.Year && exact(f) {
				return f
			}
		}
	}
	for _, f := range found {
		if exact(f) {
			return f
		}
	}
	if l.Year > 0 {
		for _, f := range found {
			if f.Year == l.Year {
				return f
			}
		}
	}
	return found[0]
}

func (s *Server) importOne(ctx context.Context, l importLine, status string, now time.Time) importResult {
	out := importResult{Raw: l.Raw, Status: statusWords[status]}

	found, err := s.catalog.Search(ctx, l.Query, 6)
	if err != nil {
		s.log.Error("import search", "q", l.Query, "err", err)
		return out
	}
	if len(found) == 0 {
		return out
	}

	hit := pickMatch(found, l)

	mediaID, err := s.catalog.Import(ctx, hit.TMDBType, hit.TMDBID, now)
	if err != nil {
		s.log.Error("import fetch", "q", l.Query, "err", err)
		return out
	}
	_, created, err := s.store.AddEntry(ctx, defaultUserID, mediaID, status, l.Position, now)
	if err != nil {
		s.log.Error("import add", "q", l.Query, "err", err)
		return out
	}

	out.Matched = true
	out.Already = !created
	out.Title = hit.Title
	out.Kind = hit.Kind
	out.Meta = yearLabel(hit.Year)
	return out
}
