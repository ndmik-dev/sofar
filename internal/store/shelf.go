package store

import (
	"context"
	"fmt"
	"strings"
)

type Shelf struct {
	EntryID int64
	Title   string
	Sub     string
	Kind    string
	Status  string
	Pos     int
	Total   int
	HasEnd  bool
}

// shelfMatches is the whole matching rule, kept apart so it can be tested
// without a database.
func shelfMatches(needle, title, sub string) bool {
	needle = strings.ToLower(needle)
	return strings.Contains(strings.ToLower(title), needle) ||
		(sub != "" && strings.Contains(strings.ToLower(sub), needle))
}

// SearchShelf finds what is already here. The catalog answers "what exists";
// this answers "where did I put it", which is the question that grows more
// common as the shelf does.
//
// Matching happens in Go, not in SQL: SQLite's LOWER and LIKE only fold ASCII,
// so "дюна" would never find "Дюна" — the one case this interface is for. A
// single person's shelf is small enough that reading the titles costs nothing.
func (s *Store) SearchShelf(ctx context.Context, userID int64, q string, limit int) ([]Shelf, error) {
	needle := strings.ToLower(strings.TrimSpace(q))
	if needle == "" {
		return nil, nil
	}

	rows, err := s.DB.QueryContext(ctx, `
		SELECT e.id, m.title, COALESCE(m.title_orig,''), m.kind, e.status,
		       e.position, m.total_units
		FROM entry e
		JOIN media m ON m.id = e.media_id
		WHERE e.user_id = ? AND e.deleted_at IS NULL
		ORDER BY e.updated_at DESC, e.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("search shelf: %w", err)
	}
	defer rows.Close()

	var starts, contains []Shelf
	for rows.Next() {
		var sh Shelf
		var total any
		if err := rows.Scan(&sh.EntryID, &sh.Title, &sh.Sub, &sh.Kind, &sh.Status, &sh.Pos, &total); err != nil {
			return nil, err
		}
		if n, ok := total.(int64); ok {
			sh.Total, sh.HasEnd = int(n), true
		}

		switch {
		case strings.HasPrefix(strings.ToLower(sh.Title), needle):
			starts = append(starts, sh)
		case shelfMatches(needle, sh.Title, sh.Sub):
			contains = append(contains, sh)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// A title that begins with what you typed is what you meant.
	out := append(starts, contains...)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
