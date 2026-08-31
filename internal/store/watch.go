package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Watch struct {
	EntryID   int64
	Source    string
	Query     string
	LastVol   int
	LastTitle string
	LastURL   string
	FoundAt   int64
	CheckedAt int64
	Title     string // of the entry, for logging
}

func (s *Store) SetWatch(ctx context.Context, entryID int64, source, query string, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO watch (entry_id, source, query, created_at) VALUES (?,?,?,?)
		ON CONFLICT (entry_id) DO UPDATE SET source = excluded.source, query = excluded.query`,
		entryID, source, query, now.Unix())
	if err != nil {
		return fmt.Errorf("set watch: %w", err)
	}
	return nil
}

func (s *Store) DropWatch(ctx context.Context, entryID int64) error {
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM watch WHERE entry_id = ?`, entryID); err != nil {
		return fmt.Errorf("drop watch: %w", err)
	}
	return nil
}

func (s *Store) WatchFor(ctx context.Context, entryID int64) (Watch, bool, error) {
	var w Watch
	var title, u sql.NullString
	var found, checked sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `
		SELECT entry_id, source, query, last_vol, last_title, last_url, found_at, checked_at
		FROM watch WHERE entry_id = ?`, entryID).
		Scan(&w.EntryID, &w.Source, &w.Query, &w.LastVol, &title, &u, &found, &checked)
	if err == sql.ErrNoRows {
		return Watch{}, false, nil
	}
	if err != nil {
		return Watch{}, false, fmt.Errorf("watch: %w", err)
	}
	w.LastTitle, w.LastURL = title.String, u.String
	w.FoundAt, w.CheckedAt = found.Int64, checked.Int64
	return w, true, nil
}

// DueWatches lists watches on entries that still exist and are not deleted.
func (s *Store) DueWatches(ctx context.Context) ([]Watch, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT w.entry_id, w.source, w.query, w.last_vol, m.title
		FROM watch w
		JOIN entry e ON e.id = w.entry_id
		JOIN media m ON m.id = e.media_id
		WHERE e.deleted_at IS NULL AND e.status IN ('active','backlog')
		ORDER BY COALESCE(w.checked_at, 0)`)
	if err != nil {
		return nil, fmt.Errorf("due watches: %w", err)
	}
	defer rows.Close()

	var out []Watch
	for rows.Next() {
		var w Watch
		if err := rows.Scan(&w.EntryID, &w.Source, &w.Query, &w.LastVol, &w.Title); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// RecordRelease remembers the newest volume seen. found_at only moves when the
// number actually grows, so "new" means new to you, not "checked recently".
func (s *Store) RecordRelease(ctx context.Context, entryID int64, vol int, title, url string, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE watch SET last_vol = ?, last_title = ?, last_url = ?, found_at = ?, checked_at = ?
		WHERE entry_id = ?`, vol, title, url, now.Unix(), now.Unix(), entryID)
	if err != nil {
		return fmt.Errorf("record release: %w", err)
	}
	return nil
}

func (s *Store) MarkChecked(ctx context.Context, entryID int64, now time.Time) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE watch SET checked_at = ? WHERE entry_id = ?`, now.Unix(), entryID)
	return err
}
