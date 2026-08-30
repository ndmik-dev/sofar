package store

import (
	"context"
	"fmt"

	"github.com/ndmik-dev/sofar/internal/domain"
)

// ProgressRows returns forward manual progress in [from, to). Backfill and
// import are excluded on purpose: dumping an archive into the app must not
// read as a heroic watching day.
func (s *Store) ProgressRows(ctx context.Context, userID, from, to int64) ([]domain.ProgressRow, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT p.at, p.to_pos - p.from_pos, p.to_pos, COALESCE(m.total_units, 0),
		       m.kind, m.unit, COALESCE(m.runtime_min, 0)
		FROM progress p
		JOIN entry e ON e.id = p.entry_id
		JOIN media m ON m.id = e.media_id
		WHERE e.user_id = ? AND p.undone_at IS NULL AND p.source = 'manual'
		  AND p.to_pos > p.from_pos AND p.at >= ? AND p.at < ?`,
		userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("progress rows: %w", err)
	}
	defer rows.Close()

	var out []domain.ProgressRow
	for rows.Next() {
		var r domain.ProgressRow
		if err := rows.Scan(&r.At, &r.Delta, &r.To, &r.Total, &r.Kind, &r.Unit, &r.Runtime); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FinishedInYear(ctx context.Context, userID int64, kind string, from, to int64) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM entry e JOIN media m ON m.id = e.media_id
		WHERE e.user_id = ? AND m.kind = ? AND e.status = 'done'
		  AND e.deleted_at IS NULL AND e.finished_at >= ? AND e.finished_at < ?`,
		userID, kind, from, to).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("finished in year: %w", err)
	}
	return n, nil
}

type TypeSetting struct {
	Kind  string
	Depth string
	Step  int
}

func (s *Store) TypeSettings(ctx context.Context, userID int64) (map[string]TypeSetting, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT kind, depth, step FROM type_settings WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("type settings: %w", err)
	}
	defer rows.Close()

	out := map[string]TypeSetting{}
	for rows.Next() {
		var ts TypeSetting
		if err := rows.Scan(&ts.Kind, &ts.Depth, &ts.Step); err != nil {
			return nil, err
		}
		out[ts.Kind] = ts
	}
	return out, rows.Err()
}

func (s *Store) SetTypeSetting(ctx context.Context, userID int64, ts TypeSetting) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO type_settings (user_id, kind, depth, step) VALUES (?,?,?,?)
		ON CONFLICT (user_id, kind) DO UPDATE SET depth = excluded.depth, step = excluded.step`,
		userID, ts.Kind, ts.Depth, ts.Step)
	if err != nil {
		return fmt.Errorf("set type setting: %w", err)
	}
	return nil
}
