package store

import (
	"context"
	"fmt"
	"time"
)

type Pace struct {
	FirstAt   int64
	LastAt    int64
	Sessions  int
	Units     int
	Days      int
	PerWeek   float64
	HasEnough bool
}

// Pace reads the progress log rather than the entry, because the log is the
// only place that knows when things actually happened.
func (s *Store) Pace(ctx context.Context, entryID int64, now time.Time) (Pace, error) {
	var p Pace
	var first, last *int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*), MIN(at), MAX(at), COALESCE(SUM(to_pos - from_pos), 0)
		FROM progress WHERE entry_id = ? AND undone_at IS NULL AND to_pos > from_pos`,
		entryID).Scan(&p.Sessions, &first, &last, &p.Units)
	if err != nil {
		return p, fmt.Errorf("pace %d: %w", entryID, err)
	}
	if first == nil || last == nil {
		return p, nil
	}

	p.FirstAt, p.LastAt = *first, *last
	p.Days = int(now.Unix()-p.FirstAt)/86400 + 1
	if p.Days > 0 && p.Units > 0 {
		p.PerWeek = float64(p.Units) * 7 / float64(p.Days)
	}
	// One sitting says nothing about a rhythm.
	p.HasEnough = p.Sessions > 1 && p.Days > 1
	return p, nil
}

func (s *Store) SetNote(ctx context.Context, id int64, note string, now time.Time) (Entry, error) {
	value := any(note)
	if note == "" {
		value = nil
	}
	_, err := s.DB.ExecContext(ctx,
		`UPDATE entry SET note = ?, updated_at = ? WHERE id = ?`, value, now.Unix(), id)
	if err != nil {
		return Entry{}, fmt.Errorf("set note %d: %w", id, err)
	}
	return s.GetEntry(ctx, id)
}
