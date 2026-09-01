package domain

import (
	"strings"
	"testing"
)

func seasonsOf(counts ...int) []Unit {
	var us []Unit
	idx := 0
	for s, n := range counts {
		for e := 1; e <= n; e++ {
			idx++
			us = append(us, Unit{Idx: idx, Season: s + 1, Number: e, AirDate: "2020-01-01"})
		}
	}
	return us
}

func shape(blocks []Block) string {
	var parts []string
	for _, b := range blocks {
		switch b.Kind {
		case BlockCells:
			parts = append(parts, "cells:"+states(b.Cells))
		default:
			parts = append(parts, b.Kind)
		}
	}
	return strings.Join(parts, " ")
}

func TestTieredCollapsesEverySeasonButTheCurrent(t *testing.T) {
	us := seasonsOf(3, 3, 3)

	got := shape(BuildTiered(us, 4, "2026-08-27"))
	if want := "done cells:fna later"; got != want {
		t.Errorf("mid-show = %q, want %q", got, want)
	}
}

func TestTieredStartOfShow(t *testing.T) {
	got := shape(BuildTiered(seasonsOf(2, 2), 0, "2026-08-27"))
	if want := "cells:na later"; got != want {
		t.Errorf("nothing watched = %q, want %q", got, want)
	}
}

func TestTieredSingleSeasonStaysFlat(t *testing.T) {
	blocks := BuildTiered(seasonsOf(5), 2, "2026-08-27")
	if len(blocks) != 1 || blocks[0].Kind != BlockCells {
		t.Fatalf("single season should stay one cells block, got %s", shape(blocks))
	}
}

func TestTieredLaterSeasonKnowsWhetherItAired(t *testing.T) {
	us := seasonsOf(2, 2)
	us[2].AirDate = "2030-01-01"
	us[3].AirDate = "2030-01-08"

	blocks := BuildTiered(us, 1, "2026-08-27")
	if blocks[1].Aired {
		t.Error("a season entirely in the future must not read as aired")
	}

	us[2].AirDate = "2020-01-01"
	blocks = BuildTiered(us, 1, "2026-08-27")
	if !blocks[1].Aired {
		t.Error("a season with an aired episode must read as aired")
	}
}

func TestTieredHugeSeasonFallsBackToABar(t *testing.T) {
	blocks := BuildTiered(seasonsOf(MaxCells+1), 10, "2026-08-27")
	if blocks[0].Kind != BlockBar {
		t.Errorf("a season past %d episodes should be a bar, got %q", MaxCells, blocks[0].Kind)
	}
	if blocks[0].Percent == 0 {
		t.Error("bar fallback must still report progress")
	}
}

func TestTieredDenseFlagFollowsSeasonSize(t *testing.T) {
	small := BuildTiered(seasonsOf(10), 0, "2026-08-27")
	if small[0].Dense {
		t.Error("a ten-episode season should not be dense")
	}
	big := BuildTiered(seasonsOf(DenseCells+1), 0, "2026-08-27")
	if !big[0].Dense {
		t.Error("a season past the dense threshold should be dense")
	}
}

func TestTieredEmpty(t *testing.T) {
	if BuildTiered(nil, 0, "2026-08-27") != nil {
		t.Error("no units means no blocks")
	}
}

func TestTieredFinishedShowCollapsesEverySeason(t *testing.T) {
	got := shape(BuildTiered(seasonsOf(2, 2, 2), 6, "2026-08-27"))
	if want := "done done done"; got != want {
		t.Errorf("finished show = %q, want %q", got, want)
	}
}

func TestTieredOneEpisodeShortOfTheEndStaysExpanded(t *testing.T) {
	got := shape(BuildTiered(seasonsOf(2, 2), 3, "2026-08-27"))
	if want := "done cells:fn"; got != want {
		t.Errorf("almost finished = %q, want %q", got, want)
	}
}

func TestSeasonEndSkipsUnairedEpisodes(t *testing.T) {
	us := seasonsOf(10)
	for i := 6; i < 10; i++ {
		us[i].AirDate = "2030-01-01"
	}
	blocks := BuildTiered(us, 2, "2026-08-27")
	if got := CurrentSeasonEnd(blocks); got != 6 {
		t.Errorf("season end = %d, want 6 (episodes 7-10 have not aired)", got)
	}
}

func TestSeasonEndFullyAired(t *testing.T) {
	blocks := BuildTiered(seasonsOf(8), 3, "2026-08-27")
	if got := CurrentSeasonEnd(blocks); got != 8 {
		t.Errorf("season end = %d, want 8", got)
	}
}

func TestBuildVolumes(t *testing.T) {
	blocks := BuildVolumes(13, 7)
	if len(blocks) != 1 || blocks[0].Kind != BlockCells {
		t.Fatalf("got %+v, want one block of cells", blocks)
	}
	cells := blocks[0].Cells
	if len(cells) != 13 {
		t.Fatalf("got %d cells, want 13", len(cells))
	}

	want := map[int]string{
		1: StateWatched, 7: StateWatched,
		// A volume that exists but is unread is "out", never "not out yet":
		// there are no air dates to tell them apart, and every printed volume
		// is on a shelf somewhere.
		8: StateNext, 9: StateAired, 13: StateAired,
	}
	for idx, state := range want {
		if got := cells[idx-1].State; got != state {
			t.Errorf("volume %d is %q, want %q", idx, got, state)
		}
	}
	if cells[0].Hint != "Том 1" {
		t.Errorf("hint = %q, want \"Том 1\"", cells[0].Hint)
	}
}

func TestBuildVolumesFallsBackToABarWhenHuge(t *testing.T) {
	blocks := BuildVolumes(MaxCells+1, 10)
	if len(blocks) != 1 || blocks[0].Kind != BlockBar {
		t.Fatalf("got %+v, want a bar", blocks)
	}
	if blocks[0].Percent == 0 {
		t.Error("a bar with progress showed none")
	}
}

func TestBuildVolumesNeedsMoreThanOne(t *testing.T) {
	for _, total := range []int{0, 1, -3} {
		if b := BuildVolumes(total, 0); b != nil {
			t.Errorf("BuildVolumes(%d) = %+v, want nothing to draw", total, b)
		}
	}
}
