package catalog

import (
	"strconv"
	"strings"
)

// Found is what any non-TMDB catalog can tell us. Total is a suggestion, never
// a fact: your edition's page count is the one that matters, so it lands in an
// editable field rather than straight in the database.
type Found struct {
	Source   string
	ExtID    string
	Kind     string
	Title    string
	Subtitle string
	Year     int
	Total    int
	Cover    string
}

func yearOf(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}

func joinMax(vals []string, n int) string {
	if len(vals) > n {
		vals = vals[:n]
	}
	return strings.Join(vals, " · ")
}
