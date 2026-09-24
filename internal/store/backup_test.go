package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupIsACompleteCopy(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	id := seedEntry(t, st, 10)

	dir := filepath.Join(t.TempDir(), "backups")
	now := time.Date(2026, 9, 24, 4, 20, 0, 0, time.UTC)
	f, err := st.Backup(ctx, dir, now)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "sofar-2026-09-24.db" || f.Size == 0 {
		t.Fatalf("got %+v", f)
	}

	copyDB, err := Open(f.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	if _, err := copyDB.GetEntry(ctx, id); err != nil {
		t.Fatalf("entry missing from the copy: %v", err)
	}

	// Same day again must replace, not fail on the existing file.
	if _, err := st.Backup(ctx, dir, now); err != nil {
		t.Fatal(err)
	}
}

func TestPruneKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"2026-09-01", "2026-09-02", "2026-09-03", "junk"} {
		if err := os.WriteFile(filepath.Join(dir, "sofar-"+d+".db"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := PruneBackups(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed %d, want 1", removed)
	}
	left, _ := ListBackups(dir)
	if len(left) != 2 || left[0].Name != "sofar-2026-09-02.db" || left[1].Name != "sofar-2026-09-03.db" {
		t.Fatalf("left %+v", left)
	}
	if _, err := os.Stat(filepath.Join(dir, "sofar-junk.db")); err != nil {
		t.Fatal("a file that is not a dated backup must be left alone")
	}
}
