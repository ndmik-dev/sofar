package server

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/ndmik-dev/sofar/internal/catalog"
	"github.com/ndmik-dev/sofar/internal/config"
	"github.com/ndmik-dev/sofar/internal/store"
)

//go:embed templates static
var assetsFS embed.FS

type Server struct {
	cfg       config.Config
	store     *store.Store
	catalog   *catalog.Catalog
	log       *slog.Logger
	templates map[string]*template.Template
	fragments *template.Template
	mux       *http.ServeMux
	attempts  *attempts
	handler   http.Handler
}

func New(cfg config.Config, st *store.Store, cat *catalog.Catalog, log *slog.Logger) (*Server, error) {
	if cfg.Dev && !devTemplatesPresent() {
		return nil, fmt.Errorf("SOFAR_DEV is set but %s is missing — run from the repo root", devTemplateDir)
	}

	tmpls, err := loadTemplates(cfg.Dev)
	if err != nil {
		return nil, err
	}
	frags, err := loadFragments(cfg.Dev)
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:       cfg,
		store:     st,
		catalog:   cat,
		log:       log,
		templates: tmpls,
		fragments: frags,
		mux:       http.NewServeMux(),
		attempts:  newAttempts(),
	}
	if err := s.routes(); err != nil {
		return nil, err
	}
	s.handler = s.guard(s.mux)
	return s, nil
}

func (s *Server) routes() error {
	static, err := s.staticHandler()
	if err != nil {
		return err
	}
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", static))

	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /login", s.handleLogin)
	s.mux.HandleFunc("POST /login", s.handleLoginPost)
	s.mux.HandleFunc("POST /logout", s.handleLogout)
	s.mux.HandleFunc("GET /{$}", s.handleRoot)
	s.mux.HandleFunc("GET /active", s.handleActive)
	s.mux.HandleFunc("GET /backlog", s.handleBacklog)
	s.mux.HandleFunc("GET /done", s.handleDone)
	s.mux.HandleFunc("GET /dropped", s.handleDropped)
	s.mux.HandleFunc("GET /year", s.handleYear)
	s.mux.HandleFunc("GET /year/{y}", s.handleYear)
	s.mux.HandleFunc("GET /export", s.handleExport)
	s.mux.HandleFunc("GET /import", s.handleImport)
	s.mux.HandleFunc("POST /import", s.handleImportRun)
	s.mux.HandleFunc("GET /settings", s.handleSettings)
	s.mux.HandleFunc("POST /settings/{kind}", s.handleSetSetting)
	s.mux.HandleFunc("POST /entry/{id}/advance", s.handleAdvance)
	s.mux.HandleFunc("POST /entry/{id}/undo", s.handleUndo)
	s.mux.HandleFunc("POST /entry/{id}/status", s.handleStatus)
	s.mux.HandleFunc("GET /entry/{id}/panel", s.handlePanel)
	s.mux.HandleFunc("POST /entry/{id}/note", s.handleNote)
	s.mux.HandleFunc("POST /entry/{id}/edit", s.handleEdit)
	s.mux.HandleFunc("POST /entry/{id}/link", s.handleAddLink)
	s.mux.HandleFunc("POST /entry/{id}/link/{link}/delete", s.handleDeleteLink)
	s.mux.HandleFunc("POST /entry/{id}/rating", s.handleRating)
	s.mux.HandleFunc("POST /entry/{id}/delete", s.handleDelete)
	s.mux.HandleFunc("POST /entry/{id}/restore", s.handleRestore)
	s.mux.HandleFunc("GET /search", s.handleSearch)
	s.mux.HandleFunc("POST /add", s.handleAdd)
	s.mux.HandleFunc("GET /manual", s.handleManualForm)
	s.mux.HandleFunc("POST /add/manual", s.handleAddManual)
	return nil
}

func (s *Server) staticHandler() (http.Handler, error) {
	if s.cfg.Dev {
		// Heuristic caching serves a stale app.css or keys.js long after the
		// file changed, which reads exactly like the bug you just fixed.
		fsrv := http.FileServer(http.Dir("internal/server/static"))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			fsrv.ServeHTTP(w, r)
		}), nil
	}
	sub, err := fs.Sub(assetsFS, "static")
	if err != nil {
		return nil, fmt.Errorf("static sub-fs: %w", err)
	}
	return http.FileServer(http.FS(sub)), nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}
