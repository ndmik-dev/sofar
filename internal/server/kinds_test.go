package server

import (
	"os"
	"strings"
	"testing"

	"github.com/ndmik-dev/sofar/internal/domain"
)

// Every kind needs a colour token in both themes and a unit the domain can
// name; a kind added in one place and not the others is the bug this guards.
func TestKindsAreCompleteEverywhere(t *testing.T) {
	css, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range kinds {
		token := "--k-" + k.Kind + ":"
		if n := strings.Count(string(css), token); n < 3 {
			t.Errorf("%s: %s defined %d times in app.css, want light + two dark blocks", k.Kind, token, n)
		}
		if _, ok := domain.Units[k.Unit]; !ok {
			t.Errorf("%s: unit %q is not in domain.Units", k.Kind, k.Unit)
		}
		if len(k.Steps) == 0 || len(k.Prefixes) == 0 || k.Nav == "" || k.Label == "" {
			t.Errorf("%s: incomplete definition %+v", k.Kind, k)
		}
	}
	if len(kindPrefixes) < len(kinds) {
		t.Errorf("prefixes collide: %d prefixes for %d kinds", len(kindPrefixes), len(kinds))
	}
}
