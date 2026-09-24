package domain

import "strconv"

// UnitWords is every form a unit is ever written in. One table, so the row,
// the panel, the palette and the toast cannot drift apart — they did, once:
// one map said «год», another «годин».
type UnitWords struct {
	One, Few, Many string
	// Beside a number on a button that is 32px wide: «+10 стор.».
	Short string
}

var Units = map[string]UnitWords{
	"episode": {"серія", "серії", "серій", "сер."},
	"page":    {"сторінка", "сторінки", "сторінок", "стор."},
	"hour":    {"година", "години", "годин", "год"},
	"chapter": {"розділ", "розділи", "розділів", "розд."},
	"lesson":  {"урок", "уроки", "уроків", "ур."},
	"minute":  {"хвилина", "хвилини", "хвилин", "хв"},
	"volume":  {"том", "томи", "томів", "т."},
}

// UnitMany is the genitive plural that follows a total («усього 86 серій»).
// Empty for "none" and anything unknown, so callers can print nothing.
func UnitMany(unit string) string { return Units[unit].Many }

// UnitCount names an arbitrary number of units: «3 серії», «21 сторінка».
func UnitCount(n int, unit string) string {
	u, ok := Units[unit]
	if !ok {
		return strconv.Itoa(n)
	}
	return Count(n, u.One, u.Few, u.Many)
}
