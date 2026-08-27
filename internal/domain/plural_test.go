package domain

import "testing"

func TestPlural(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{
		{1, "1 серія"}, {2, "2 серії"}, {4, "4 серії"}, {5, "5 серій"},
		{11, "11 серій"}, {12, "12 серій"}, {14, "14 серій"},
		{21, "21 серія"}, {22, "22 серії"}, {25, "25 серій"},
		{62, "62 серії"}, {101, "101 серія"}, {111, "111 серій"},
		{0, "0 серій"},
	} {
		if got := Count(tc.n, "серія", "серії", "серій"); got != tc.want {
			t.Errorf("Count(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}
