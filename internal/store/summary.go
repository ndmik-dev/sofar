package store

import (
	"context"
	"fmt"
)

type Summary struct {
	Waiting     int
	WaitingMins int
	Airing      int
	Active      int
}

// Summary counts only entries that are still moving: anything untouched since
// staleBefore has dropped out of the list and should not inflate the numbers.
func (s *Store) Summary(ctx context.Context, userID int64, today string, staleBefore int64) (Summary, error) {
	var out Summary

	err := s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(u.runtime_min), 0)
		FROM entry e
		JOIN unit u ON u.media_id = e.media_id AND u.idx > e.position
		WHERE e.user_id = ? AND e.status = 'active' AND e.deleted_at IS NULL
		  AND e.updated_at >= ? AND u.air_date IS NOT NULL AND u.air_date <= ?`,
		userID, staleBefore, today).Scan(&out.Waiting, &out.WaitingMins)
	if err != nil {
		return out, fmt.Errorf("summary waiting: %w", err)
	}

	err = s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(m.airing = 'returning'), 0)
		FROM entry e
		JOIN media m ON m.id = e.media_id
		WHERE e.user_id = ? AND e.status = 'active' AND e.deleted_at IS NULL
		  AND e.updated_at >= ?`,
		userID, staleBefore).Scan(&out.Active, &out.Airing)
	if err != nil {
		return out, fmt.Errorf("summary active: %w", err)
	}

	return out, nil
}
