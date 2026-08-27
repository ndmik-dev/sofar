package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL      = "https://api.themoviedb.org/3"
	langPrimary  = "uk-UA"
	langFallback = "en-US"
	animeGenre   = 16
)

type Client struct {
	token string
	http  *http.Client
	cache *Cache
	gate  chan struct{}
}

func New(token, cacheDir string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 15 * time.Second},
		cache: NewCache(cacheDir),
		// TMDB has no hard limit any more, but a three-season import fires
		// four calls at once; four in flight is plenty and stays polite.
		gate: make(chan struct{}, 4),
	}
}

func (c *Client) Enabled() bool { return c.token != "" }

type Result struct {
	TMDBType string
	TMDBID   int
	Kind     string
	Title    string
	Original string
	Year     int
	Overview string
	Poster   string
	Episodes int
}

type Details struct {
	Result
	Runtime  int
	Airing   string
	Seasons  []SeasonRef
	Episodes []Episode
}

type SeasonRef struct {
	Number   int
	Episodes int
}

type Episode struct {
	Season  int
	Number  int
	Title   string
	AirDate string
	Runtime int
}

func (c *Client) get(ctx context.Context, path string, q url.Values, ttl time.Duration, out any) error {
	if !c.Enabled() {
		return fmt.Errorf("TMDB_TOKEN is not set")
	}

	full := path
	if len(q) > 0 {
		full += "?" + q.Encode()
	}

	if body, ok := c.cache.Get(full, ttl); ok {
		return json.Unmarshal(body, out)
	}

	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+full, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("tmdb %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := readLimited(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tmdb %s: %s: %s", path, resp.Status, snippet(body))
	}

	c.cache.Put(full, body)
	return json.Unmarshal(body, out)
}

type searchResponse struct {
	Results []struct {
		ID            int      `json:"id"`
		MediaType     string   `json:"media_type"`
		Name          string   `json:"name"`
		Title         string   `json:"title"`
		OriginalName  string   `json:"original_name"`
		OriginalT     string   `json:"original_title"`
		Overview      string   `json:"overview"`
		PosterPath    string   `json:"poster_path"`
		FirstAir      string   `json:"first_air_date"`
		Release       string   `json:"release_date"`
		GenreIDs      []int    `json:"genre_ids"`
		OriginCountry []string `json:"origin_country"`
	} `json:"results"`
}

func (c *Client) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	var raw searchResponse
	q := url.Values{
		"query":         {query},
		"language":      {langPrimary},
		"include_adult": {"false"},
	}
	if err := c.get(ctx, "/search/multi", q, time.Hour, &raw); err != nil {
		return nil, err
	}

	out := make([]Result, 0, limit)
	for _, r := range raw.Results {
		if r.MediaType != "tv" && r.MediaType != "movie" {
			continue
		}
		res := Result{
			TMDBType: r.MediaType,
			TMDBID:   r.ID,
			Title:    firstNonEmpty(r.Name, r.Title),
			Original: firstNonEmpty(r.OriginalName, r.OriginalT),
			Overview: r.Overview,
			Poster:   r.PosterPath,
			Year:     yearOf(firstNonEmpty(r.FirstAir, r.Release)),
		}
		res.Kind = kindFor(r.MediaType, r.GenreIDs, r.OriginCountry)
		if res.Original == res.Title {
			res.Original = ""
		}
		out = append(out, res)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

type tvResponse struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	OriginalName  string   `json:"original_name"`
	Overview      string   `json:"overview"`
	PosterPath    string   `json:"poster_path"`
	FirstAir      string   `json:"first_air_date"`
	Status        string   `json:"status"`
	InProduction  bool     `json:"in_production"`
	EpisodeRunMin []int    `json:"episode_run_time"`
	OriginCountry []string `json:"origin_country"`
	Genres        []struct {
		ID int `json:"id"`
	} `json:"genres"`
	Seasons []struct {
		SeasonNumber int `json:"season_number"`
		EpisodeCount int `json:"episode_count"`
	} `json:"seasons"`
}

type seasonResponse struct {
	Episodes []struct {
		EpisodeNumber int    `json:"episode_number"`
		Name          string `json:"name"`
		AirDate       string `json:"air_date"`
		Runtime       int    `json:"runtime"`
	} `json:"episodes"`
}

// TV pulls the show plus every episode of every regular season. Season 0 holds
// specials and is skipped: including it would shift the absolute numbering that
// the whole progress model relies on.
func (c *Client) TV(ctx context.Context, id int) (Details, error) {
	var raw tvResponse
	path := "/tv/" + strconv.Itoa(id)
	if err := c.get(ctx, path, url.Values{"language": {langPrimary}}, 24*time.Hour, &raw); err != nil {
		return Details{}, err
	}

	genres := make([]int, 0, len(raw.Genres))
	for _, g := range raw.Genres {
		genres = append(genres, g.ID)
	}

	d := Details{
		Result: Result{
			TMDBType: "tv",
			TMDBID:   raw.ID,
			Kind:     kindFor("tv", genres, raw.OriginCountry),
			Title:    raw.Name,
			Original: raw.OriginalName,
			Overview: raw.Overview,
			Poster:   raw.PosterPath,
			Year:     yearOf(raw.FirstAir),
		},
		Airing: "ended",
	}
	if d.Original == d.Title {
		d.Original = ""
	}
	if raw.InProduction || raw.Status == "Returning Series" {
		d.Airing = "returning"
	}
	if len(raw.EpisodeRunMin) > 0 {
		d.Runtime = raw.EpisodeRunMin[0]
	}

	if d.Overview == "" {
		var en tvResponse
		if err := c.get(ctx, path, url.Values{"language": {langFallback}}, 24*time.Hour, &en); err == nil {
			d.Overview = en.Overview
		}
	}

	for _, s := range raw.Seasons {
		if s.SeasonNumber == 0 || s.EpisodeCount == 0 {
			continue
		}
		var sr seasonResponse
		sp := fmt.Sprintf("/tv/%d/season/%d", id, s.SeasonNumber)
		if err := c.get(ctx, sp, url.Values{"language": {langPrimary}}, 24*time.Hour, &sr); err != nil {
			return Details{}, err
		}
		d.Seasons = append(d.Seasons, SeasonRef{Number: s.SeasonNumber, Episodes: len(sr.Episodes)})
		for _, e := range sr.Episodes {
			runtime := e.Runtime
			if runtime == 0 {
				runtime = d.Runtime
			}
			d.Episodes = append(d.Episodes, Episode{
				Season:  s.SeasonNumber,
				Number:  e.EpisodeNumber,
				Title:   e.Name,
				AirDate: e.AirDate,
				Runtime: runtime,
			})
		}
	}
	d.Result.Episodes = len(d.Episodes)
	if d.Runtime == 0 {
		d.Runtime = medianRuntime(d.Episodes)
	}
	return d, nil
}

// TMDB leaves episode_run_time empty for plenty of modern shows, but the
// episodes themselves carry a runtime. Without this the "how much is left"
// figure would silently read zero.
func medianRuntime(eps []Episode) int {
	mins := make([]int, 0, len(eps))
	for _, e := range eps {
		if e.Runtime > 0 {
			mins = append(mins, e.Runtime)
		}
	}
	if len(mins) == 0 {
		return 0
	}
	sort.Ints(mins)
	return mins[len(mins)/2]
}

type movieResponse struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	OriginalT   string `json:"original_title"`
	Overview    string `json:"overview"`
	PosterPath  string `json:"poster_path"`
	ReleaseDate string `json:"release_date"`
	Runtime     int    `json:"runtime"`
}

func (c *Client) Movie(ctx context.Context, id int) (Details, error) {
	var raw movieResponse
	path := "/movie/" + strconv.Itoa(id)
	if err := c.get(ctx, path, url.Values{"language": {langPrimary}}, 24*time.Hour, &raw); err != nil {
		return Details{}, err
	}

	d := Details{
		Result: Result{
			TMDBType: "movie",
			TMDBID:   raw.ID,
			Kind:     "movie",
			Title:    raw.Title,
			Original: raw.OriginalT,
			Overview: raw.Overview,
			Poster:   raw.PosterPath,
			Year:     yearOf(raw.ReleaseDate),
			Episodes: 1,
		},
		Runtime: raw.Runtime,
	}
	if d.Original == d.Title {
		d.Original = ""
	}
	if d.Overview == "" {
		var en movieResponse
		if err := c.get(ctx, path, url.Values{"language": {langFallback}}, 24*time.Hour, &en); err == nil {
			d.Overview = en.Overview
		}
	}
	return d, nil
}

// Japanese animation gets its own kind so it can be filtered and coloured
// separately, even though TMDB serves it as an ordinary series.
func kindFor(mediaType string, genres []int, countries []string) string {
	if mediaType == "movie" {
		return "movie"
	}
	animated := false
	for _, g := range genres {
		if g == animeGenre {
			animated = true
		}
	}
	if animated {
		for _, c := range countries {
			if strings.EqualFold(c, "JP") {
				return "anime"
			}
		}
	}
	return "show"
}

func yearOf(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
