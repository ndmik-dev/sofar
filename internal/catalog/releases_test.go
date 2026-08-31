package catalog

import (
	"context"
	"testing"
)

func TestVolumeOf(t *testing.T) {
	cases := map[string]int{
		"Given, Том 3": 3,
		"Дрон-Сіті Том 1 Робовечірка":      1,
		"Ранобе «Мій щасливий шлюб» Том 1": 1,
		"Tomie Vol. 2": 2,
		"Tomie vol1":   1,
		"Шлях домогосподаря том 12": 12,
		"Артбук без тому":           0,
		"":                          0,
	}
	for title, want := range cases {
		if got := volumeOf(title); got != want {
			t.Errorf("volumeOf(%q) = %d, want %d", title, got, want)
		}
	}
}

func TestLiveVolumes(t *testing.T) {
	r := NewReleases(t.TempDir())
	got, err := r.Volumes(context.Background(), "Given")
	if err != nil {
		t.Skipf("publisher unreachable: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no volumes for a series known to have several")
	}
	for _, v := range got {
		t.Logf("том %d · %s · %s", v.Volume, v.At.Format("2006-01-02"), v.Title)
		if v.Volume == 0 || v.Title == "" || v.URL == "" {
			t.Errorf("incomplete release: %+v", v)
		}
	}
}

func TestLiveVolumesIgnoresUnrelatedHits(t *testing.T) {
	r := NewReleases(t.TempDir())
	got, err := r.Volumes(context.Background(), "Given")
	if err != nil {
		t.Skipf("publisher unreachable: %v", err)
	}
	// The shop's search is fuzzy; a watch must not follow the wrong series.
	for _, v := range got {
		if !contains(v.Title, "Given") {
			t.Errorf("unrelated title kept: %q", v.Title)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
