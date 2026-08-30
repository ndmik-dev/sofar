package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ndmik-dev/sofar/internal/domain"
	"github.com/ndmik-dev/sofar/internal/store"
	"github.com/ndmik-dev/sofar/internal/tmdb"
)

type Catalog struct {
	tmdb     *tmdb.Client
	books    *Books
	games    *Games
	podcasts *Podcasts
	store    *store.Store
}

func New(t *tmdb.Client, b *Books, g *Games, p *Podcasts, s *store.Store) *Catalog {
	return &Catalog{tmdb: t, books: b, games: g, podcasts: p, store: s}
}

func (c *Catalog) Enabled() bool { return c.tmdb.Enabled() }

// FindByKind routes to whichever catalog owns that type. A provider without a
// key returns nothing rather than an error: the manual form still works.
func (c *Catalog) FindByKind(ctx context.Context, kind, query string, limit int) ([]Found, error) {
	switch kind {
	case "book":
		return c.books.Search(ctx, query, limit)
	case "game":
		return c.games.Search(ctx, query, limit)
	case "podcast":
		return c.podcasts.Search(ctx, query, limit)
	}
	return nil, nil
}

func (c *Catalog) HasProvider(kind string) bool {
	switch kind {
	case "book":
		return c.books.Enabled()
	case "game":
		return c.games.Enabled()
	case "podcast":
		return c.podcasts.Enabled()
	}
	return false
}

func (c *Catalog) Search(ctx context.Context, query string, limit int) ([]tmdb.Result, error) {
	return c.tmdb.Search(ctx, query, limit)
}

// Import fetches a title and writes it to the catalog cache, returning the
// media row id. Re-importing an existing title refreshes it in place.
func (c *Catalog) Import(ctx context.Context, tmdbType string, tmdbID int, now time.Time) (int64, error) {
	var (
		d   tmdb.Details
		err error
	)
	switch tmdbType {
	case "tv":
		d, err = c.tmdb.TV(ctx, tmdbID)
	case "movie":
		d, err = c.tmdb.Movie(ctx, tmdbID)
	default:
		return 0, fmt.Errorf("unknown tmdb type %q", tmdbType)
	}
	if err != nil {
		return 0, err
	}

	in := store.MediaInput{
		Kind:       d.Kind,
		Source:     "tmdb",
		TMDBType:   d.TMDBType,
		TMDBID:     d.TMDBID,
		Title:      d.Title,
		TitleOrig:  d.Original,
		Year:       d.Year,
		Overview:   d.Overview,
		PosterPath: d.Poster,
		RuntimeMin: d.Runtime,
		Airing:     d.Airing,
	}

	var units []domain.Unit
	if tmdbType == "tv" {
		units = unitsOf(d)
		in.Unit = "episode"
		in.TotalUnits = sql.NullInt64{Int64: int64(len(units)), Valid: len(units) > 0}
	} else if d.Runtime > 0 {
		// A film is one long unit, and the useful position inside it is the
		// minute you stopped at. Without a runtime there is nothing to count.
		in.Unit = "minute"
		in.TotalUnits = sql.NullInt64{Int64: int64(d.Runtime), Valid: true}
	} else {
		in.Unit = "none"
		in.TotalUnits = sql.NullInt64{Int64: 1, Valid: true}
	}

	return c.store.UpsertMedia(ctx, in, units, now)
}

// idx runs 1..N straight through every season, which is what the track draws
// and what entry.position counts in.
func unitsOf(d tmdb.Details) []domain.Unit {
	units := make([]domain.Unit, 0, len(d.Episodes))
	for i, e := range d.Episodes {
		units = append(units, domain.Unit{
			Idx:     i + 1,
			Season:  e.Season,
			Number:  e.Number,
			Title:   e.Title,
			AirDate: e.AirDate,
			Runtime: e.Runtime,
		})
	}
	return units
}
