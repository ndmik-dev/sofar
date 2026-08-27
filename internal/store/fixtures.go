package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type fixtureSeason struct {
	Episodes int
	Titles   []string
}

type fixture struct {
	Kind       string
	Source     string
	Title      string
	TitleOrig  string
	Year       int
	Overview   string
	Runtime    int
	Unit       string
	Total      sql.NullInt64
	Airing     string
	Seasons    []fixtureSeason
	AiredUpTo  int
	Position   int
	Status     string
	StaleDays  int
	WeeklyFrom int
}

// SeedFixtures fills an empty database with the set the mockups were drawn
// against, so the list has something to render before TMDB exists.
func (s *Store) SeedFixtures(ctx context.Context, userID int64, now time.Time) (int, error) {
	var n int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM entry WHERE user_id = ?`, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count entries: %w", err)
	}
	if n > 0 {
		return 0, nil
	}

	for _, f := range fixtures() {
		if err := s.insertFixture(ctx, userID, f, now); err != nil {
			return 0, err
		}
	}
	return len(fixtures()), nil
}

func fixtures() []fixture {
	return []fixture{
		{
			Kind: "show", Source: "tmdb", Title: "Severance", Year: 2022, Runtime: 47,
			Overview: "Марк керує невеликим відділом у Lumon Industries, де співробітники погодились на розділення памʼяті: на роботі вони нічого не знають про своє життя ззовні, а вдома — нічого про роботу.",
			Unit:     "episode", Airing: "returning",
			Seasons: []fixtureSeason{
				{Episodes: 9},
				{Episodes: 10, Titles: []string{
					"Hello, Ms. Cobel", "Goodbye, Mrs. Selvig", "Who Is Alive?", "Woe's Hollow",
					"Trojan's Horse", "Attila", "Chikhai Bardo", "Sweet Vitriol",
					"The After Hours", "Cold Harbor",
				}},
			},
			AiredUpTo: 19, Position: 13, Status: "active", WeeklyFrom: 210,
		},
		{
			Kind: "anime", Source: "tmdb", Title: "Frieren: Beyond Journey's End",
			TitleOrig: "Sousou no Frieren", Year: 2023, Runtime: 24,
			Overview: "Ельфійка Фрірен пережила своїх супутників на століття і лише після їхньої смерті починає розуміти, чого не встигла в них спитати.",
			Unit:     "episode", Airing: "returning",
			Seasons:   []fixtureSeason{{Episodes: 28}},
			AiredUpTo: 24, Position: 18, Status: "active", WeeklyFrom: 168,
		},
		{
			Kind: "book", Source: "manual", Title: "Проєкт «Аве Марія»",
			TitleOrig: "Енді Вейр", Year: 2021, Unit: "hour",
			Total:    sql.NullInt64{Int64: 16, Valid: true},
			Position: 12, Status: "active", StaleDays: 1,
		},
		{
			Kind: "book", Source: "manual", Title: "Ім'я вітру",
			TitleOrig: "Патрік Ротфусс", Year: 2007, Unit: "page",
			Total:    sql.NullInt64{Int64: 662, Valid: true},
			Position: 218, Status: "active", StaleDays: 3,
		},
		{
			Kind: "game", Source: "manual", Title: "Baldur's Gate 3", Year: 2023,
			Unit: "none", Status: "active", StaleDays: 3,
		},
		{
			Kind: "show", Source: "tmdb", Title: "The Bear", Year: 2022, Runtime: 32,
			Unit: "episode", Airing: "returning",
			Seasons: []fixtureSeason{
				{Episodes: 8}, {Episodes: 10}, {Episodes: 10},
			},
			AiredUpTo: 28, Position: 22, Status: "active", StaleDays: 7, WeeklyFrom: 400,
		},
		{
			Kind: "anime", Source: "tmdb", Title: "Vinland Saga", Year: 2019, Runtime: 24,
			Unit: "episode", Airing: "ended",
			Seasons: []fixtureSeason{
				{Episodes: 24},
			},
			AiredUpTo: 24, Position: 5, Status: "active", StaleDays: 64, WeeklyFrom: 900,
		},
	}
}

func (s *Store) insertFixture(ctx context.Context, userID int64, f fixture, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	total := f.Total
	if len(f.Seasons) > 0 {
		var sum int64
		for _, sn := range f.Seasons {
			sum += int64(sn.Episodes)
		}
		total = sql.NullInt64{Int64: sum, Valid: true}
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO media (kind, source, title, title_orig, year, overview, runtime_min,
		                   total_units, unit, airing, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		f.Kind, f.Source, f.Title, f.TitleOrig, f.Year, f.Overview, f.Runtime,
		total, f.Unit, nullIfEmpty(f.Airing), now.Unix())
	if err != nil {
		return fmt.Errorf("insert media %s: %w", f.Title, err)
	}
	mediaID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	idx := 0
	for si, sn := range f.Seasons {
		for ep := 1; ep <= sn.Episodes; ep++ {
			idx++
			title := ""
			if ep-1 < len(sn.Titles) {
				title = sn.Titles[ep-1]
			}
			// Weekly release counting back from WeeklyFrom days ago, so the
			// fixture keeps showing aired-and-waiting cells as time passes.
			day := now.AddDate(0, 0, -f.WeeklyFrom+(idx-1)*7)
			airDate := day.Format("2006-01-02")
			if idx > f.AiredUpTo {
				airDate = now.AddDate(0, 0, (idx-f.AiredUpTo)*7).Format("2006-01-02")
			}
			_, err := tx.ExecContext(ctx, `
				INSERT INTO unit (media_id, idx, season, number, title, air_date, runtime_min)
				VALUES (?,?,?,?,?,?,?)`,
				mediaID, idx, si+1, ep, nullIfEmpty(title), airDate, f.Runtime)
			if err != nil {
				return fmt.Errorf("insert unit %s #%d: %w", f.Title, idx, err)
			}
		}
	}

	updated := now.AddDate(0, 0, -f.StaleDays).Unix()
	res, err = tx.ExecContext(ctx, `
		INSERT INTO entry (user_id, media_id, status, position, started_at, updated_at, created_at)
		VALUES (?,?,?,?,?,?,?)`,
		userID, mediaID, f.Status, f.Position, now.AddDate(0, 0, -90).Unix(), updated, now.Unix())
	if err != nil {
		return fmt.Errorf("insert entry %s: %w", f.Title, err)
	}
	entryID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	if f.Position > 0 {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO progress (entry_id, from_pos, to_pos, at, source)
			VALUES (?,?,?,?,'backfill')`,
			entryID, 0, f.Position, updated)
		if err != nil {
			return fmt.Errorf("insert progress %s: %w", f.Title, err)
		}
	}

	return tx.Commit()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
