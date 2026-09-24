package server

import "github.com/ndmik-dev/sofar/internal/domain"

// kindDef is everything the server knows about one kind. Adding a kind is one
// entry here plus a --k-<kind> colour in app.css; the test beside this file
// checks the colour exists.
type kindDef struct {
	Kind  string
	Label string // one of them: «серіал»
	Nav   string // the sidebar: «Серіали»
	Unit  string // what progress is counted in
	Steps []int  // what a press can add; the first is the default
	// The palette prefixes that pick this kind: «с:», «s:».
	Prefixes []string
	// No catalog behind it, so the palette goes straight to the manual form.
	Manual bool
	// A catalog exists for it, so the form can say "not found" honestly.
	Catalog bool
	// The second line of a manual entry: an author, a platform, an original
	// title.
	SubtitleLabel, SubtitleHint string
}

// kinds is in sidebar order.
var kinds = []kindDef{
	{Kind: "show", Label: "серіал", Nav: "Серіали", Unit: "episode", Steps: []int{1, 2},
		Prefixes: []string{"с", "s"}, Catalog: true,
		SubtitleLabel: "Оригінал", SubtitleHint: "назва мовою оригіналу"},
	{Kind: "anime", Label: "аніме", Nav: "Аніме", Unit: "episode", Steps: []int{1, 2},
		Prefixes: []string{"а", "a"}, Catalog: true,
		SubtitleLabel: "Оригінал", SubtitleHint: "назва мовою оригіналу"},
	{Kind: "movie", Label: "фільм", Nav: "Фільми", Unit: "minute", Steps: []int{1, 5, 10},
		Prefixes: []string{"ф", "f", "m"}, Catalog: true,
		SubtitleLabel: "Оригінал", SubtitleHint: "назва мовою оригіналу"},
	{Kind: "book", Label: "книга", Nav: "Книги", Unit: "page", Steps: []int{1, 5, 10, 25},
		Prefixes: []string{"к", "b"}, Manual: true, Catalog: true,
		SubtitleLabel: "Автор", SubtitleHint: "можна пропустити"},
	{Kind: "manga", Label: "манга", Nav: "Манґа", Unit: "volume", Steps: []int{1, 5, 10},
		Prefixes: []string{"м", "манга", "manga"}, Manual: true, Catalog: true,
		SubtitleLabel: "Автор", SubtitleHint: "мангака"},
	{Kind: "game", Label: "гра", Nav: "Ігри", Unit: "hour", Steps: []int{1, 2},
		Prefixes: []string{"і", "и", "g"}, Manual: true, Catalog: true,
		SubtitleLabel: "Платформа", SubtitleHint: "PC, PS5, Switch…"},
	// No catalog anywhere lists courses, so a "not found" would be a lie.
	{Kind: "course", Label: "курс", Nav: "Курси", Unit: "lesson", Steps: []int{1, 2},
		Prefixes: []string{"н", "c"}, Manual: true,
		SubtitleLabel: "Платформа", SubtitleHint: "Coursera, YouTube, курси в компанії…"},
}

var (
	kindByName   = map[string]kindDef{}
	kindLabels   = map[string]string{}
	unitForKind  = map[string]string{}
	manualKinds  = map[string]bool{}
	catalogKinds = map[string]bool{}
	kindPrefixes = map[string]string{}
)

func init() {
	for _, k := range kinds {
		kindByName[k.Kind] = k
		kindLabels[k.Kind] = k.Label
		unitForKind[k.Kind] = k.Unit
		manualKinds[k.Kind] = k.Manual
		catalogKinds[k.Kind] = k.Catalog
		for _, p := range k.Prefixes {
			kindPrefixes[p] = k.Kind
		}
	}
}

// kindOf tolerates an unknown kind: an old row with a retired kind still has
// to render, just without a name.
func kindOf(kind string) kindDef { return kindByName[kind] }

// unitsSwitchLabel is the depth switch in settings: «серії», «сторінки».
func unitsSwitchLabel(k kindDef) string { return domain.Units[k.Unit].Few }

func subtitleField(kind string) (label, placeholder string) {
	k, ok := kindByName[kind]
	if !ok {
		return "Оригінал", "назва мовою оригіналу"
	}
	return k.SubtitleLabel, k.SubtitleHint
}
