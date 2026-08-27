package catalog

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/ndmik-dev/sofar/internal/fetch"
)

type Games struct {
	key   string
	fetch *fetch.Client
}

func NewGames(key, cacheDir string) *Games {
	return &Games{key: key, fetch: fetch.New("https://api.rawg.io/api", cacheDir, nil)}
}

func (g *Games) Enabled() bool { return g.key != "" }

type gamesResponse struct {
	Results []struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Released  string `json:"released"`
		Image     string `json:"background_image"`
		Platforms []struct {
			Platform struct {
				Name string `json:"name"`
			} `json:"platform"`
		} `json:"parent_platforms"`
	} `json:"results"`
}

func (g *Games) Search(ctx context.Context, query string, limit int) ([]Found, error) {
	if !g.Enabled() {
		return nil, nil
	}
	var raw gamesResponse
	q := url.Values{
		"search":    {query},
		"page_size": {strconv.Itoa(limit)},
		"key":       {g.key},
	}
	if err := g.fetch.GetJSON(ctx, "/games", q, time.Hour, &raw); err != nil {
		return nil, err
	}

	out := make([]Found, 0, limit)
	for _, r := range raw.Results {
		var platforms []string
		for _, p := range r.Platforms {
			platforms = append(platforms, p.Platform.Name)
		}
		out = append(out, Found{
			Source:   "rawg",
			ExtID:    strconv.Itoa(r.ID),
			Kind:     "game",
			Title:    r.Name,
			Subtitle: joinMax(platforms, 3),
			Year:     yearOf(r.Released),
			Cover:    r.Image,
		})
	}
	return out, nil
}
