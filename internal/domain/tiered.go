package domain

const (
	// Thresholds now apply to a single season rather than a whole show, which
	// is the only place they ever made sense.
	DenseCells = 40
	MaxCells   = 130
)

const (
	BlockDone  = "done"
	BlockLater = "later"
	BlockCells = "cells"
	BlockBar   = "bar"
)

type Block struct {
	Kind    string
	Season  int
	Count   int
	Cells   []Cell
	Dense   bool
	Aired   bool
	Percent int
}

// BuildTiered collapses every season except the one you are in. Seasons before
// it are finished by definition — position is a single running number — so a
// filled block says everything there is to say about them.
func BuildTiered(units []Unit, position int, today string) []Block {
	if len(units) == 0 {
		return nil
	}

	seasons := groupSeasons(units)
	if len(seasons) == 1 {
		return []Block{cellsBlock(seasons[0], position, today)}
	}

	current := currentSeason(seasons, position)
	blocks := make([]Block, 0, len(seasons))

	for i, s := range seasons {
		switch {
		case i == current:
			blocks = append(blocks, cellsBlock(s, position, today))
		case i < current:
			blocks = append(blocks, Block{
				Kind: BlockDone, Season: s[0].Season, Count: len(s), Percent: 100,
			})
		default:
			blocks = append(blocks, Block{
				Kind:   BlockLater,
				Season: s[0].Season,
				Count:  len(s),
				Aired:  anyAired(s, today),
			})
		}
	}
	return blocks
}

func cellsBlock(season []Unit, position int, today string) Block {
	b := Block{
		Kind:   BlockCells,
		Season: season[0].Season,
		Count:  len(season),
		Dense:  len(season) > DenseCells,
	}
	if len(season) > MaxCells {
		b.Kind = BlockBar
		b.Percent = percentWatched(season, position)
		return b
	}
	b.Cells = BuildTrack(season, position, today)
	return b
}

func groupSeasons(units []Unit) [][]Unit {
	var out [][]Unit
	var cur []Unit
	for i, u := range units {
		if i > 0 && u.Season != units[i-1].Season {
			out = append(out, cur)
			cur = nil
		}
		cur = append(cur, u)
	}
	return append(out, cur)
}

// The season you are in is the one holding the next unwatched unit; once
// everything is watched it is the last one.
func currentSeason(seasons [][]Unit, position int) int {
	next := position + 1
	for i, s := range seasons {
		if next <= s[len(s)-1].Idx {
			return i
		}
	}
	return len(seasons) - 1
}

func anyAired(season []Unit, today string) bool {
	for _, u := range season {
		if u.aired(today) {
			return true
		}
	}
	return false
}

func percentWatched(season []Unit, position int) int {
	if len(season) == 0 {
		return 0
	}
	var done int
	for _, u := range season {
		if u.Idx <= position {
			done++
		}
	}
	return done * 100 / len(season)
}
