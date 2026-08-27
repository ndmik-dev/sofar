package server

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

var pages = []string{"active.html"}

var partials = []string{"row.html"}

const devTemplateDir = "internal/server/templates"

func loadTemplates(dev bool) (map[string]*template.Template, error) {
	var tfs fs.FS
	if dev {
		tfs = os.DirFS(devTemplateDir)
	} else {
		sub, err := fs.Sub(assetsFS, "templates")
		if err != nil {
			return nil, fmt.Errorf("templates sub-fs: %w", err)
		}
		tfs = sub
	}

	out := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		files := append([]string{"layout.html", page}, partials...)
		t, err := template.New("layout.html").ParseFS(tfs, files...)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", page, err)
		}
		out[page] = t
	}
	return out, nil
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, page string, data any) {
	tmpls := s.templates
	if s.cfg.Dev {
		fresh, err := loadTemplates(true)
		if err != nil {
			s.fail(w, r, fmt.Errorf("reload templates: %w", err))
			return
		}
		tmpls = fresh
	}

	t, ok := tmpls[page]
	if !ok {
		s.fail(w, r, fmt.Errorf("unknown page %q", page))
		return
	}

	// Buffer first: a template error halfway through must not leave a partial
	// body behind an already-sent 200.
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		s.fail(w, r, fmt.Errorf("execute %s: %w", page, err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = buf.WriteTo(w)
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("request failed", "path", r.URL.Path, "err", err)
	http.Error(w, "Щось пішло не так. Спробуй ще раз.", http.StatusInternalServerError)
}

func devTemplatesPresent() bool {
	_, err := os.Stat(filepath.Join(devTemplateDir, "layout.html"))
	return err == nil
}
