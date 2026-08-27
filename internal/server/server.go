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
	}
	if err := s.routes(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) routes() error {
	static, err := s.staticHandler()
	if err != nil {
		return err
	}
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", static))

	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /{$}", s.handleRoot)
	s.mux.HandleFunc("GET /active", s.handleActive)
	s.mux.HandleFunc("POST /entry/{id}/advance", s.handleAdvance)
	s.mux.HandleFunc("POST /entry/{id}/undo", s.handleUndo)
	s.mux.HandleFunc("POST /entry/{id}/status", s.handleStatus)
	s.mux.HandleFunc("POST /entry/{id}/delete", s.handleDelete)
	s.mux.HandleFunc("POST /entry/{id}/restore", s.handleRestore)
	s.mux.HandleFunc("GET /search", s.handleSearch)
	s.mux.HandleFunc("POST /add", s.handleAdd)
	s.mux.HandleFunc("POST /add/manual", s.handleAddManual)
	return nil
}

func (s *Server) staticHandler() (http.Handler, error) {
	if s.cfg.Dev {
		return http.FileServer(http.Dir("internal/server/static")), nil
	}
	sub, err := fs.Sub(assetsFS, "static")
	if err != nil {
		return nil, fmt.Errorf("static sub-fs: %w", err)
	}
	return http.FileServer(http.FS(sub)), nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
