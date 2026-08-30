package server

import "testing"

func TestNormalizeLink(t *testing.T) {
	cases := []struct {
		in        string
		wantURL   string
		wantLabel string
		wantOK    bool
	}{
		{"https://www.netflix.com/title/70136120", "https://www.netflix.com/title/70136120", "Netflix", true},
		{"netflix.com/browse", "https://netflix.com/browse", "Netflix", true},
		{"  https://yakaboo.ua/ua/book.html  ", "https://yakaboo.ua/ua/book.html", "Yakaboo", true},
		{"https://example.org/x", "https://example.org/x", "example.org", true},
		{"https://www.example.org/x", "https://www.example.org/x", "example.org", true},
		// A link is put in an href and opened; only the two web schemes qualify.
		{"javascript:alert(1)", "", "", false},
		{"data:text/html,<script>", "", "", false},
		{"ftp://files.example.org", "", "", false},
		{"", "", "", false},
		{"   ", "", "", false},
	}
	for _, c := range cases {
		url, label, ok := normalizeLink(c.in)
		if ok != c.wantOK {
			t.Errorf("normalizeLink(%q) ok = %v, want %v", c.in, ok, c.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if url != c.wantURL || label != c.wantLabel {
			t.Errorf("normalizeLink(%q) = %q/%q, want %q/%q", c.in, url, label, c.wantURL, c.wantLabel)
		}
	}
}
