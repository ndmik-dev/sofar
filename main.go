package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "time/tzdata"

	"github.com/ndmik-dev/sofar/internal/catalog"
	"github.com/ndmik-dev/sofar/internal/config"
	"github.com/ndmik-dev/sofar/internal/server"
	"github.com/ndmik-dev/sofar/internal/store"
	"github.com/ndmik-dev/sofar/internal/tmdb"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := st.Migrate(ctx); err != nil {
		return err
	}

	if cfg.Password == "" {
		log.Warn("no SOFAR_PASSWORD — anyone who can reach this address has full access")
	}

	if cfg.Fixtures {
		n, err := st.SeedFixtures(ctx, 1, time.Now().In(cfg.Loc))
		if err != nil {
			return err
		}
		if n > 0 {
			log.Info("seeded fixtures into an empty database — SOFAR_FIXTURES=0 to keep it empty", "entries", n)
		}
	}

	cat := catalog.New(
		tmdb.New(cfg.TMDBToken, cfg.CacheDir),
		catalog.NewBooks(cfg.BooksKey, cfg.CacheDir),
		catalog.NewGames(cfg.GamesKey, cfg.CacheDir),
		catalog.NewPodcasts(cfg.CacheDir),
		st,
	)
	if !cat.Enabled() {
		log.Warn("TMDB_TOKEN is not set — catalog search is disabled")
	}

	srv, err := server.New(cfg, st, cat, log)
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return fmt.Errorf("%s is taken — free it or run with SOFAR_ADDR=:PORT (see who has it: lsof -nP -iTCP%s -sTCP:LISTEN)", cfg.Addr, cfg.Addr)
		}
		return err
	}
	log.Info("listening", "addr", ln.Addr().String(), "db", cfg.DBPath, "dev", cfg.Dev)

	errc := make(chan error, 1)
	go func() {
		if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}
