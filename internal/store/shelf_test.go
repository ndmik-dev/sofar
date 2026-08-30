package store

import "testing"

func TestShelfMatchingFoldsCyrillic(t *testing.T) {
	// SQLite's LOWER and LIKE fold ASCII only, which is why the matching moved
	// into Go. These are the cases that used to find nothing.
	cases := []struct {
		needle, title string
		want          bool
	}{
		{"дюна", "Дюна: Частина друга", true},
		{"ДЮНА", "Дюна: Частина друга", true},
		{"части", "Дюна: Частина друга", true},
		{"сопрано", "Клан Сопрано", true},
		{"dune", "Dune: Part Two", true},
		{"DUNE", "Dune: Part Two", true},
		{"зовсім інше", "Дюна: Частина друга", false},
	}
	for _, c := range cases {
		if got := shelfMatches(c.needle, c.title, ""); got != c.want {
			t.Errorf("%q in %q = %v, want %v", c.needle, c.title, got, c.want)
		}
	}
}

func TestShelfMatchesTheOriginalTitleToo(t *testing.T) {
	if !shelfMatches("sopranos", "Клан Сопрано", "The Sopranos") {
		t.Error("the original title was not searched")
	}
}
