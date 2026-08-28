package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}

func seedEntry(t *testing.T, st *Store, total int) int64 {
	t.Helper()
	now := time.Now()
	id, err := st.UpsertMedia(context.Background(), MediaInput{
		Kind: "show", Source: "manual", Title: "Test", Unit: "episode",
		TotalUnits: sql.NullInt64{Int64: int64(total), Valid: true},
	}, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	entryID, _, err := st.AddEntry(context.Background(), 1, id, "active", 0, now)
	if err != nil {
		t.Fatal(err)
	}
	return entryID
}

// withDeadline fails loudly instead of hanging the suite: these calls used to
// block forever, because a query issued inside an open transaction waits for
// the single pooled connection that the transaction itself is holding.
func withDeadline(t *testing.T, name string, fn func(context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- fn(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	case <-time.After(4 * time.Second):
		t.Fatalf("%s deadlocked", name)
	}
}

func TestAdvanceWithoutChangeDoesNotDeadlock(t *testing.T) {
	st := testStore(t)
	id := seedEntry(t, st, 10)
	now := time.Now()

	withDeadline(t, "advance by zero", func(ctx context.Context) error {
		_, err := st.Advance(ctx, id, 0, nil, now)
		return err
	})

	zero := 0
	withDeadline(t, "advance to the current position", func(ctx context.Context) error {
		_, err := st.Advance(ctx, id, 0, &zero, now)
		return err
	})
}

func TestUndoWithNothingToUndoDoesNotDeadlock(t *testing.T) {
	st := testStore(t)
	id := seedEntry(t, st, 10)

	withDeadline(t, "undo on an untouched entry", func(ctx context.Context) error {
		move, err := st.Undo(ctx, id, time.Now())
		if err == nil && move.Changed {
			t.Error("undo with no history must not report a change")
		}
		return err
	})
}

func TestRepeatedUndoStopsAtTheStartingPosition(t *testing.T) {
	st := testStore(t)
	now := time.Now()
	mediaID, err := st.UpsertMedia(context.Background(), MediaInput{
		Kind: "show", Source: "manual", Title: "Test", Unit: "episode",
		TotalUnits: sql.NullInt64{Int64: 62, Valid: true},
	}, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	// Position 22 arrives as setup, the way the "where did you stop" step
	// records it, then two watched episodes on top.
	id, _, err := st.AddEntry(context.Background(), 1, mediaID, "active", 22, now)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := st.Advance(context.Background(), id, 1, nil, now); err != nil {
			t.Fatal(err)
		}
	}

	for i := range 5 {
		withDeadline(t, "undo round", func(ctx context.Context) error {
			_, err := st.Undo(ctx, id, now)
			return err
		})
		_ = i
	}

	e, err := st.GetEntry(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if e.Position != 22 {
		t.Errorf("position = %d after undoing everything, want 22: setup is not an action", e.Position)
	}
}
