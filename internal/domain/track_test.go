package domain

import "testing"

func units(specs ...Unit) []Unit {
	for i := range specs {
		specs[i].Idx = i + 1
		if specs[i].Season == 0 {
			specs[i].Season = 1
		}
		if specs[i].Number == 0 {
			specs[i].Number = i + 1
		}
	}
	return specs
}

func states(cells []Cell) string {
	out := ""
	for _, c := range cells {
		if c.Sep {
			out += "|"
			continue
		}
		out += c.State
	}
	return out
}

func TestBuildTrackStates(t *testing.T) {
	const today = "2026-08-27"

	us := units(
		Unit{AirDate: "2026-01-01"},
		Unit{AirDate: "2026-01-08"},
		Unit{AirDate: "2026-01-15"},
		Unit{AirDate: "2026-01-22"},
		Unit{AirDate: "2026-09-03"},
		Unit{AirDate: ""},
	)

	got := states(BuildTrack(us, 2, today))
	if want := "ffnauu"; got != want {
		t.Errorf("states = %q, want %q", got, want)
	}
}

func TestBuildTrackSeasonSeparator(t *testing.T) {
	us := []Unit{
		{Idx: 1, Season: 1, Number: 1, AirDate: "2026-01-01"},
		{Idx: 2, Season: 1, Number: 2, AirDate: "2026-01-08"},
		{Idx: 3, Season: 2, Number: 1, AirDate: "2026-02-01"},
	}
	if got, want := states(BuildTrack(us, 1, "2026-08-27")), "fn|a"; got != want {
		t.Errorf("states = %q, want %q", got, want)
	}
}

func TestBuildTrackFullyWatchedHasNoNext(t *testing.T) {
	us := units(Unit{AirDate: "2026-01-01"}, Unit{AirDate: "2026-01-08"})
	if got, want := states(BuildTrack(us, 2, "2026-08-27")), "ff"; got != want {
		t.Errorf("states = %q, want %q", got, want)
	}
}

func TestUnitAiringToday(t *testing.T) {
	us := units(Unit{AirDate: "2026-08-27"})
	if got, want := states(BuildTrack(us, -1, "2026-08-27")), "a"; got != want {
		t.Errorf("an episode airing today counts as out: got %q, want %q", got, want)
	}
}

func TestRemainingFrom(t *testing.T) {
	us := units(
		Unit{AirDate: "2026-01-01", Runtime: 24},
		Unit{AirDate: "2026-01-08", Runtime: 24},
		Unit{AirDate: "2026-01-15", Runtime: 24},
		Unit{AirDate: "2026-09-03", Runtime: 24},
	)

	rem := RemainingFrom(us, 1, "2026-08-27")
	if rem.Left != 3 || rem.LeftMins != 72 {
		t.Errorf("left = %d (%d min), want 3 (72 min)", rem.Left, rem.LeftMins)
	}
	if rem.Waiting != 2 || rem.WaitingMins != 48 {
		t.Errorf("waiting = %d (%d min), want 2 (48 min)", rem.Waiting, rem.WaitingMins)
	}
}

func TestNextUnit(t *testing.T) {
	us := units(Unit{}, Unit{}, Unit{})
	got, ok := NextUnit(us, 1)
	if !ok || got.Idx != 2 {
		t.Errorf("NextUnit(pos 1) = %d, %v; want idx 2, true", got.Idx, ok)
	}
	if _, ok := NextUnit(us, 3); ok {
		t.Error("NextUnit past the end should report false")
	}
}

func TestHoursMins(t *testing.T) {
	for _, tc := range []struct {
		in   int
		want string
	}{{0, ""}, {-5, ""}, {59, "0:59"}, {60, "1:00"}, {188, "3:08"}} {
		if got := HoursMins(tc.in); got != tc.want {
			t.Errorf("HoursMins(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNextLabelDropsSeasonForSingleSeason(t *testing.T) {
	single := units(Unit{}, Unit{Title: "Пробудження"})
	if got, want := NextLabel(single, 1), "E2 «Пробудження»"; got != want {
		t.Errorf("single-season label = %q, want %q", got, want)
	}

	multi := []Unit{
		{Idx: 1, Season: 1, Number: 1},
		{Idx: 2, Season: 2, Number: 1, Title: "Hello"},
	}
	if got, want := NextLabel(multi, 1), "S2E1 «Hello»"; got != want {
		t.Errorf("multi-season label = %q, want %q", got, want)
	}

	if got := NextLabel(single, 9); got != "" {
		t.Errorf("past the end should be empty, got %q", got)
	}
}

func TestJustAired(t *testing.T) {
	units := []Unit{
		{Idx: 1, AirDate: "2026-08-01"},
		{Idx: 2, AirDate: "2026-08-08"},
		{Idx: 3, AirDate: "2026-08-22"},
		{Idx: 4, AirDate: "2026-08-29"},
		{Idx: 5, AirDate: "2026-09-05"},
		{Idx: 6, AirDate: ""},
	}
	const since, today = "2026-08-16", "2026-08-30"

	cases := []struct {
		name     string
		position int
		wantIdx  int
	}{
		{"caught up to the last aired episode", 4, 0},
		{"two fresh episodes waiting, the earlier one wins", 2, 3},
		{"one fresh episode waiting", 3, 4},
		{"far behind, but the backlog is old, not new", 0, 3},
		{"everything watched", 6, 0},
	}
	for _, c := range cases {
		u, ok := JustAired(units, c.position, since, today)
		if c.wantIdx == 0 {
			if ok {
				t.Errorf("%s: got E%d, want nothing", c.name, u.Idx)
			}
			continue
		}
		if !ok || u.Idx != c.wantIdx {
			t.Errorf("%s: got %v/%d, want E%d", c.name, ok, u.Idx, c.wantIdx)
		}
	}
}

func TestJustAiredIgnoresTheFuture(t *testing.T) {
	units := []Unit{{Idx: 1, AirDate: "2026-12-01"}}
	if _, ok := JustAired(units, 0, "2026-08-16", "2026-08-30"); ok {
		t.Error("an episode that has not aired counted as fresh")
	}
}
