package store

import (
	"context"
	"fmt"
	"time"
)

type Link struct {
	ID    int64  `json:"-"`
	URL   string `json:"url"`
	Label string `json:"label"`
}

func (s *Store) AddLink(ctx context.Context, entryID int64, url, label string, now time.Time) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO link (entry_id, url, label, created_at) VALUES (?,?,?,?)`,
		entryID, url, label, now.Unix())
	if err != nil {
		return fmt.Errorf("add link: %w", err)
	}
	return nil
}

func (s *Store) DeleteLink(ctx context.Context, entryID, linkID int64) error {
	_, err := s.DB.ExecContext(ctx,
		`DELETE FROM link WHERE id = ? AND entry_id = ?`, linkID, entryID)
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	return nil
}

func (s *Store) LinksFor(ctx context.Context, entryID int64) ([]Link, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, url, label FROM link WHERE entry_id = ? ORDER BY id`, entryID)
	if err != nil {
		return nil, fmt.Errorf("links: %w", err)
	}
	defer rows.Close()

	var out []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.URL, &l.Label); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
