package store

import (
	"context"
	"database/sql"
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

// Restore leaves updated_at untouched: undo means "as it was", and bumping the
// timestamp would fling the row to the top of the list.
func (s *Store) Restore(ctx context.Context, id int64) (Entry, error) {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE entry SET deleted_at = NULL WHERE id = ?`, id)
	if err != nil {
		return Entry{}, fmt.Errorf("restore entry %d: %w", id, err)
	}
	return s.GetEntry(ctx, id)
}

// Successor is the id of the entry a restored row should sit above, per the
// list ordering (updated_at DESC). Zero means it belongs at the end.
func (s *Store) Successor(ctx context.Context, userID int64, status string, updatedAt, excludeID int64) (int64, error) {
	var id int64
	// Ties on updated_at are broken by id, mirroring the list ordering —
	// otherwise a restored row lands one slot off whenever timestamps collide.
	err := s.DB.QueryRowContext(ctx, `
		SELECT id FROM entry
		WHERE user_id = ? AND status = ? AND deleted_at IS NULL
		  AND (updated_at < ? OR (updated_at = ? AND id < ?))
		ORDER BY updated_at DESC, id DESC LIMIT 1`,
		userID, status, updatedAt, updatedAt, excludeID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("successor: %w", err)
	}
	return id, nil
}
