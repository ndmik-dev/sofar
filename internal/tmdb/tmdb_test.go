package tmdb

import (
	"context"
	"os"
	"testing"

	"github.com/ndmik-dev/sofar/internal/config"
)

func liveClient(t *testing.T) *Client {
	t.Helper()
	_ = config.LoadDotEnv("../../.env")
	token := os.Getenv("TMDB_TOKEN")
	if token == "" {
		t.Skip("TMDB_TOKEN not set")
	}
	return New(token, t.TempDir())
}

func TestLiveSearch(t *testing.T) {
	c := liveClient(t)
	res, err := c.Search(context.Background(), "severance", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("no results")
	}
	if res[0].Title == "" || res[0].TMDBID == 0 {
		t.Errorf("first result is incomplete: %+v", res[0])
	}
	for _, r := range res {
		t.Logf("%-6s %-8s %-40s %d", r.TMDBType, r.Kind, r.Title, r.Year)
	}
}

func TestLiveTVNumbering(t *testing.T) {
	c := liveClient(t)
	// Severance: two seasons, specials excluded.
	d, err := c.TV(context.Background(), 95396)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s (%d) kind=%s airing=%s runtime=%d seasons=%d episodes=%d",
		d.Title, d.Year, d.Kind, d.Airing, d.Runtime, len(d.Seasons), len(d.Episodes))

	if len(d.Episodes) == 0 {
		t.Fatal("no episodes")
	}
	for _, s := range d.Seasons {
		if s.Number == 0 {
			t.Error("season 0 must be skipped")
		}
	}
	seen := map[int]bool{}
	for _, e := range d.Episodes {
		if e.Season == 0 {
			t.Errorf("special leaked in: S%dE%d", e.Season, e.Number)
		}
		key := e.Season*1000 + e.Number
		if seen[key] {
			t.Errorf("duplicate S%dE%d", e.Season, e.Number)
		}
		seen[key] = true
	}
	if d.Overview == "" {
		t.Error("overview is empty even after the English fallback")
	}
	t.Logf("first: S%dE%d %q %s", d.Episodes[0].Season, d.Episodes[0].Number,
		d.Episodes[0].Title, d.Episodes[0].AirDate)
}

func TestLiveAnimeDetection(t *testing.T) {
	c := liveClient(t)
	// Frieren: Beyond Journey's End
	d, err := c.TV(context.Background(), 209867)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != "anime" {
		t.Errorf("kind = %q, want anime (%s)", d.Kind, d.Title)
	}
}

func TestLiveMovie(t *testing.T) {
	c := liveClient(t)
	// Dune: Part Two
	d, err := c.Movie(context.Background(), 693134)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != "movie" || d.Runtime == 0 {
		t.Errorf("unexpected movie: %+v", d.Result)
	}
	t.Logf("%s (%d) %d хв", d.Title, d.Year, d.Runtime)
}
