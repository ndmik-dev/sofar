package server

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ndmik-dev/sofar/internal/domain"
)

var pages = []string{"active.html", "backlog.html", "done.html", "year.html", "settings.html", "import.html"}

var partials = []string{"row.html", "fragments.html", "nav.html", "panel.html"}

// The login page is the one thing rendered before there is a session, so it
// carries no sidebar and no layout.
var standalone = []string{"login.html"}

const devTemplateDir = "internal/server/templates"

var funcs = template.FuncMap{
	"plural": domain.Plural,
	"inc":    func(i int) int { return i + 1 },
}

func templateFS(dev bool) (fs.FS, error) {
	if dev {
		return os.DirFS(devTemplateDir), nil
	}
	sub, err := fs.Sub(assetsFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("templates sub-fs: %w", err)
	}
	return sub, nil
}

func loadTemplates(dev bool) (map[string]*template.Template, error) {
	tfs, err := templateFS(dev)
	if err != nil {
		return nil, err
	}

	out := make(map[string]*template.Template, len(pages)+len(standalone))
	for _, page := range standalone {
		t, err := template.New(page).Funcs(funcs).ParseFS(tfs, page)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", page, err)
		}
		out[page] = t
	}
	for _, page := range pages {
		files := append([]string{"layout.html", page}, partials...)
		t, err := template.New("layout.html").Funcs(funcs).ParseFS(tfs, files...)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", page, err)
		}
		out[page] = t
	}
	return out, nil
}

func loadFragments(dev bool) (*template.Template, error) {
	tfs, err := templateFS(dev)
	if err != nil {
		return nil, err
	}
	t, err := template.New("fragments").Funcs(funcs).ParseFS(tfs, partials...)
	if err != nil {
		return nil, fmt.Errorf("parse fragments: %w", err)
	}
	return t, nil
}

func (s *Server) renderFragment(w http.ResponseWriter, r *http.Request, name string, data any) {
	t := s.fragments
	if s.cfg.Dev {
		fresh, err := loadFragments(true)
		if err != nil {
			s.fail(w, r, fmt.Errorf("reload fragments: %w", err))
			return
		}
		t = fresh
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		s.fail(w, r, fmt.Errorf("execute %s: %w", name, err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = buf.WriteTo(w)
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
