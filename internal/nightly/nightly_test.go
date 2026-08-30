package nightly

import (
	"testing"
	"time"

	"github.com/ndmik-dev/sofar/internal/config"
)

func job(t *testing.T, hour int) *Job {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		t.Fatal(err)
	}
	return &Job{cfg: config.Config{Loc: loc, DayStart: hour}}
}

func TestNextRunIsAlwaysAhead(t *testing.T) {
	j := job(t, 4)
	loc := j.cfg.Loc

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			"before the hour, same day",
			time.Date(2026, 3, 10, 1, 0, 0, 0, loc),
			time.Date(2026, 3, 10, 4, 20, 0, 0, loc),
		},
		{
			"after the hour, next day",
			time.Date(2026, 3, 10, 9, 0, 0, 0, loc),
			time.Date(2026, 3, 11, 4, 20, 0, 0, loc),
		},
		{
			// Firing again the same second would spin the loop.
			"exactly on the hour, next day",
			time.Date(2026, 3, 10, 4, 20, 0, 0, loc),
			time.Date(2026, 3, 11, 4, 20, 0, 0, loc),
		},
		{
			"across a month boundary",
			time.Date(2026, 3, 31, 23, 0, 0, 0, loc),
			time.Date(2026, 4, 1, 4, 20, 0, 0, loc),
		},
	}
	for _, c := range cases {
		got := j.nextRun(c.now)
		if !got.Equal(c.want) {
			t.Errorf("%s: nextRun(%s) = %s, want %s", c.name, c.now, got, c.want)
		}
		if !got.After(c.now) {
			t.Errorf("%s: next run is not in the future", c.name)
		}
	}
}

func TestNextRunFollowsTheConfiguredHour(t *testing.T) {
	j := job(t, 6)
	now := time.Date(2026, 3, 10, 5, 0, 0, 0, j.cfg.Loc)
	if got := j.nextRun(now); got.Hour() != 6 {
		t.Errorf("nextRun hour = %d, want 6", got.Hour())
	}
}
