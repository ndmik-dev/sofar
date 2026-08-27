package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ndmik-dev/sofar/internal/domain"
)

type MediaInput struct {
	Kind       string
	Source     string
	TMDBType   string
	TMDBID     int
	Title      string
	TitleOrig  string
	Year       int
	Overview   string
	PosterPath string
	RuntimeMin int
	TotalUnits sql.NullInt64
	Unit       string
	Airing     string
}

// UpsertMedia writes catalog data, replacing the unit list wholesale. Progress
// lives on entry, not on units, so a re-import never disturbs where the user is.
func (s *Store) UpsertMedia(ctx context.Context, in MediaInput, units []domain.Unit, now time.Time) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var id int64
	if in.TMDBID != 0 {
		err := tx.QueryRowContext(ctx,
			`SELECT id FROM media WHERE tmdb_type = ? AND tmdb_id = ?`,
			in.TMDBType, in.TMDBID).Scan(&id)
		if err != nil && err != sql.ErrNoRows {
			return 0, fmt.Errorf("find media: %w", err)
		}
	}

	args := []any{
		in.Kind, in.Source, nullIfEmpty(in.TMDBType), nullIfZero(in.TMDBID),
		in.Title, nullIfEmpty(in.TitleOrig), nullIfZero(in.Year),
		nullIfEmpty(in.Overview), nullIfEmpty(in.PosterPath), nullIfZero(in.RuntimeMin),
		in.TotalUnits, in.Unit, nullIfEmpty(in.Airing), now.Unix(),
	}

	if id == 0 {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO media (kind, source, tmdb_type, tmdb_id, title, title_orig, year,
			                   overview, poster_path, runtime_min, total_units, unit,
			                   airing, refreshed_at, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, append(args, now.Unix())...)
		if err != nil {
			return 0, fmt.Errorf("insert media: %w", err)
		}
		if id, err = res.LastInsertId(); err != nil {
			return 0, err
		}
	} else {
		_, err := tx.ExecContext(ctx, `
			UPDATE media SET kind=?, source=?, tmdb_type=?, tmdb_id=?, title=?, title_orig=?,
			                 year=?, overview=?, poster_path=?, runtime_min=?, total_units=?,
			                 unit=?, airing=?, refreshed_at=?
			WHERE id = ?`, append(args, id)...)
		if err != nil {
			return 0, fmt.Errorf("update media: %w", err)
		}
	}

	if len(units) > 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM unit WHERE media_id = ?`, id); err != nil {
			return 0, fmt.Errorf("clear units: %w", err)
		}
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO unit (media_id, idx, season, number, title, air_date, runtime_min)
			VALUES (?,?,?,?,?,?,?)`)
		if err != nil {
			return 0, err
		}
		defer stmt.Close()
		for _, u := range units {
			_, err := stmt.ExecContext(ctx, id, u.Idx, u.Season, u.Number,
				nullIfEmpty(u.Title), nullIfEmpty(u.AirDate), nullIfZero(u.Runtime))
			if err != nil {
				return 0, fmt.Errorf("insert unit %d: %w", u.Idx, err)
			}
		}
	}

	return id, tx.Commit()
}

// AddEntry is idempotent: adding a title already on the shelf returns the entry
// that is there rather than creating a duplicate.
func (s *Store) AddEntry(ctx context.Context, userID, mediaID int64, status string, position int, now time.Time) (int64, bool, error) {
	var id int64
	err := s.DB.QueryRowContext(ctx,
		`SELECT id FROM entry WHERE user_id = ? AND media_id = ? AND run = 1 AND deleted_at IS NULL`,
		userID, mediaID).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, fmt.Errorf("find entry: %w", err)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()

	started, finished := any(nil), any(nil)
	if status == "active" || position > 0 {
		started = now.Unix()
	}
	if status == "done" {
		finished = now.Unix()
		// Finished means finished: an archive entry with a zero track would
		// read as a bug every time you opened the list.
		if position == 0 {
			position = totalUnitsOf(ctx, tx, mediaID)
		}
	}

	// A previously deleted entry for the same title still occupies the unique
	// key, so make room before inserting a fresh one.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM entry WHERE user_id = ? AND media_id = ? AND run = 1 AND deleted_at IS NOT NULL`,
		userID, mediaID); err != nil {
		return 0, false, fmt.Errorf("clear deleted entry: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO entry (user_id, media_id, status, position, started_at, finished_at,
		                   updated_at, created_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		userID, mediaID, status, position, started, finished, now.Unix(), now.Unix())
	if err != nil {
		return 0, false, fmt.Errorf("insert entry: %w", err)
	}
	if id, err = res.LastInsertId(); err != nil {
		return 0, false, err
	}

	if position > 0 {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO progress (entry_id, from_pos, to_pos, at, source)
			VALUES (?,0,?,?,'backfill')`, id, position, now.Unix())
		if err != nil {
			return 0, false, fmt.Errorf("seed progress: %w", err)
		}
	}

	return id, true, tx.Commit()
}

func nullIfZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func totalUnitsOf(ctx context.Context, tx *sql.Tx, mediaID int64) int {
	var total sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT total_units FROM media WHERE id = ?`, mediaID).Scan(&total); err != nil {
		return 0
	}
	return int(total.Int64)
}
