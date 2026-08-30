// Package nightly keeps the shelf current while nobody is looking: new
// episodes of running shows appear on their own, and long-deleted entries
// finally go away.
package nightly

import (
	"context"
	"log/slog"
	"time"

	"github.com/ndmik-dev/sofar/internal/catalog"
	"github.com/ndmik-dev/sofar/internal/config"
	"github.com/ndmik-dev/sofar/internal/store"
)

const (
	// TMDB is polite but not free, and a shelf does not change by the minute.
	refreshBatch = 40
	refreshEvery = 20 * time.Hour
	betweenCalls = 700 * time.Millisecond
	keepDeleted  = 30 * 24 * time.Hour
)

type Job struct {
	cfg     config.Config
	store   *store.Store
	catalog *catalog.Catalog
	log     *slog.Logger
}

func New(cfg config.Config, st *store.Store, cat *catalog.Catalog, log *slog.Logger) *Job {
	return &Job{cfg: cfg, store: st, catalog: cat, log: log}
}

// Run blocks until the context is cancelled, waking once a day at the hour the
// day itself starts — the quietest moment there is by definition.
func (j *Job) Run(ctx context.Context) {
	for {
		wait := time.Until(j.nextRun(time.Now().In(j.cfg.Loc)))
		j.log.Info("nightly scheduled", "in", wait.Round(time.Minute).String())

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		j.Once(ctx)
	}
}

func (j *Job) nextRun(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), j.cfg.DayStart, 20, 0, 0, j.cfg.Loc)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// Once is the whole night's work, exported so it can be triggered by hand.
func (j *Job) Once(ctx context.Context) {
	now := time.Now().In(j.cfg.Loc)

	if n, err := j.store.PurgeDeleted(ctx, now.Add(-keepDeleted)); err != nil {
		j.log.Error("purge deleted", "err", err)
	} else if n > 0 {
		j.log.Info("purged deleted entries", "entries", n)
	}

	if !j.catalog.Enabled() {
		return
	}
	due, err := j.store.DueForRefresh(ctx, now.Add(-refreshEvery), refreshBatch)
	if err != nil {
		j.log.Error("list refreshable", "err", err)
		return
	}

	var done, failed int
	for _, r := range due {
		select {
		case <-ctx.Done():
			return
		case <-time.After(betweenCalls):
		}
		// Import replaces the unit list wholesale. Progress lives on entry, so
		// a re-import never disturbs where the user is.
		if _, err := j.catalog.Import(ctx, r.TMDBType, r.TMDBID, now); err != nil {
			// One unreachable title must not stop the rest of the night.
			j.log.Warn("refresh failed", "title", r.Title, "err", err)
			failed++
			continue
		}
		done++
	}
	if done > 0 || failed > 0 {
		j.log.Info("refreshed running shows", "ok", done, "failed", failed, "due", len(due))
	}
}
