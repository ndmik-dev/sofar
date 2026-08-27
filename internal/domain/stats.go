package domain

import "time"

const dayFormat = "2006-01-02"

// ProgressRow is one non-undone, forward, manual progress record. Backfill is
// deliberately excluded upstream: dumping a 228-episode archive entry must not
// look like a 228-episode day.
type ProgressRow struct {
	At      int64
	Delta   int
	Kind    string
	Unit    string
	Runtime int
}

func (r ProgressRow) mins() int {
	switch r.Unit {
	case "episode", "none":
		return r.Delta * r.Runtime
	case "hour":
		return r.Delta * 60
	}
	return 0
}

type DayCell struct {
	Kind string
	Mins int
}

type YearStats struct {
	Episodes int
	Films    int
	Pages    int
	Mins     int
	Days     map[string]DayCell
	Months   [12]map[string]int
	MaxMonth int
}

// BuildYear aggregates a year of progress. day maps a timestamp to its
// calendar day honouring the 04:00 boundary.
func BuildYear(rows []ProgressRow, day func(int64) time.Time) YearStats {
	st := YearStats{Days: map[string]DayCell{}}
	for i := range st.Months {
		st.Months[i] = map[string]int{}
	}
	perDayKind := map[string]map[string]int{}

	for _, r := range rows {
		d := day(r.At)
		key := d.Format(dayFormat)
		mins := r.mins()

		st.Mins += mins
		switch {
		case r.Unit == "episode":
			st.Episodes += r.Delta
		case r.Kind == "movie":
			st.Films += r.Delta
		case r.Unit == "page":
			st.Pages += r.Delta
		}

		// Pages and chapters carry no minutes but still colour the day.
		weight := mins
		if weight == 0 {
			weight = r.Delta
		}

		if perDayKind[key] == nil {
			perDayKind[key] = map[string]int{}
		}
		perDayKind[key][r.Kind] += weight

		cell := st.Days[key]
		cell.Mins += weight
		st.Days[key] = cell

		st.Months[int(d.Month())-1][r.Kind] += weight
	}

	// The day takes the kind that dominated it.
	for key, kinds := range perDayKind {
		bestKind, best := "", -1
		for kind, w := range kinds {
			if w > best {
				bestKind, best = kind, w
			}
		}
		cell := st.Days[key]
		cell.Kind = bestKind
		st.Days[key] = cell
	}

	for _, kinds := range st.Months {
		var sum int
		for _, v := range kinds {
			sum += v
		}
		if sum > st.MaxMonth {
			st.MaxMonth = sum
		}
	}
	return st
}

type StreakInfo struct {
	Current int
	Best    int
	Frozen  []string
}

// Streak walks days backwards from today. A missed day is bridged when that
// calendar month's single freeze is still unspent; two missed days in a row
// always break the run. Today itself is never a miss — the day is not over.
func Streak(days map[string]bool, today time.Time) StreakInfo {
	info := StreakInfo{}
	if len(days) == 0 {
		return info
	}

	t := today
	if !days[t.Format(dayFormat)] {
		t = t.AddDate(0, 0, -1)
	}
	usedFreeze := map[string]bool{}
	for {
		key := t.Format(dayFormat)
		if days[key] {
			info.Current++
			t = t.AddDate(0, 0, -1)
			continue
		}
		month := key[:7]
		prev := t.AddDate(0, 0, -1)
		if !usedFreeze[month] && days[prev.Format(dayFormat)] {
			usedFreeze[month] = true
			info.Frozen = append(info.Frozen, key)
			// A frozen day still counts: a streak is an unbroken stretch of
			// calendar, not a tally of active days.
			info.Current++
			t = prev
			continue
		}
		break
	}

	info.Best = bestStreak(days)
	if info.Current > info.Best {
		info.Best = info.Current
	}
	return info
}

func bestStreak(days map[string]bool) int {
	var keys []time.Time
	for k, ok := range days {
		if !ok {
			continue
		}
		if t, err := time.Parse(dayFormat, k); err == nil {
			keys = append(keys, t)
		}
	}
	if len(keys) == 0 {
		return 0
	}
	sortTimes(keys)

	best, run := 1, 1
	usedFreeze := map[string]bool{}
	for i := 1; i < len(keys); i++ {
		gap := int(keys[i].Sub(keys[i-1]).Hours() / 24)
		switch {
		case gap == 1:
			run++
		case gap == 2:
			// One missed day between two active ones — bridgeable once per
			// month of the missed day.
			missed := keys[i-1].AddDate(0, 0, 1).Format(dayFormat)[:7]
			if !usedFreeze[missed] {
				usedFreeze[missed] = true
				run += 2
			} else {
				run = 1
			}
		default:
			run = 1
			usedFreeze = map[string]bool{}
		}
		if run > best {
			best = run
		}
	}
	return best
}

func sortTimes(ts []time.Time) {
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && ts[j].Before(ts[j-1]); j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}
