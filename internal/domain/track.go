package domain

import "fmt"

const (
	StateWatched = "f"
	StateNext    = "n"
	StateAired   = "a"
	StateUnaired = "u"
)

type Unit struct {
	Idx     int
	Season  int
	Number  int
	Title   string
	AirDate string
	Runtime int
}

type Cell struct {
	Sep   bool
	Idx   int
	State string
	Hint  string
}

// A cell is "aired" only when we know it aired. An empty date means the
// episode is unannounced or far off, which reads as not out yet.
func (u Unit) aired(today string) bool {
	return u.AirDate != "" && u.AirDate <= today
}

// Label is the episode's name in the interface: S2E5 when a show has more than
// one season, E5 when it does not.
func Label(u Unit, multiSeason bool) string { return u.label(multiSeason) }

func (u Unit) label(multiSeason bool) string {
	if multiSeason && u.Season > 0 {
		return fmt.Sprintf("S%dE%d", u.Season, u.Number)
	}
	return fmt.Sprintf("E%d", u.Number)
}

func (u Unit) hint(multiSeason bool) string {
	s := u.label(multiSeason)
	if u.Title != "" {
		s += " · " + u.Title
	}
	if u.Runtime > 0 {
		s += fmt.Sprintf(" · %d хв", u.Runtime)
	}
	return s
}

func MultiSeason(units []Unit) bool {
	for _, u := range units {
		if u.Season != units[0].Season {
			return true
		}
	}
	return false
}

func BuildTrack(units []Unit, position int, today string) []Cell {
	multi := MultiSeason(units)
	cells := make([]Cell, 0, len(units)+4)
	prevSeason := 0
	for i, u := range units {
		if i > 0 && u.Season != prevSeason {
			cells = append(cells, Cell{Sep: true})
		}
		prevSeason = u.Season

		var state string
		switch {
		case u.Idx <= position:
			state = StateWatched
		case u.Idx == position+1:
			state = StateNext
		case u.aired(today):
			state = StateAired
		default:
			state = StateUnaired
		}
		cells = append(cells, Cell{Idx: u.Idx, State: state, Hint: u.hint(multi)})
	}
	return cells
}

func NextUnit(units []Unit, position int) (Unit, bool) {
	for _, u := range units {
		if u.Idx == position+1 {
			return u, true
		}
	}
	return Unit{}, false
}

func NextLabel(units []Unit, position int) string {
	u, ok := NextUnit(units, position)
	if !ok {
		return ""
	}
	s := u.label(MultiSeason(units))
	if u.Title != "" {
		s += " «" + u.Title + "»"
	}
	return s
}

type Remaining struct {
	Waiting     int
	WaitingMins int
	Left        int
	LeftMins    int
}

func RemainingFrom(units []Unit, position int, today string) Remaining {
	var r Remaining
	for _, u := range units {
		if u.Idx <= position {
			continue
		}
		r.Left++
		r.LeftMins += u.Runtime
		if u.aired(today) {
			r.Waiting++
			r.WaitingMins += u.Runtime
		}
	}
	return r
}

func HoursMins(mins int) string {
	if mins <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%02d", mins/60, mins%60)
}

// FreshDays is how long "new" lasts. Two weeks covers a missed week without
// turning the section into a second copy of the whole list.
const FreshDays = 14

// JustAired reports the first unwatched episode that landed between since and
// today. It answers "what is there to watch tonight" from air dates alone —
// the shelf already knows, nothing new has to be stored. Dates are ISO, so
// string order is date order and no parsing is needed.
func JustAired(units []Unit, position int, since, today string) (Unit, bool) {
	for _, u := range units {
		if u.Idx <= position || u.AirDate == "" {
			continue
		}
		if u.AirDate > since && u.AirDate <= today {
			return u, true
		}
	}
	return Unit{}, false
}
