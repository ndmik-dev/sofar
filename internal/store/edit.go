package store

import (
	"context"
	"fmt"
	"time"
)

type Edit struct {
	Title    string
	Subtitle string
	Total    int
	SetTotal bool
}

// UpdateDetails corrects what was typed at add time. It never touches
// updated_at: fixing a title is not watching, and a stale row must stay stale.
func (s *Store) UpdateDetails(ctx context.Context, id int64, ed Edit, now time.Time) (Entry, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Entry{}, err
	}
	defer tx.Rollback()

	var mediaID int64
	var position int
	if err := tx.QueryRowContext(ctx,
		`SELECT media_id, position FROM entry WHERE id = ? AND deleted_at IS NULL`,
		id).Scan(&mediaID, &position); err != nil {
		return Entry{}, err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE media SET title = ?, title_orig = ? WHERE id = ?`,
		ed.Title, nullIfEmpty(ed.Subtitle), mediaID); err != nil {
		return Entry{}, fmt.Errorf("update media: %w", err)
	}

	if ed.SetTotal {
		var total any
		if ed.Total > 0 {
			total = ed.Total
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE media SET total_units = ? WHERE id = ?`, total, mediaID); err != nil {
			return Entry{}, fmt.Errorf("update total: %w", err)
		}
		// A shorter book cannot leave you past its last page. The clamp is a
		// progress record like any other, because position is only a cache of
		// that log and must never be written behind its back.
		if ed.Total > 0 && position > ed.Total {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO progress (entry_id, from_pos, to_pos, at, source)
				VALUES (?,?,?,?,'setup')`, id, position, ed.Total, now.Unix()); err != nil {
				return Entry{}, fmt.Errorf("clamp progress: %w", err)
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE entry SET position = ? WHERE id = ?`, ed.Total, id); err != nil {
				return Entry{}, fmt.Errorf("clamp position: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return Entry{}, err
	}
	return s.GetEntry(ctx, id)
}
