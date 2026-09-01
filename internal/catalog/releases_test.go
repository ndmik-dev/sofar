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

func TestSeriesOf(t *testing.T) {
	cases := map[string]string{
		"Берсерк. Том 3":               "Берсерк",
		"Given, Том 3":                 "Given",
		"Проводжальниця Фрірен. Том 1": "Проводжальниця Фрірен",
		"Токійські месники. Том 14":    "Токійські месники",
		"Клинок та Бастард. Том 1":     "Клинок та Бастард",
		"Дрон-Сіті Том 1 Робовечірка":  "Дрон-Сіті",
	}
	for title, want := range cases {
		if got := seriesOf(title); got != want {
			t.Errorf("seriesOf(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestLiveVolumes(t *testing.T) {
	r := NewReleases(t.TempDir())
	got, err := r.Volumes(context.Background(), "Берсерк")
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
	// "Ван Піс" is not published in Ukrainian, but the shop's fuzzy search
	// answers with whatever it likes. A watch must not adopt those.
	got, err := r.Volumes(context.Background(), "Ван Піс")
	if err != nil {
		t.Skipf("shop unreachable: %v", err)
	}
	for _, v := range got {
		if !contains(v.Title, "Ван Піс") {
			t.Errorf("unrelated title kept: %q", v.Title)
		}
	}
}

func TestLiveSearchMangaGroupsVolumes(t *testing.T) {
	r := NewReleases(t.TempDir())
	got, err := r.SearchManga(context.Background(), "Фрірен", 5)
	if err != nil {
		t.Skipf("shop unreachable: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no series for a title the shop certainly carries")
	}
	for _, f := range got {
		t.Logf("%s · до тому %d", f.Title, f.Total)
		if f.Kind != "manga" || f.Title == "" || f.Total == 0 {
			t.Errorf("incomplete series: %+v", f)
		}
		// The series, not one of its volumes.
		if volumeOf(f.Title) != 0 {
			t.Errorf("a volume leaked in as a series: %q", f.Title)
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
