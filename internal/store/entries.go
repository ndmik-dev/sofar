package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ndmik-dev/sofar/internal/domain"
)

type Media struct {
	ID         int64
	Kind       string
	Source     string
	Title      string
	TitleOrig  string
	Year       int
	Overview   string
	RuntimeMin int
	TotalUnits sql.NullInt64
	Unit       string
	Airing     string
}

type Entry struct {
	ID         int64
	Status     string
	Position   int
	Rating     sql.NullInt64
	UpdatedAt  int64
	FinishedAt int64
	Media      Media
	Units      []domain.Unit
	Depth      string
	Step       int
}

const entrySelect = `
SELECT e.id, e.status, e.position, e.rating, e.updated_at, COALESCE(e.finished_at,0),
       m.id, m.kind, m.source, m.title, COALESCE(m.title_orig,''), COALESCE(m.year,0),
       COALESCE(m.overview,''), COALESCE(m.runtime_min,0), m.total_units, m.unit,
       COALESCE(m.airing,''),
       COALESCE(ts.depth,'units'), COALESCE(ts.step,1)
FROM entry e
JOIN media m ON m.id = e.media_id
LEFT JOIN type_settings ts ON ts.user_id = e.user_id AND ts.kind = m.kind
`

func (s *Store) GetEntry(ctx context.Context, id int64) (Entry, error) {
	entries, err := s.queryEntries(ctx, entrySelect+`WHERE e.id = ? AND e.deleted_at IS NULL`, id)
	if err != nil {
		return Entry{}, err
	}
	if len(entries) == 0 {
		return Entry{}, sql.ErrNoRows
	}
	return entries[0], nil
}

func (s *Store) ListEntries(ctx context.Context, userID int64, status string) ([]Entry, error) {
	return s.queryEntries(ctx,
		entrySelect+`WHERE e.user_id = ? AND e.status = ? AND e.deleted_at IS NULL
		             ORDER BY e.updated_at DESC`,
		userID, status)
}

func (s *Store) queryEntries(ctx context.Context, q string, args ...any) ([]Entry, error) {
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		err := rows.Scan(
			&e.ID, &e.Status, &e.Position, &e.Rating, &e.UpdatedAt, &e.FinishedAt,
			&e.Media.ID, &e.Media.Kind, &e.Media.Source, &e.Media.Title,
			&e.Media.TitleOrig, &e.Media.Year, &e.Media.Overview,
			&e.Media.RuntimeMin, &e.Media.TotalUnits, &e.Media.Unit, &e.Media.Airing,
			&e.Depth, &e.Step,
		)
		if err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := s.attachUnits(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) attachUnits(ctx context.Context, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}

	ids := make([]any, 0, len(entries))
	byMedia := make(map[int64][]int, len(entries))
	for i, e := range entries {
		if _, seen := byMedia[e.Media.ID]; !seen {
			ids = append(ids, e.Media.ID)
		}
		byMedia[e.Media.ID] = append(byMedia[e.Media.ID], i)
	}

	q := `SELECT media_id, idx, COALESCE(season,0), COALESCE(number,0),
	             COALESCE(title,''), COALESCE(air_date,''), COALESCE(runtime_min,0)
	      FROM unit WHERE media_id IN (` + placeholders(len(ids)) + `)
	      ORDER BY media_id, idx`

	rows, err := s.DB.QueryContext(ctx, q, ids...)
	if err != nil {
		return fmt.Errorf("list units: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var mediaID int64
		var u domain.Unit
		if err := rows.Scan(&mediaID, &u.Idx, &u.Season, &u.Number, &u.Title, &u.AirDate, &u.Runtime); err != nil {
			return fmt.Errorf("scan unit: %w", err)
		}
		for _, i := range byMedia[mediaID] {
			entries[i].Units = append(entries[i].Units, u)
		}
	}
	return rows.Err()
}

func (s *Store) CountsByStatus(ctx context.Context, userID int64) (map[string]int, error) {
	return s.countBy(ctx, `SELECT status, COUNT(*) FROM entry
	                       WHERE user_id = ? AND deleted_at IS NULL GROUP BY status`, userID)
}

func (s *Store) CountsByKind(ctx context.Context, userID int64) (map[string]int, error) {
	const q = `SELECT m.kind, COUNT(*) FROM entry e JOIN media m ON m.id = e.media_id
	           WHERE e.user_id = ? AND e.deleted_at IS NULL GROUP BY m.kind`
	return s.countBy(ctx, q, userID)
}

func (s *Store) countBy(ctx context.Context, q string, userID int64) (map[string]int, error) {
	rows, err := s.DB.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("count: %w", err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var key string
		var n int
		if err := rows.Scan(&key, &n); err != nil {
			return nil, err
		}
		out[key] = n
	}
	return out, rows.Err()
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
