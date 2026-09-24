package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Backups are named by the day they were taken, so a second run on the same
// day replaces the first instead of piling up.
const backupPrefix = "sofar-"

type BackupFile struct {
	Path string
	Name string
	Size int64
	At   time.Time
}

// Backup writes a consistent copy of the whole database into dir. VACUUM INTO
// works through the same connection that serves requests, so it needs no lock,
// no second process and no sqlite3 binary — none of which a distroless image
// has. The copy is a plain single file: WAL pages are folded in.
func (s *Store) Backup(ctx context.Context, dir string, now time.Time) (BackupFile, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return BackupFile{}, fmt.Errorf("backup dir: %w", err)
	}
	name := backupPrefix + now.Format("2006-01-02") + ".db"
	path := filepath.Join(dir, name)
	// VACUUM INTO refuses to overwrite, and a half-written file from a crashed
	// run must not survive as a "backup".
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return BackupFile{}, fmt.Errorf("replace old backup: %w", err)
	}
	if _, err := s.DB.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		os.Remove(path)
		return BackupFile{}, fmt.Errorf("vacuum into: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return BackupFile{}, err
	}
	return BackupFile{Path: path, Name: name, Size: info.Size(), At: now}, nil
}

// PruneBackups keeps the newest `keep` copies. Names sort by date, so the
// oldest come first.
func PruneBackups(dir string, keep int) (int, error) {
	files, err := ListBackups(dir)
	if err != nil {
		return 0, err
	}
	var removed int
	for len(files) > keep {
		if err := os.Remove(files[0].Path); err != nil {
			return removed, fmt.Errorf("prune backup: %w", err)
		}
		files = files[1:]
		removed++
	}
	return removed, nil
}

// ListBackups returns the copies in dir, oldest first. A missing dir is an
// empty list: nothing has run yet.
func ListBackups(dir string) ([]BackupFile, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	var out []BackupFile
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, backupPrefix) || !strings.HasSuffix(name, ".db") {
			continue
		}
		day := strings.TrimSuffix(strings.TrimPrefix(name, backupPrefix), ".db")
		at, err := time.Parse("2006-01-02", day)
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupFile{Path: filepath.Join(dir, name), Name: name, Size: info.Size(), At: at})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
