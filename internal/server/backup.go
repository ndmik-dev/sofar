package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ndmik-dev/sofar/internal/store"
)

// A backup is only a backup once a copy exists somewhere the volume does not.
// The nightly job makes the file; these two handlers are how it leaves the box
// — by hand, once a month, from the settings page. No cron, no bucket, no
// credentials to rotate: the person who owns the data downloads it.

type backupView struct {
	Has   bool
	Last  string
	Count int
	Size  string
}

func (s *Server) backupView() (backupView, error) {
	files, err := store.ListBackups(s.cfg.BackupDir)
	if err != nil {
		return backupView{}, err
	}
	if len(files) == 0 {
		return backupView{}, nil
	}
	newest := files[len(files)-1]
	return backupView{
		Has:   true,
		Last:  fmt.Sprintf("%d %s", newest.At.Day(), monthNames[newest.At.Month()-1]),
		Count: len(files),
		Size:  fmt.Sprintf("%.1f МБ", float64(newest.Size)/1024/1024),
	}, nil
}

func (s *Server) handleBackupNow(w http.ResponseWriter, r *http.Request) {
	now := time.Now().In(s.cfg.Loc)
	if _, err := s.store.Backup(r.Context(), s.cfg.BackupDir, now); err != nil {
		s.fail(w, r, err)
		return
	}
	view, err := s.backupView()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderFragment(w, r, "backup-row", view)
}

// handleBackupDownload serves the newest copy. Only a file the listing knows
// about is ever served, so the path never comes from the request.
func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	files, err := store.ListBackups(s.cfg.BackupDir)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if len(files) == 0 {
		http.Error(w, "Копій ще немає — перша зʼявиться вночі", http.StatusNotFound)
		return
	}
	newest := files[len(files)-1]
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+newest.Name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, newest.Path)
}
