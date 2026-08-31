package tmdb

import "testing"

func TestKindFor(t *testing.T) {
	const animation, drama = 16, 18

	cases := []struct {
		name      string
		mediaType string
		genres    []int
		countries []string
		lang      string
		want      string
	}{
		{"live-action series", "tv", []int{drama}, []string{"US"}, "en", "show"},
		{"live-action film", "movie", []int{drama}, nil, "en", "movie"},
		{"japanese series", "tv", []int{animation}, []string{"JP"}, "ja", "anime"},
		// The case that sent "а: стометровка" to a book form: an animated
		// Japanese feature used to come back as a plain film, so the anime
		// filter threw the only correct hit away.
		{"japanese animated film", "movie", []int{animation, drama}, nil, "ja", "anime"},
		{"western animated film", "movie", []int{animation}, nil, "en", "movie"},
		{"western animated series", "tv", []int{animation}, []string{"US"}, "en", "show"},
		// Movies carry no origin_country, series sometimes carry no language.
		{"japanese series by country alone", "tv", []int{animation}, []string{"JP"}, "", "anime"},
		{"japanese film by language alone", "movie", []int{animation}, nil, "JA", "anime"},
		{"japanese live action stays a film", "movie", []int{drama}, nil, "ja", "movie"},
	}
	for _, c := range cases {
		if got := kindFor(c.mediaType, c.genres, c.countries, c.lang); got != c.want {
			t.Errorf("%s: kindFor = %q, want %q", c.name, got, c.want)
		}
	}
}
