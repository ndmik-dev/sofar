package config

import (
	"testing"
	"time"
)

func TestDay(t *testing.T) {
	kyiv, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Loc: kyiv, DayStart: 4}

	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{"evening stays put", time.Date(2026, 8, 27, 21, 30, 0, 0, kyiv), "2026-08-27"},
		{"after midnight counts as yesterday", time.Date(2026, 8, 28, 1, 30, 0, 0, kyiv), "2026-08-27"},
		{"just before the boundary", time.Date(2026, 8, 28, 3, 59, 0, 0, kyiv), "2026-08-27"},
		{"on the boundary starts the new day", time.Date(2026, 8, 28, 4, 0, 0, 0, kyiv), "2026-08-28"},
		{"utc input is converted first", time.Date(2026, 8, 28, 0, 30, 0, 0, time.UTC), "2026-08-27"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cfg.Day(tc.at).Format("2006-01-02"); got != tc.want {
				t.Errorf("Day(%s) = %s, want %s", tc.at.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}
