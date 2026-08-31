package store

import (
	"context"
	"testing"
	"time"
)

func TestWatchLifecycle(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	id := seedEntry(t, st, 10)
	now := time.Unix(1_800_000_000, 0)

	if _, on, err := st.WatchFor(ctx, id); err != nil || on {
		t.Fatalf("a fresh entry should not be watched: on=%v err=%v", on, err)
	}

	if err := st.SetWatch(ctx, id, "nashaidea", "Given", now); err != nil {
		t.Fatal(err)
	}
	w, on, err := st.WatchFor(ctx, id)
	if err != nil || !on || w.Query != "Given" {
		t.Fatalf("watch not stored: %+v on=%v err=%v", w, on, err)
	}

	due, err := st.DueWatches(ctx)
	if err != nil || len(due) != 1 || due[0].EntryID != id {
		t.Fatalf("DueWatches = %+v, err=%v", due, err)
	}

	// Nothing new: only the check time moves, so "new" keeps meaning new.
	if err := st.MarkChecked(ctx, id, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if w, _, _ := st.WatchFor(ctx, id); w.FoundAt != 0 || w.CheckedAt == 0 {
		t.Errorf("a plain check should not count as a find: %+v", w)
	}

	if err := st.RecordRelease(ctx, id, 3, "Given, Том 3", "https://example.org/3", now); err != nil {
		t.Fatal(err)
	}
	w, _, _ = st.WatchFor(ctx, id)
	if w.LastVol != 3 || w.LastTitle != "Given, Том 3" || w.FoundAt == 0 {
		t.Errorf("release not recorded: %+v", w)
	}

	// Setting it again keeps the volume already found: re-watching is not
	// forgetting.
	if err := st.SetWatch(ctx, id, "nashaidea", "Given ранобе", now); err != nil {
		t.Fatal(err)
	}
	if w, _, _ := st.WatchFor(ctx, id); w.LastVol != 3 || w.Query != "Given ранобе" {
		t.Errorf("re-watch lost state: %+v", w)
	}

	if err := st.DropWatch(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, on, _ := st.WatchFor(ctx, id); on {
		t.Error("watch survived being dropped")
	}
}

func TestDueWatchesSkipsDeletedEntries(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	id := seedEntry(t, st, 10)
	now := time.Unix(1_800_000_000, 0)

	if err := st.SetWatch(ctx, id, "nashaidea", "Given", now); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SoftDelete(ctx, id, now); err != nil {
		t.Fatal(err)
	}
	due, err := st.DueWatches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Errorf("a deleted entry is still being polled: %+v", due)
	}
}
