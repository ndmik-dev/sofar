package store

import (
	"context"
	"fmt"
	"time"
)

type Refreshable struct {
	MediaID  int64
	TMDBType string
	TMDBID   int
	Title    string
}

// DueForRefresh lists shows still running that somebody is actually watching.
// A finished show cannot grow an episode, and nobody needs last year's dropped
// series polled every night.
func (s *Store) DueForRefresh(ctx context.Context, staleAfter time.Time, limit int) ([]Refreshable, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT DISTINCT m.id, m.tmdb_type, m.tmdb_id, m.title
		FROM media m
		JOIN entry e ON e.media_id = m.id
		WHERE m.airing = 'returning'
		  AND m.source = 'tmdb'
		  AND m.tmdb_id IS NOT NULL
		  AND e.deleted_at IS NULL
		  AND e.status IN ('active','backlog')
		  AND COALESCE(m.refreshed_at, 0) < ?
		ORDER BY COALESCE(m.refreshed_at, 0)
		LIMIT ?`, staleAfter.Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("due for refresh: %w", err)
	}
	defer rows.Close()

	var out []Refreshable
	for rows.Next() {
		var r Refreshable
		if err := rows.Scan(&r.MediaID, &r.TMDBType, &r.TMDBID, &r.Title); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PurgeDeleted finally removes what was soft-deleted long enough ago that undo
// is no longer a plausible intention.
func (s *Store) PurgeDeleted(ctx context.Context, before time.Time) (int, error) {
	res, err := s.DB.ExecContext(ctx,
		`DELETE FROM entry WHERE deleted_at IS NOT NULL AND deleted_at < ?`, before.Unix())
	if err != nil {
		return 0, fmt.Errorf("purge deleted: %w", err)
	}
	n, err := res.RowsAffected()
	return int(n), err
}
