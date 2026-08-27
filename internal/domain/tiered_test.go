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
