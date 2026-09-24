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
	// Two weeks of nightly copies: enough to notice a bad day and go back past
	// it, small enough that a shelf never fills the volume.
	keepBackups = 14
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

// checkWatches asks each publisher whether a volume appeared. It runs whatever
// the TMDB key is: a manga watch has nothing to do with films.
func (j *Job) checkWatches(ctx context.Context, now time.Time) {
	watches, err := j.store.DueWatches(ctx)
	if err != nil {
		j.log.Error("list watches", "err", err)
		return
	}

	var found int
	for _, w := range watches {
		select {
		case <-ctx.Done():
			return
		case <-time.After(betweenCalls):
		}

		vols, err := j.catalog.Volumes(ctx, w.Query)
		if err != nil {
			j.log.Warn("watch failed", "title", w.Title, "err", err)
			continue
		}
		newest := catalog.Release{}
		for _, v := range vols {
			if v.Volume > newest.Volume {
				newest = v
			}
		}
		if newest.Volume <= w.LastVol {
			if err := j.store.MarkChecked(ctx, w.EntryID, now); err != nil {
				j.log.Error("mark checked", "title", w.Title, "err", err)
			}
			continue
		}
		if err := j.store.RecordRelease(ctx, w.EntryID, newest.Volume, newest.Title, newest.URL, now); err != nil {
			j.log.Error("record release", "title", w.Title, "err", err)
			continue
		}
		j.log.Info("new volume", "title", w.Title, "volume", newest.Volume)
		found++
	}
	if len(watches) > 0 {
		j.log.Info("checked watches", "watches", len(watches), "new", found)
	}
}

// backup writes tonight's copy and drops the oldest ones. A failed copy is
// logged loudly and nothing else is skipped: a missing backup is bad, a stale
// shelf on top of it is worse.
func (j *Job) backup(ctx context.Context, now time.Time) {
	f, err := j.store.Backup(ctx, j.cfg.BackupDir, now)
	if err != nil {
		j.log.Error("backup failed", "dir", j.cfg.BackupDir, "err", err)
		return
	}
	removed, err := store.PruneBackups(j.cfg.BackupDir, keepBackups)
	if err != nil {
		j.log.Error("prune backups", "err", err)
	}
	j.log.Info("backup written", "file", f.Name, "bytes", f.Size, "pruned", removed)
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

	// The copy comes first: whatever the rest of the night does to the data,
	// the file from before it exists.
	j.backup(ctx, now)

	if n, err := j.store.PurgeDeleted(ctx, now.Add(-keepDeleted)); err != nil {
		j.log.Error("purge deleted", "err", err)
	} else if n > 0 {
		j.log.Info("purged deleted entries", "entries", n)
	}

	j.checkWatches(ctx, now)

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
