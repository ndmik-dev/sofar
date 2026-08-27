package catalog

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/ndmik-dev/sofar/internal/config"
)

func env(t *testing.T, key string) string {
	t.Helper()
	_ = config.LoadDotEnv("../../.env")
	v := os.Getenv(key)
	if v == "" {
		t.Skip(key + " not set")
	}
	return v
}

func TestLiveBooks(t *testing.T) {
	b := NewBooks(env(t, "GOOGLE_BOOKS_KEY"), t.TempDir())
	res, err := b.Search(context.Background(), "ім'я вітру ротфусс", 5)
	if err != nil {
		if strings.Contains(err.Error(), "503") {
			t.Skip("Google Books is having one of its moments:", err)
		}
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("no results")
	}
	for _, r := range res {
		t.Logf("%-46s %-24s %d  стор:%d", trunc(r.Title, 44), trunc(r.Subtitle, 22), r.Year, r.Total)
	}
	if res[0].Kind != "book" || res[0].ExtID == "" {
		t.Errorf("incomplete result: %+v", res[0])
	}
}

func TestLiveGames(t *testing.T) {
	g := NewGames(env(t, "RAWG_KEY"), t.TempDir())
	res, err := g.Search(context.Background(), "disco elysium", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("no results")
	}
	for _, r := range res {
		t.Logf("%-40s %-30s %d", trunc(r.Title, 38), trunc(r.Subtitle, 28), r.Year)
	}
	if res[0].Kind != "game" {
		t.Errorf("kind = %q, want game", res[0].Kind)
	}
}

func TestLivePodcasts(t *testing.T) {
	p := NewPodcasts(t.TempDir())
	res, err := p.Search(context.Background(), "darknet diaries", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("no results")
	}
	for _, r := range res {
		t.Logf("%-40s %-26s %d", trunc(r.Title, 38), trunc(r.Subtitle, 24), r.Year)
	}
	if res[0].Kind != "podcast" {
		t.Errorf("kind = %q, want podcast", res[0].Kind)
	}
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
