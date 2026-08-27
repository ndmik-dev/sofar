package store

import (
	"context"
	"fmt"
	"time"
)

func (s *Store) SoftDelete(ctx context.Context, id int64, now time.Time) (Entry, error) {
	e, err := s.GetEntry(ctx, id)
	if err != nil {
		return Entry{}, err
	}
	_, err = s.DB.ExecContext(ctx,
		`UPDATE entry SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, now.Unix(), id)
	if err != nil {
		return Entry{}, fmt.Errorf("delete entry %d: %w", id, err)
	}
	return e, nil
}

func (s *Store) Restore(ctx context.Context, id int64, now time.Time) (Entry, error) {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE entry SET deleted_at = NULL, updated_at = ? WHERE id = ?`, now.Unix(), id)
	if err != nil {
		return Entry{}, fmt.Errorf("restore entry %d: %w", id, err)
	}
	return s.GetEntry(ctx, id)
}
