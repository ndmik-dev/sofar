package catalog

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/ndmik-dev/sofar/internal/fetch"
)

// Podcasts needs no key at all, so it is the one adapter that can never be
// switched off by a missing credential.
type Podcasts struct {
	fetch *fetch.Client
}

func NewPodcasts(cacheDir string) *Podcasts {
	return &Podcasts{fetch: fetch.New("https://itunes.apple.com", cacheDir, nil)}
}

func (p *Podcasts) Enabled() bool { return true }

type podcastResponse struct {
	Results []struct {
		ID       int    `json:"collectionId"`
		Name     string `json:"collectionName"`
		Artist   string `json:"artistName"`
		Artwork  string `json:"artworkUrl600"`
		Feed     string `json:"feedUrl"`
		Released string `json:"releaseDate"`
	} `json:"results"`
}

func (p *Podcasts) Search(ctx context.Context, query string, limit int) ([]Found, error) {
	var raw podcastResponse
	q := url.Values{
		"term":   {query},
		"media":  {"podcast"},
		"limit":  {strconv.Itoa(limit)},
		"entity": {"podcast"},
	}
	if err := p.fetch.GetJSON(ctx, "/search", q, time.Hour, &raw); err != nil {
		return nil, err
	}

	out := make([]Found, 0, limit)
	for _, r := range raw.Results {
		if r.Name == "" {
			continue
		}
		out = append(out, Found{
			Source:   "itunes",
			ExtID:    strconv.Itoa(r.ID),
			Kind:     "podcast",
			Title:    r.Name,
			Subtitle: r.Artist,
			Year:     yearOf(r.Released),
			Cover:    r.Artwork,
		})
	}
	return out, nil
}
