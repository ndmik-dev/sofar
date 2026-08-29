package server

import (
	"strings"
	"testing"

	"github.com/ndmik-dev/sofar/internal/tmdb"
)

func TestParseImportTakesWhatPeoplePaste(t *testing.T) {
	lines, overflow := parseImport(`
Клан Сопрано
Breaking Bad (2008)
Друзі | 96
"The Wire",2002
# коментар

`)
	if overflow != 0 {
		t.Fatalf("overflow = %d, want 0", overflow)
	}
	want := []importLine{
		{Raw: "Клан Сопрано", Query: "Клан Сопрано"},
		{Raw: "Breaking Bad (2008)", Query: "Breaking Bad", Year: 2008},
		{Raw: "Друзі", Query: "Друзі", Position: 96},
		{Raw: "The Wire", Query: "The Wire"},
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %+v", len(lines), len(want), lines)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d = %+v, want %+v", i, lines[i], w)
		}
	}
}

func TestParseImportCapsTheBatch(t *testing.T) {
	lines, overflow := parseImport(strings.Repeat("Дюна\n", importLimit+7))
	if len(lines) != importLimit {
		t.Errorf("kept %d lines, want %d", len(lines), importLimit)
	}
	if overflow != 7 {
		t.Errorf("overflow = %d, want 7", overflow)
	}
}

func TestPickMatchPrefersTheExactTitle(t *testing.T) {
	found := []tmdb.Result{
		{Title: "Друзі-товариші", Year: 1951},
		{Title: "Друзі", Year: 1994},
		{Title: "Друзі назавжди", Year: 2015},
	}
	if got := pickMatch(found, importLine{Query: "Друзі"}); got.Year != 1994 {
		t.Errorf("exact title lost to popularity: %+v", got)
	}
}

func TestPickMatchPrefersTheStatedYear(t *testing.T) {
	found := []tmdb.Result{
		{Title: "Дюна", Year: 1984},
		{Title: "Дюна", Year: 2021},
	}
	if got := pickMatch(found, importLine{Query: "Дюна", Year: 2021}); got.Year != 2021 {
		t.Errorf("year hint ignored: %+v", got)
	}
}

func TestPickMatchMatchesTheOriginalTitle(t *testing.T) {
	found := []tmdb.Result{
		{Title: "Щось інше", Year: 2000},
		{Title: "Пуститися берега", Original: "Breaking Bad", Year: 2008},
	}
	if got := pickMatch(found, importLine{Query: "breaking bad"}); got.Year != 2008 {
		t.Errorf("original title ignored: %+v", got)
	}
}

func TestPickMatchFallsBackToTheFirstHit(t *testing.T) {
	found := []tmdb.Result{{Title: "Перший", Year: 1999}, {Title: "Другий", Year: 2000}}
	if got := pickMatch(found, importLine{Query: "нічого схожого"}); got.Title != "Перший" {
		t.Errorf("fallback = %+v, want the first hit", got)
	}
}
