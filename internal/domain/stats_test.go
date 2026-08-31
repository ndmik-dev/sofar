package domain

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, _ := time.Parse(dayFormat, s)
	return t
}

func daySet(dates ...string) map[string]bool {
	m := map[string]bool{}
	for _, d := range dates {
		m[d] = true
	}
	return m
}

func TestStreakSimpleRun(t *testing.T) {
	days := daySet("2026-08-25", "2026-08-26", "2026-08-27")
	got := Streak(days, day("2026-08-27"))
	if got.Current != 3 || got.Best != 3 {
		t.Errorf("current=%d best=%d, want 3/3", got.Current, got.Best)
	}
}

func TestStreakTodayNotOverYet(t *testing.T) {
	days := daySet("2026-08-25", "2026-08-26")
	got := Streak(days, day("2026-08-27"))
	if got.Current != 2 {
		t.Errorf("a day with no activity yet must not break the run: current=%d, want 2", got.Current)
	}
}

func TestStreakFreezeBridgesOneMissedDay(t *testing.T) {
	days := daySet("2026-08-23", "2026-08-24", "2026-08-26", "2026-08-27")
	got := Streak(days, day("2026-08-27"))
	// 23..27 is five calendar days; the frozen 25th counts like any other.
	if got.Current != 5 {
		t.Errorf("one missed day inside the month must be bridged: current=%d, want 5", got.Current)
	}
	if len(got.Frozen) != 1 || got.Frozen[0] != "2026-08-25" {
		t.Errorf("frozen=%v, want [2026-08-25]", got.Frozen)
	}
}

func TestStreakSecondMissInSameMonthBreaks(t *testing.T) {
	days := daySet("2026-08-20", "2026-08-22", "2026-08-24", "2026-08-25", "2026-08-26", "2026-08-27")
	got := Streak(days, day("2026-08-27"))
	// 27..24 = 4, freeze bridges the 23rd, +22 = 6; the 21st needs a second
	// August freeze, which does not exist.
	if got.Current != 6 {
		t.Errorf("current=%d, want 6", got.Current)
	}
}

func TestStreakTwoMissedDaysInARowBreak(t *testing.T) {
	days := daySet("2026-08-23", "2026-08-26", "2026-08-27")
	got := Streak(days, day("2026-08-27"))
	if got.Current != 2 {
		t.Errorf("two consecutive missed days must break: current=%d, want 2", got.Current)
	}
}

func TestStreakFreezeResetsAcrossMonths(t *testing.T) {
	days := daySet("2026-07-30", "2026-08-01", "2026-08-02", "2026-08-04", "2026-08-05")
	got := Streak(days, day("2026-08-05"))
	// 03.08 takes August's freeze, 31.07 takes July's.
	if got.Current != 7 {
		t.Errorf("current=%d, want 7 (one freeze per month)", got.Current)
	}
}

func TestStreakEmpty(t *testing.T) {
	got := Streak(map[string]bool{}, day("2026-08-27"))
	if got.Current != 0 || got.Best != 0 {
		t.Errorf("empty history: %+v", got)
	}
}

func TestBuildYearCounts(t *testing.T) {
	at := func(d string) int64 { return day(d).Unix() + 12*3600 }
	rows := []ProgressRow{
		{At: at("2026-03-10"), Delta: 2, Kind: "show", Unit: "episode", Runtime: 50},
		{At: at("2026-03-10"), Delta: 30, Kind: "book", Unit: "page"},
		{At: at("2026-03-11"), Delta: 1, Kind: "movie", Unit: "none", Runtime: 120},
		{At: at("2026-04-01"), Delta: 2, Kind: "book", Unit: "hour"},
	}
	st := BuildYear(rows, func(ts int64) time.Time { return time.Unix(ts, 0).UTC() })

	if st.Episodes != 2 || st.Films != 1 || st.Pages != 30 {
		t.Errorf("episodes=%d films=%d pages=%d", st.Episodes, st.Films, st.Pages)
	}
	if want := 2*50 + 120 + 2*60; st.Mins != want {
		t.Errorf("mins=%d, want %d", st.Mins, want)
	}
	if st.Days["2026-03-10"].Kind != "show" {
		t.Errorf("10.03 dominated by %q, want show (100 min vs 30 pages)", st.Days["2026-03-10"].Kind)
	}
	if st.Months[2]["show"] != 100 || st.Months[3]["book"] != 120 {
		t.Errorf("months: %v %v", st.Months[2], st.Months[3])
	}
}

func TestYearCountsEachKindInItsOwnUnit(t *testing.T) {
	day := time.Date(2026, 5, 10, 20, 0, 0, 0, time.UTC).Unix()
	rows := []ProgressRow{
		// Three episodes of a 45-minute show.
		{At: day, Delta: 3, To: 3, Total: 10, Kind: "show", Unit: "episode", Runtime: 45},
		// A film watched to the end: one film, 120 minutes — not 120 films.
		{At: day, Delta: 120, To: 120, Total: 120, Kind: "movie", Unit: "minute", Runtime: 120},
		// A film left half-watched counts its minutes but is not a film yet.
		{At: day, Delta: 50, To: 50, Total: 140, Kind: "movie", Unit: "minute", Runtime: 140},
		// Manga is read in chapters and often has no end at all.
		{At: day, Delta: 6, To: 6, Kind: "manga", Unit: "chapter"},
		{At: day, Delta: 30, To: 30, Total: 300, Kind: "book", Unit: "page"},
	}

	st := BuildYear(rows, func(ts int64) time.Time { return time.Unix(ts, 0).UTC() })
	if st.Episodes != 3 {
		t.Errorf("Episodes = %d, want 3", st.Episodes)
	}
	if st.Films != 1 {
		t.Errorf("Films = %d, want 1 — a film is one film however long it is", st.Films)
	}
	if st.Pages != 30 {
		t.Errorf("Pages = %d, want 30", st.Pages)
	}
	// 3*45 + 120 + 50 = 305
	if st.Mins != 305 {
		t.Errorf("Mins = %d, want 305 — minutes are the unit for a film", st.Mins)
	}
}
