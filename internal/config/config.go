package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr      string
	DBPath    string
	Dev       bool
	Fixtures  bool
	Loc       *time.Location
	DayStart  int
	Password  string
	TMDBToken string
	BooksKey  string
	GamesKey  string
	CacheDir  string
}

func Load() (Config, error) {
	if err := LoadDotEnv(env("SOFAR_ENV_FILE", ".env")); err != nil {
		return Config{}, fmt.Errorf("read .env: %w", err)
	}

	cfg := Config{
		Addr:      env("SOFAR_ADDR", ":8099"),
		DBPath:    env("SOFAR_DB", "sofar.db"),
		Dev:       env("SOFAR_DEV", "") != "",
		DayStart:  4,
		Password:  env("SOFAR_PASSWORD", ""),
		TMDBToken: env("TMDB_TOKEN", ""),
		BooksKey:  env("GOOGLE_BOOKS_KEY", ""),
		GamesKey:  env("RAWG_KEY", ""),
		CacheDir:  env("SOFAR_CACHE", "cache"),
	}

	// Fixtures follow dev mode unless asked otherwise: an empty database is
	// meant to stay empty once you have your own titles in it.
	cfg.Fixtures = cfg.Dev && !falsy(env("SOFAR_FIXTURES", ""))

	tz := env("SOFAR_TZ", "Europe/Kyiv")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Config{}, fmt.Errorf("SOFAR_TZ %q: %w", tz, err)
	}
	cfg.Loc = loc

	if raw := env("SOFAR_DAY_START", ""); raw != "" {
		h, err := strconv.Atoi(raw)
		if err != nil || h < 0 || h > 23 {
			return Config{}, fmt.Errorf("SOFAR_DAY_START %q: want an hour 0-23", raw)
		}
		cfg.DayStart = h
	}

	if cfg.Password == "" && !cfg.Dev && !loopback(cfg.Addr) {
		return Config{}, fmt.Errorf(
			"SOFAR_PASSWORD is empty and %s is reachable from outside — set a password or listen on 127.0.0.1",
			cfg.Addr)
	}

	return cfg, nil
}

// A listener that only answers on the loopback interface is already private,
// which is what makes a password optional in development and nowhere else.
func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return false
}

// Anything before DayStart counts as the previous day: an episode at 01:30
// belongs to the evening before.
func (c Config) Day(t time.Time) time.Time {
	t = t.In(c.Loc).Add(-time.Duration(c.DayStart) * time.Hour)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, c.Loc)
}

func falsy(v string) bool {
	switch v {
	case "0", "false", "no", "off":
		return true
	}
	return false
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}
