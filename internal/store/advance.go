package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Move struct {
	Entry    Entry
	From     int
	To       int
	Changed  bool
	Finished bool
}

// Advance moves an entry to abs when it is non-nil, otherwise by delta.
// Reaching the last unit closes the entry; Undo reopens it.
func (s *Store) Advance(ctx context.Context, id int64, delta int, abs *int, now time.Time) (Move, error) {
	return s.reposition(ctx, id, now, func(cur, total int, hasTotal bool) int {
		want := cur + delta
		if abs != nil {
			want = *abs
		}
		return clamp(want, total, hasTotal)
	}, "manual")
}

// Undo reverses the newest progress record. Status and position are recomputed
// from the log, never guessed, so undoing a completion reopens the entry.
func (s *Store) Undo(ctx context.Context, id int64, now time.Time) (Move, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Move{}, err
	}
	defer tx.Rollback()

	var progressID int64
	var from int
	err = tx.QueryRowContext(ctx, `
		SELECT id, from_pos FROM progress
		WHERE entry_id = ? AND undone_at IS NULL
		ORDER BY at DESC, id DESC LIMIT 1`, id).Scan(&progressID, &from)
	if err == sql.ErrNoRows {
		e, err := s.GetEntry(ctx, id)
		return Move{Entry: e, From: e.Position, To: e.Position}, err
	}
	if err != nil {
		return Move{}, fmt.Errorf("find last progress: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE progress SET undone_at = ? WHERE id = ?`, now.Unix(), progressID); err != nil {
		return Move{}, fmt.Errorf("mark undone: %w", err)
	}

	cur, total, hasTotal, err := positionOf(ctx, tx, id)
	if err != nil {
		return Move{}, err
	}
	if err := applyPosition(ctx, tx, id, from, total, hasTotal, now); err != nil {
		return Move{}, err
	}
	if err := tx.Commit(); err != nil {
		return Move{}, err
	}

	e, err := s.GetEntry(ctx, id)
	return Move{Entry: e, From: cur, To: from, Changed: cur != from}, err
}

func (s *Store) reposition(ctx context.Context, id int64, now time.Time, next func(cur, total int, hasTotal bool) int, source string) (Move, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Move{}, err
	}
	defer tx.Rollback()

	cur, total, hasTotal, err := positionOf(ctx, tx, id)
	if err != nil {
		return Move{}, err
	}

	to := next(cur, total, hasTotal)
	if to == cur {
		e, err := s.GetEntry(ctx, id)
		return Move{Entry: e, From: cur, To: cur}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO progress (entry_id, from_pos, to_pos, at, source)
		VALUES (?,?,?,?,?)`, id, cur, to, now.Unix(), source); err != nil {
		return Move{}, fmt.Errorf("log progress: %w", err)
	}
	if err := applyPosition(ctx, tx, id, to, total, hasTotal, now); err != nil {
		return Move{}, err
	}
	if err := tx.Commit(); err != nil {
		return Move{}, err
	}

	e, err := s.GetEntry(ctx, id)
	return Move{
		Entry:    e,
		From:     cur,
		To:       to,
		Changed:  true,
		Finished: hasTotal && to >= total,
	}, err
}

func positionOf(ctx context.Context, tx *sql.Tx, id int64) (pos, total int, hasTotal bool, err error) {
	var t sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT e.position, m.total_units
		FROM entry e JOIN media m ON m.id = e.media_id
		WHERE e.id = ?`, id).Scan(&pos, &t)
	if err != nil {
		return 0, 0, false, fmt.Errorf("read entry %d: %w", id, err)
	}
	return pos, int(t.Int64), t.Valid, nil
}

func applyPosition(ctx context.Context, tx *sql.Tx, id int64, to, total int, hasTotal bool, now time.Time) error {
	status, finished := "active", any(nil)
	if hasTotal && to >= total {
		status, finished = "done", now.Unix()
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE entry SET position = ?, status = ?, finished_at = ?, updated_at = ?
		WHERE id = ?`, to, status, finished, now.Unix(), id)
	if err != nil {
		return fmt.Errorf("update entry %d: %w", id, err)
	}
	return nil
}

func (s *Store) SetStatus(ctx context.Context, id int64, status string, now time.Time) (Entry, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Entry{}, err
	}
	defer tx.Rollback()

	finished := any(nil)
	position, total, hasTotal, err := positionOf(ctx, tx, id)
	if err != nil {
		return Entry{}, err
	}
	if status == "done" {
		finished = now.Unix()
		if hasTotal && position < total {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO progress (entry_id, from_pos, to_pos, at, source)
				VALUES (?,?,?,?,'manual')`, id, position, total, now.Unix()); err != nil {
				return Entry{}, fmt.Errorf("log completion: %w", err)
			}
			position = total
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE entry SET status = ?, position = ?, finished_at = ?, updated_at = ? WHERE id = ?`,
		status, position, finished, now.Unix(), id); err != nil {
		return Entry{}, fmt.Errorf("set status %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return Entry{}, err
	}
	return s.GetEntry(ctx, id)
}

func clamp(want, total int, hasTotal bool) int {
	if want < 0 {
		return 0
	}
	if hasTotal && want > total {
		return total
	}
	return want
}
